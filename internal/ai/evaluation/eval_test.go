package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/agent"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/rag"
)

// TestEvalAll runs all evaluation tests (RAG, Agent, SSE).
// This is the main CI gate test - run with `go test -run TestEvalAll ./internal/ai/evaluation/`
func TestEvalAll(t *testing.T) {
	// Skip if not explicitly enabled (to avoid running in normal CI without API keys)
	if os.Getenv("EVAL_ENABLED") != "1" {
		t.Skip("EVAL_ENABLED=1 not set, skipping evaluation tests")
	}

	ctx := context.Background()

	// Build test dependencies
	// Use mock LLM for evaluation to keep it deterministic and fast
	mockLLM := llm.NewMock()
	mockEmbedder := llm.NewMockEmbedder(32)

	// Build RAG knowledge base with test documents
	kb := rag.NewService(mockEmbedder)
	setupTestKnowledgeBase(ctx, t, kb)

	// Build agent
	llmGateway := mockLLM
	agentInstance := agent.NewLoop(llmGateway, nil, agent.Config{
		MaxRounds: 8,
		Timeout:   60 * time.Second,
	})

	// Run evaluations
	t.Run("RAG_Retrieval", func(t *testing.T) {
		evalRAGRetrieval(ctx, t, kb)
	})

	t.Run("Agent_ToolCalling", func(t *testing.T) {
		evalAgentToolCalling(ctx, t, agentInstance)
	})

	t.Run("SSE_Latency", func(t *testing.T) {
		evalSSELatency(ctx, t, agentInstance)
	})
}

func setupTestKnowledgeBase(ctx context.Context, t *testing.T, kb aidomain.KnowledgeBase) {
	docs := []struct {
		id, title, text string
	}{
		{"doc-room-open", "开播流程", "主播点击开始直播按钮即可开播，房间状态变为直播中"},
		{"doc-room-close", "关播流程", "主播点击结束直播按钮即可关播，房间状态变为已结束"},
		{"doc-room-create", "创建房间", "在后台创建房间，填写标题和封面"},
		{"doc-gift-catalog", "礼物目录", "小心心1币，棒棒糖10币，火箭500币，游艇10000币"},
		{"doc-gift-price", "礼物价格", "所有礼物价格见礼物目录，充值后可购买"},
		{"doc-miclink-invite", "发起连麦", "主播点击邀请连麦，观众接受后建立连接"},
		{"doc-miclink-accept", "接受连麦", "观众收到邀请后点击接受即可连麦"},
		{"doc-pk-create", "发起PK", "主播点击发起PK，选择对战房间"},
		{"doc-pk-rules", "PK规则", "PK时长5分钟，倒计时最后10秒提醒，礼物总额高者胜"},
		{"doc-moderation-rules", "违规规则", "涉政、色情、暴力、赌博、诈骗、辱骂、广告引流、刷屏均为违规"},
		{"doc-appeal-process", "申诉流程", "被误封可在24小时内提交申诉，人工复核"},
		{"doc-moderator-permissions", "房管权限", "房管可禁言、解禁、踢人、管理弹幕"},
		{"doc-mute-unmute", "禁言解禁", "房管可对用户设置禁言时长，或解除禁言"},
		{"doc-room-stats", "房间数据", "在线人数、点赞数、礼物榜实时更新"},
		{"doc-online-count", "在线人数", "右上角显示当前在线观众数"},
		{"doc-leaderboard-rules", "排行榜规则", "按礼物总金额排名，日/周/月三个周期"},
		{"doc-gift-settlement", "礼物结算", "礼物发送后异步结算，更新排行榜"},
		{"doc-blocked-keywords", "违规词库", "包含敏感词、广告词、辱骂词等"},
		{"doc-moderator-application", "房管申请", "主播可在后台设置房管，或观众申请"},
		{"doc-miclink-reject", "连麦拒绝", "观众拒绝连麦邀请后，主播可重新邀请"},
		{"doc-miclink-rules", "连麦规则", "连麦需主播邀请，观众接受，单次最长30分钟"},
		{"doc-pk-countdown", "PK倒计时", "PK最后10秒全员倒计时提醒"},
		{"doc-pk-settings", "PK设置", "可设置PK时长、倒计时提醒时间"},
		{"doc-danmaku-ratelimit", "弹幕限流", "每用户每秒最多1条弹幕，超限自动拦截"},
		{"doc-interaction-rules", "互动规则", "点赞无限制，礼物需余额，弹幕有限流"},
		{"doc-gift-send", "送礼流程", "点击礼物面板选择礼物和数量，确认发送"},
		{"doc-room-title", "修改标题", "主播可在直播中修改房间标题"},
		{"doc-room-settings", "房间设置", "封面、标题、标签、公告均可设置"},
		{"doc-unmute-process", "解禁流程", "房管点击用户头像选择解除禁言"},
		{"doc-like-send", "点赞操作", "点击右下角爱心即可点赞"},
		{"doc-analytics", "数据看板", "后台提供实时数据和历史报表"},
		{"doc-report-process", "举报流程", "长按弹幕选择举报，填写原因提交"},
	}

	for _, doc := range docs {
		if err := kb.Index(ctx, doc.id, doc.title, doc.text); err != nil {
			t.Fatalf("index doc %s: %v", doc.id, err)
		}
	}
}

func evalRAGRetrieval(ctx context.Context, t *testing.T, kb aidomain.KnowledgeBase) {
	suitePath := filepath.Join("testdata", "rag_eval_cases.json")
	suite, err := LoadRAGTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load rag test suite: %v", err)
	}

	ks := []int{1, 3, 5, 10}
	metrics := EvaluateRAG(ctx, kb, suite, ks)

	// Print metrics
	t.Log(PrintRAGMetrics(metrics))

	// Save metrics for CI artifact
	saveMetrics(t, "rag_metrics.json", metrics)

	// Assert minimum thresholds (adjust based on your requirements)
	if metrics.MRR < 0.5 {
		t.Errorf("MRR too low: %.4f (expected >= 0.5)", metrics.MRR)
	}
	if metrics.RecallAtK[3] < 0.4 {
		t.Errorf("Recall@3 too low: %.4f (expected >= 0.4)", metrics.RecallAtK[3])
	}
}

func evalAgentToolCalling(ctx context.Context, t *testing.T, ag aidomain.Agent) {
	suitePath := filepath.Join("testdata", "agent_tool_eval_cases.json")
	suite, err := LoadAgentToolCallTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load agent test suite: %v", err)
	}

	metrics := EvaluateAgentToolCalls(ctx, ag, suite)

	t.Log(PrintAgentToolCallMetrics(metrics))
	saveMetrics(t, "agent_tool_metrics.json", metrics)

	if metrics.ToolNameAccuracy < 0.75 {
		t.Errorf("Tool name accuracy too low: %.2f%% (expected >= 75%%)", metrics.ToolNameAccuracy*100)
	}
	if metrics.ArgsParseSuccess < 0.75 {
		t.Errorf("Args parse success too low: %.2f%% (expected >= 75%%)", metrics.ArgsParseSuccess*100)
	}
}

func evalSSELatency(ctx context.Context, t *testing.T, ag aidomain.Agent) {
	suitePath := filepath.Join("testdata", "sse_latency_eval_cases.json")
	suite, err := LoadSSELatencyTestSuite(suitePath)
	if err != nil {
		t.Fatalf("load sse test suite: %v", err)
	}

	metrics := EvaluateSSELatency(ctx, ag, suite, 3) // 3 runs per case

	t.Log(PrintSSELatencyMetrics(metrics))
	saveMetrics(t, "sse_latency_metrics.json", metrics)

	// Assert latency thresholds (adjust based on your SLO)
	if metrics.FirstTokenLatency.P95 > 5*time.Second {
		t.Errorf("First token P95 too high: %v (expected <= 5s)", metrics.FirstTokenLatency.P95)
	}
	if metrics.FullResponseLatency.P95 > 30*time.Second {
		t.Errorf("Full response P95 too high: %v (expected <= 30s)", metrics.FullResponseLatency.P95)
	}
}

func saveMetrics(t *testing.T, filename string, metrics any) {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		t.Logf("failed to marshal metrics: %v", err)
		return
	}
	// Write to test output directory (GitHub Actions can pick up as artifact)
	outDir := os.Getenv("TEST_OUTPUT_DIR")
	if outDir == "" {
		outDir = "test_output"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Logf("failed to create output dir: %v", err)
		return
	}
	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Logf("failed to write metrics: %v", err)
	}
}