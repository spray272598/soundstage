package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// Dependencies are the external capabilities the built-in tools need. They are
// supplied as ports (defined in the ai domain) so the tools never import the
// room/miclink/interaction contexts directly.
type Dependencies struct {
	Status    aidomain.RoomStatusProvider
	Leader    aidomain.LeaderboardProvider
	Muted     aidomain.RoomModerator
	Broadcast aidomain.Broadcaster
	KB        aidomain.KnowledgeBase
}

// RegisterBuiltinTools wires the room-moderator tool set into the registry.
func RegisterBuiltinTools(reg aidomain.Registry, deps Dependencies) {
	reg.Register(&roomStatusTool{deps: deps})
	reg.Register(&leaderboardTool{deps: deps})
	reg.Register(&roomRulesTool{deps: deps})
	reg.Register(&muteTool{deps: deps})
	reg.Register(&announceTool{deps: deps})
}

// --- get_room_status ---

type roomStatusTool struct{ deps Dependencies }

func (t *roomStatusTool) Name() string { return "get_room_status" }
func (t *roomStatusTool) Description() string {
	return "查询直播间当前状态：在线人数、连麦状态、PK 比分等。"
}
func (t *roomStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"room_id": map[string]any{"type": "string", "description": "直播间 ID"},
		},
		"required": []string{"room_id"},
	}
}
func (t *roomStatusTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	roomID, _ := args["room_id"].(string)
	if roomID == "" {
		return "", fmt.Errorf("room_id is required")
	}
	st, err := t.deps.Status.Status(ctx, roomID)
	if err != nil {
		return "", err
	}
	if st == nil {
		return "未找到该直播间或直播间未开播。", nil
	}
	b, _ := json.Marshal(st)
	return string(b), nil
}

// --- get_leaderboard ---

type leaderboardTool struct{ deps Dependencies }

func (t *leaderboardTool) Name() string { return "get_leaderboard" }
func (t *leaderboardTool) Description() string {
	return "查询直播间礼物榜单（按日/周/月）。"
}
func (t *leaderboardTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"room_id": map[string]any{"type": "string"},
			"period":  map[string]any{"type": "string", "enum": []string{"day", "week", "month"}, "description": "统计周期，默认 day"},
			"n":       map[string]any{"type": "integer", "description": "返回前几名，默认 5"},
		},
		"required": []string{"room_id"},
	}
}
func (t *leaderboardTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	roomID, _ := args["room_id"].(string)
	if roomID == "" {
		return "", fmt.Errorf("room_id is required")
	}
	period, _ := args["period"].(string)
	if period == "" {
		period = "day"
	}
	n := intArg(args["n"], 5)
	entries, err := t.deps.Leader.TopGifts(ctx, roomID, period, n)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "当前榜单暂无数据。", nil
	}
	b, _ := json.Marshal(entries)
	return string(b), nil
}

// --- get_room_rules (RAG) ---

type roomRulesTool struct{ deps Dependencies }

func (t *roomRulesTool) Name() string { return "get_room_rules" }
func (t *roomRulesTool) Description() string {
	return "从知识库检索直播间规则 / 常见问题解答，用于回答合规相关问题。"
}
func (t *roomRulesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "要查询的合规问题或规则关键词"},
		},
		"required": []string{"query"},
	}
}
func (t *roomRulesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" || t.deps.KB == nil {
		return "未提供查询或无知识库可用。", nil
	}
	chunks, err := t.deps.KB.Query(ctx, query, 3)
	if err != nil {
		return "", err
	}
	if len(chunks) == 0 {
		return "知识库中未找到相关内容。", nil
	}
	out := ""
	for i, c := range chunks {
		out += fmt.Sprintf("[%d] %s\n%s\n\n", i+1, c.Title, c.Text)
	}
	return out, nil
}

// --- mute_user ---

type muteTool struct{ deps Dependencies }

func (t *muteTool) Name() string { return "mute_user" }
func (t *muteTool) Description() string {
	return "对直播间内某观众禁言（房管处罚动作）。"
}
func (t *muteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"room_id":          map[string]any{"type": "string"},
			"user_id":          map[string]any{"type": "string", "description": "被禁言观众 ID"},
			"duration_seconds": map[string]any{"type": "integer", "description": "禁言时长（秒），默认 300"},
		},
		"required": []string{"room_id", "user_id"},
	}
}
func (t *muteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	roomID, _ := args["room_id"].(string)
	userID, _ := args["user_id"].(string)
	if roomID == "" || userID == "" {
		return "", fmt.Errorf("room_id and user_id are required")
	}
	dur := time.Duration(intArg(args["duration_seconds"], 300)) * time.Second
	if err := t.deps.Muted.Mute(ctx, roomID, userID, dur); err != nil {
		return "", err
	}
	return fmt.Sprintf("已对观众 %s 禁言 %d 秒。", userID, int(dur.Seconds())), nil
}

// --- send_announcement ---

type announceTool struct{ deps Dependencies }

func (t *announceTool) Name() string { return "send_announcement" }
func (t *announceTool) Description() string {
	return "向直播间全体观众发布一条房管公告。"
}
func (t *announceTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"room_id": map[string]any{"type": "string"},
			"text":    map[string]any{"type": "string", "description": "公告内容"},
		},
		"required": []string{"room_id", "text"},
	}
}
func (t *announceTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	roomID, _ := args["room_id"].(string)
	text, _ := args["text"].(string)
	if roomID == "" || text == "" {
		return "", fmt.Errorf("room_id and text are required")
	}
	payload, _ := json.Marshal(map[string]any{"text": text, "at": time.Now().UTC()})
	if err := t.deps.Broadcast.Broadcast(ctx, roomID, "ai_announcement", payload); err != nil {
		return "", err
	}
	return "已向直播间发布房管公告。", nil
}

// intArg coerces JSON numbers / strings into an int with a default.
func intArg(v any, def int) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return def
	}
}
