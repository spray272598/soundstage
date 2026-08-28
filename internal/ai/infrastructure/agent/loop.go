package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// Config tunes the ReAct loop.
type Config struct {
	// MaxRounds caps tool-calling iterations to avoid runaway runs.
	MaxRounds int
	// Timeout caps the wall-clock duration of a single agent run. When the
	// run exceeds it the context is cancelled and the run is marked canceled,
	// so a slow/hung LLM can never pin an SSE connection open forever.
	Timeout time.Duration
	// SystemPrompt overrides the default moderator persona when non-empty.
	SystemPrompt string
}

// Loop is the ReAct agent: it calls the model, executes any requested tools,
// feeds results back, and repeats until the model answers without tools.
type Loop struct {
	llm aidomain.Gateway
	reg aidomain.Registry
	cfg Config
}

// NewLoop builds a Loop.
func NewLoop(llm aidomain.Gateway, reg aidomain.Registry, cfg Config) *Loop {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 8
	}
	return &Loop{llm: llm, reg: reg, cfg: cfg}
}

// Run executes one user turn and returns the final answer, streaming events.
func (l *Loop) Run(ctx context.Context, roomID, userID, message string, onEvent func(aidomain.AgentEvent)) (string, error) {
	start := time.Now()
	// Apply the run-level timeout (config.ai.agent_timeout) so a slow or hung
	// LLM cannot keep an SSE connection open indefinitely. The client's own
	// context cancellation still takes precedence via runCtx.
	runCtx := ctx
	if l.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, l.cfg.Timeout)
		defer cancel()
	}
	history := []aidomain.ChatMessage{{Role: "system", Content: l.systemPrompt(roomID, userID)}}
	history = append(history, aidomain.ChatMessage{Role: "user", Content: message})

	tools := l.reg.Specs()
	final := ""

	for round := 0; round < l.cfg.MaxRounds; round++ {
		select {
		case <-runCtx.Done():
			metrics.AIAgentRunsTotal.WithLabelValues("canceled").Inc()
			return final, runCtx.Err()
		default:
		}

		resp, err := l.llm.GenerateStream(runCtx, &aidomain.ChatRequest{
			Messages:    history,
			Tools:       tools,
			Temperature: 0.2,
		}, func(d aidomain.StreamDelta) {
			if d.Type == "text" && onEvent != nil {
				onEvent(aidomain.AgentEvent{Type: aidomain.EventTextDelta, Text: d.Text})
			}
		})
		if err != nil {
			metrics.AIAgentRunsTotal.WithLabelValues("error").Inc()
			if onEvent != nil {
				onEvent(aidomain.AgentEvent{Type: aidomain.EventError, Text: err.Error()})
			}
			logger.L().Error("agent llm call failed", zap.Error(err))
			return final, err
		}

		calls := resp.ToolCalls
		if len(calls) == 0 {
			// Mock / ReAct-style: model emitted a JSON tool call in content.
			calls = parseToolCallsFromContent(resp.Content)
		}

		if len(calls) == 0 {
			// No tool requested -> this is the final answer.
			final = strings.TrimSpace(resp.Content)
			break
		}

		// Record the assistant turn (with tool calls) for multi-turn context.
		assistant := aidomain.ChatMessage{Role: "assistant", Content: resp.Content, ToolCalls: calls}
		history = append(history, assistant)

		for _, call := range calls {
			if call.ID == "" {
				call.ID = "call_" + id.New()[:8]
			}
			if onEvent != nil {
				onEvent(aidomain.AgentEvent{Type: aidomain.EventToolCall, ToolName: call.Name, ToolArgs: call.Arguments})
			}
			result := l.execute(runCtx, call)
			if onEvent != nil {
				onEvent(aidomain.AgentEvent{Type: aidomain.EventToolResult, ToolName: call.Name, ToolResult: result})
			}
			history = append(history, aidomain.ChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    result,
			})
		}
	}

	if final == "" {
		// Ran out of rounds; ask the model to summarize without tools.
		resp, err := l.llm.Generate(runCtx, &aidomain.ChatRequest{
			Messages:    append(history, aidomain.ChatMessage{Role: "user", Content: "请基于已有信息直接给出最终回复，不要再调用工具。"}),
			Temperature: 0.2,
			ToolChoice:  "none",
		})
		if err == nil {
			final = strings.TrimSpace(resp.Content)
		}
	}

	if onEvent != nil {
		onEvent(aidomain.AgentEvent{Type: aidomain.EventDone, Text: final})
	}
	metrics.AILatencySeconds.WithLabelValues("agent").Observe(time.Since(start).Seconds())
	metrics.AIAgentRunsTotal.WithLabelValues("ok").Inc()
	return final, nil
}

// execute runs a single tool and records metrics. Tool errors are returned as
// text so the model can recover rather than the loop aborting.
func (l *Loop) execute(ctx context.Context, call aidomain.ToolCall) string {
	tool, ok := l.reg.Get(call.Name)
	if !ok {
		metrics.AIToolCallsTotal.WithLabelValues(call.Name, "unknown").Inc()
		return fmt.Sprintf("未知的工具: %s", call.Name)
	}
	var args map[string]any
	if call.Arguments != "" {
		_ = json.Unmarshal([]byte(call.Arguments), &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	result, err := tool.Execute(ctx, args)
	if err != nil {
		metrics.AIToolCallsTotal.WithLabelValues(call.Name, "error").Inc()
		logger.L().Warn("agent tool error", zap.String("tool", call.Name), zap.Error(err))
		return fmt.Sprintf("工具执行失败: %v", err)
	}
	metrics.AIToolCallsTotal.WithLabelValues(call.Name, "ok").Inc()
	return result
}

func (l *Loop) systemPrompt(roomID, userID string) string {
	if l.cfg.SystemPrompt != "" {
		return l.cfg.SystemPrompt
	}
	return fmt.Sprintf(`你是音频直播间的 AI 房管（房间 %s，操作者 %s）。
你的职责：实时审核弹幕、回答观众关于房间状态与礼物榜单的问题、对违规观众执行禁言、发布房管公告。
你可以使用以下工具：get_room_status（查房间状态）、get_leaderboard（查礼物榜）、get_room_rules（查规则知识库）、mute_user（禁言）、send_announcement（发公告）。
当需要信息或需要执行动作时，先调用对应工具；拿到结果后用简洁中文回答用户。不要编造规则或数据。`, roomID, userID)
}

// parseToolCallsFromContent extracts tool calls from a model response that
// returns JSON (object or array) instead of native function calls. Used by the
// offline mock gateway and any provider without native tool support.
func parseToolCallsFromContent(content string) []aidomain.ToolCall {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	// Strip a leading/trailing markdown code fence if present.
	if i := strings.Index(content, "```"); i >= 0 {
		rest := content[i+3:]
		if strings.HasPrefix(strings.ToLower(rest), "json") {
			rest = rest[4:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			content = strings.TrimSpace(rest[:j])
		}
	}

	var single struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(content), &single); err == nil && single.Name != "" {
		rawArgs, _ := json.Marshal(single.Args)
		return []aidomain.ToolCall{{Name: single.Name, Arguments: string(rawArgs)}}
	}

	var multi []struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(content), &multi); err == nil && len(multi) > 0 && multi[0].Name != "" {
		out := make([]aidomain.ToolCall, 0, len(multi))
		for _, m := range multi {
			rawArgs, _ := json.Marshal(m.Args)
			out = append(out, aidomain.ToolCall{Name: m.Name, Arguments: string(rawArgs)})
		}
		return out
	}
	return nil
}

// Compile-time check.
var _ aidomain.Agent = (*Loop)(nil)
