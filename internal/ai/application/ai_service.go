// Package application wires the AI domain ports into concrete use cases:
// LLM-based danmaku moderation (replacing the keyword moderator), SSE smart
// reply via the ReAct agent, contextual auto-reply, and RAG knowledge ingest.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	interactiondomain "github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// Service is the AI room moderator. It implements interaction.Moderator (so the
// interaction context can swap it in without changes) and additionally exposes
// conversational (SSE) and RAG ingest capabilities.
type Service struct {
	llm      aidomain.Gateway
	kb       aidomain.KnowledgeBase
	agent    aidomain.Agent
	keywords []string
	// realLLM is true when a real provider (not the offline mock) is configured.
	// When false, moderation degrades to the keyword fast-path and replies use
	// canned text, keeping the demo runnable without an API key.
	realLLM bool
}

// NewService builds the AI service.
func NewService(llm aidomain.Gateway, kb aidomain.KnowledgeBase, agent aidomain.Agent, keywords []string, realLLM bool) *Service {
	return &Service{llm: llm, kb: kb, agent: agent, keywords: keywords, realLLM: realLLM}
}

// Moderate implements interaction.Moderator. It runs a cheap keyword pre-filter
// and, when a real LLM is configured, a semantic audit. Messages that pass are
// allowed; anything flagged by either path is rejected (and never broadcast).
func (s *Service) Moderate(ctx context.Context, content string) (interactiondomain.ModerationDecision, error) {
	// 1) Keyword fast-path: reject obvious violations before any LLM call.
	for _, kw := range s.keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(content, kw) {
			metrics.AIModerationTotal.WithLabelValues("rejected", "keyword").Inc()
			logger.L().Debug("ai moderator keyword hit", zap.String("kw", kw))
			return interactiondomain.ModerationDecision{
				Allowed: false,
				Reason:  "contains blocked keyword: " + kw,
				Masked:  maskContent(content, kw),
			}, nil
		}
	}

	// 2) LLM semantic audit (only when a real provider is configured).
	if s.realLLM {
		decision, err := s.llmModerate(ctx, content)
		if err != nil {
			// On LLM failure, fail open to keyword result but log it; we avoid
			// silently dropping legitimate chat because of a provider outage.
			logger.L().Warn("llm moderation failed, allowing", zap.Error(err))
			metrics.AIModerationTotal.WithLabelValues("allowed", "llm_error").Inc()
			return interactiondomain.ModerationDecision{Allowed: true, Masked: content}, nil
		}
		if !decision.Allowed {
			metrics.AIModerationTotal.WithLabelValues("rejected", "llm").Inc()
			return decision, nil
		}
		metrics.AIModerationTotal.WithLabelValues("allowed", "llm").Inc()
		return decision, nil
	}

	// 3) No real LLM: keyword pass is sufficient for the demo path.
	metrics.AIModerationTotal.WithLabelValues("allowed", "keyword").Inc()
	return interactiondomain.ModerationDecision{Allowed: true, Masked: content}, nil
}

// llmModerate asks the model to classify the message and parse strict JSON.
func (s *Service) llmModerate(ctx context.Context, content string) (interactiondomain.ModerationDecision, error) {
	system := `你是直播内容安全审核员。判断用户弹幕是否违规（涉政、色情、暴力、赌博、诈骗、辱骂、广告引流、刷屏等）。
只输出严格 JSON：{"allowed":true/false,"reason":"简短原因","category":"命中类别或 none"}。不要输出其他内容。`
	resp, err := s.llm.Generate(ctx, &aidomain.ChatRequest{
		SystemPrompt: system,
		Messages:     []aidomain.ChatMessage{{Role: "user", Content: content}},
		Temperature:  0,
	})
	if err != nil {
		return interactiondomain.ModerationDecision{}, err
	}
	return parseModerationJSON(resp.Content, content)
}

var moderationJSONRe = regexp.MustCompile(`\{[^{}]*"allowed"[^{}]*\}`)

func parseModerationJSON(raw, content string) (interactiondomain.ModerationDecision, error) {
	m := moderationJSONRe.FindString(raw)
	if m == "" {
		// Fallback: try the whole string.
		m = strings.TrimSpace(raw)
	}
	var out struct {
		Allowed  bool   `json:"allowed"`
		Reason   string `json:"reason"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(m), &out); err != nil {
		return interactiondomain.ModerationDecision{}, fmt.Errorf("parse moderation json: %w", err)
	}
	reason := out.Reason
	if reason == "" {
		reason = "blocked by AI audit: " + out.Category
	}
	return interactiondomain.ModerationDecision{
		Allowed: out.Allowed,
		Reason:  reason,
		Masked:  content,
	}, nil
}

// Chat runs the ReAct agent for a conversational turn with the room moderator
// and streams events (text deltas, tool calls, results, done).
func (s *Service) Chat(ctx context.Context, roomID, userID, message string, onEvent func(aidomain.AgentEvent)) (string, error) {
	return s.agent.Run(ctx, roomID, userID, message, onEvent)
}

// AutoReply produces a short, contextual reply to a danmaku (used for AI smart
// replies). It grounds the reply in retrieved room rules when available.
func (s *Service) AutoReply(ctx context.Context, roomID, userID, message string) (string, error) {
	if !s.realLLM {
		return "收到～我是房管，有问题随时找我哦！", nil
	}

	rules := ""
	if s.kb != nil {
		chunks, err := s.kb.Query(ctx, message, 2)
		if err == nil && len(chunks) > 0 {
			var b strings.Builder
			for _, c := range chunks {
				b.WriteString("- ")
				b.WriteString(c.Title)
				b.WriteString("：")
				b.WriteString(c.Text)
				b.WriteString("\n")
			}
			rules = b.String()
		}
	}

	system := "你是音频直播间房管。请用一句亲切的中文回复这条观众弹幕，可结合房间规则，不超过 40 字，不要使用表情符号代码。"
	user := fmt.Sprintf("观众 %s 说：%s\n相关规则：\n%s", userID, message, rules)
	resp, err := s.llm.Generate(ctx, &aidomain.ChatRequest{
		SystemPrompt: system,
		Messages:     []aidomain.ChatMessage{{Role: "user", Content: user}},
		Temperature:  0.6,
		MaxTokens:    120,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// IngestKnowledge adds a document to the RAG knowledge base.
func (s *Service) IngestKnowledge(ctx context.Context, title, text string) error {
	if s.kb == nil {
		return fmt.Errorf("knowledge base not configured")
	}
	return s.kb.Index(ctx, id.New(), title, text)
}

// Masked returns the content with the matched keyword replaced by asterisks.
func maskContent(content, kw string) string {
	return strings.ReplaceAll(content, kw, strings.Repeat("*", len([]rune(kw))))
}

// Compile-time check: Service satisfies the interaction moderator port and the
// internal AI service surface.
var _ interactiondomain.Moderator = (*Service)(nil)
