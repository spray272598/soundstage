package application

import (
	"context"
	"strings"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// scriptedGateway returns queued responses in order.
type scriptedGateway struct {
	calls     int
	responses []aidomain.ChatResponse
}

func (g *scriptedGateway) Generate(_ context.Context, _ *aidomain.ChatRequest) (*aidomain.ChatResponse, error) {
	resp := g.responses[g.calls]
	g.calls++
	return &resp, nil
}
func (g *scriptedGateway) GenerateStream(_ context.Context, req *aidomain.ChatRequest, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	return g.Generate(context.Background(), req)
}

func TestModerateKeywordFastPath(t *testing.T) {
	svc := NewService(&scriptedGateway{}, nil, nil, []string{"广告", "赌博"}, false)
	dec, err := svc.Moderate(context.Background(), "点击链接买广告")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected keyword-blocked message to be rejected")
	}
	dec, err = svc.Moderate(context.Background(), "主播唱得真好听")
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatal("expected clean message to be allowed")
	}
}

func TestModerateLLMAudit(t *testing.T) {
	// Real LLM path: the gateway returns a strict moderation JSON.
	gw := &scriptedGateway{responses: []aidomain.ChatResponse{
		{Content: `{"allowed":false,"reason":"辱骂","category":"abuse"}`},
	}}
	svc := NewService(gw, nil, nil, []string{"广告"}, true)
	dec, err := svc.Moderate(context.Background(), "你这人真垃圾")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected LLM-audited abuse to be rejected")
	}
	if !strings.Contains(dec.Reason, "辱骂") {
		t.Fatalf("unexpected reason: %q", dec.Reason)
	}

	// Allowed case.
	gw2 := &scriptedGateway{responses: []aidomain.ChatResponse{
		{Content: `{"allowed":true,"reason":"","category":"none"}`},
	}}
	svc2 := NewService(gw2, nil, nil, nil, true)
	dec2, err := svc2.Moderate(context.Background(), "今晚有什么安排")
	if err != nil {
		t.Fatal(err)
	}
	if !dec2.Allowed {
		t.Fatal("expected safe message to be allowed")
	}
}

func TestAutoReplyMockMode(t *testing.T) {
	svc := NewService(&scriptedGateway{}, nil, nil, nil, false)
	reply, err := svc.AutoReply(context.Background(), "r1", "u1", "主播好厉害")
	if err != nil {
		t.Fatal(err)
	}
	if reply == "" {
		t.Fatal("expected a non-empty canned reply in mock mode")
	}
}
