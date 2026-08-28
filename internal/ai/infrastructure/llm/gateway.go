// Package llm implements the ai domain.Gateway port with an OpenAI-compatible
// HTTP client (native tool calling + SSE) and a deterministic offline mock used
// when no API key is configured.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// MaxLLMRetries is the retry budget for transient LLM errors (429 / 5xx).
const MaxLLMRetries = 5

// ErrContextOverflow means the request exceeded the model context window.
var ErrContextOverflow = errors.New("llm: context window overflow")

// Gateway is an OpenAI-compatible chat client supporting native tool calling.
type Gateway struct {
	apiKey     string
	apiBase    string
	model      string
	client     *http.Client
	maxRetries int
}

// NewGateway builds a Gateway. Empty apiBase defaults to OpenAI; empty model
// defaults to gpt-4o-mini.
func NewGateway(apiKey, apiBase, model string) *Gateway {
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	apiBase = strings.TrimRight(apiBase, "/")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Gateway{
		apiKey:     apiKey,
		apiBase:    apiBase,
		model:      model,
		client:     &http.Client{Timeout: 180 * time.Second},
		maxRetries: MaxLLMRetries,
	}
}

// NewFromConfig selects the gateway or the mock based on key + mock flag.
func NewFromConfig(apiKey, apiBase, model string, mockOnEmpty bool) aidomain.Gateway {
	if mockOnEmpty && apiKey == "" {
		return NewMock()
	}
	if apiKey == "" {
		// No key but mock disabled: still return a mock so the process can run;
		// callers should configure a key for real moderation.
		return NewMock()
	}
	return NewGateway(apiKey, apiBase, model)
}

// --- wire types ---

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
}

type wireToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireToolSpec struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatCompletion struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// --- public API ---

// Generate returns a single completion (tool calls included if any).
func (g *Gateway) Generate(ctx context.Context, req *aidomain.ChatRequest) (*aidomain.ChatResponse, error) {
	return g.doWithRetry(ctx, req, false, nil)
}

// GenerateStream streams text deltas and returns the full completion.
func (g *Gateway) GenerateStream(ctx context.Context, req *aidomain.ChatRequest, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	return g.doWithRetry(ctx, req, true, onDelta)
}

func (g *Gateway) doWithRetry(ctx context.Context, req *aidomain.ChatRequest, stream bool, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= g.maxRetries; attempt++ {
		resp, err := g.do(ctx, req, stream, onDelta)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		status := extractStatus(err)
		if status == 0 || status == 400 || status == 401 || status == 403 {
			// Non-retryable.
			if status == 400 && isContextOverflow(err.Error()) {
				return nil, ErrContextOverflow
			}
			return nil, err
		}
		// 429 / 5xx -> backoff then retry.
		backoff := retryBackoff(attempt + 1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, fmt.Errorf("llm: %d retries exhausted: %w", g.maxRetries, lastErr)
}

func (g *Gateway) do(ctx context.Context, req *aidomain.ChatRequest, stream bool, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	msgs := make([]wireMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, wireMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		wm := wireMessage{Role: m.Role, Content: m.Content, Name: m.Name, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			wm.ToolCalls = append(wm.ToolCalls, wireToolCall{
				ID:       tc.ID,
				Type:     "function",
				Function: wireFunction{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		if wm.Role == "" {
			wm.Role = "user"
		}
		msgs = append(msgs, wm)
	}

	body := map[string]any{
		"model":       g.model,
		"messages":    msgs,
		"temperature": req.Temperature,
		"stream":      stream,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		specs := make([]wireToolSpec, 0, len(req.Tools))
		for _, t := range req.Tools {
			s := wireToolSpec{Type: "function"}
			s.Function.Name = t.Name
			s.Function.Description = t.Description
			s.Function.Parameters = t.Parameters
			specs = append(specs, s)
		}
		body["tools"] = specs
		if req.ToolChoice != "" {
			body["tool_choice"] = req.ToolChoice
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiBase+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, string(b))
	}
	if stream {
		return g.readStream(resp.Body, onDelta)
	}
	return g.readCompletion(resp.Body)
}

func (g *Gateway) readCompletion(r io.Reader) (*aidomain.ChatResponse, error) {
	var out chatCompletion
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, err
	}
	out2 := &aidomain.ChatResponse{
		PromptTokens: out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
		TotalTokens:  out.Usage.TotalTokens,
	}
	if len(out.Choices) > 0 {
		c := out.Choices[0]
		out2.Content = c.Message.Content
		for _, tc := range c.Message.ToolCalls {
			out2.ToolCalls = append(out2.ToolCalls, aidomain.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}
	return out2, nil
}

func (g *Gateway) readStream(r io.Reader, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var b strings.Builder
	// toolCalls accumulated per streaming index.
	toolCalls := map[int]*wireToolCall{}
	var toolOrder []int
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			b.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(aidomain.StreamDelta{Type: "text", Text: delta.Content})
			}
		}
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			cur, ok := toolCalls[idx]
			if !ok {
				cur = &wireToolCall{ID: tc.ID, Type: tc.Type}
				if cur.Type == "" {
					cur.Type = "function"
				}
				toolCalls[idx] = cur
				toolOrder = append(toolOrder, idx)
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Function.Name != "" {
				cur.Function.Name = tc.Function.Name
			}
			cur.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := &aidomain.ChatResponse{Content: b.String(), TotalTokens: estimateTokens(b.String())}
	for _, idx := range toolOrder {
		tc := toolCalls[idx]
		out.ToolCalls = append(out.ToolCalls, aidomain.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return out, nil
}

func estimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	ascii, cjk := 0, 0
	for _, r := range s {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF, r >= 0x3400 && r <= 0x4DBF:
			cjk++
		case r < 128:
			ascii++
		default:
			ascii += 2
		}
	}
	return ascii/4 + int(float64(cjk)*1.5)
}

func retryBackoff(retry int) time.Duration {
	const base = 2 * time.Second
	const maxBackoff = 30 * time.Second
	d := base
	for i := 1; i < retry; i++ {
		d *= 2
		if d >= maxBackoff {
			d = maxBackoff
			break
		}
	}
	return d
}

func extractStatus(err error) int {
	msg := err.Error()
	if !strings.HasPrefix(msg, "llm http ") {
		return 0
	}
	rest := strings.TrimPrefix(msg, "llm http ")
	code := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			code = code*10 + int(c-'0')
		} else {
			break
		}
	}
	return code
}

func isContextOverflow(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(l, "context_length_exceeded") ||
		strings.Contains(l, "maximum context length") ||
		strings.Contains(l, "max_tokens_plus_max_prompt_tokens")
}

// Compile-time check.
var _ aidomain.Gateway = (*Gateway)(nil)
