package rag

import "context"

// doc is a seed knowledge document.
type doc struct {
	ID    string
	Title string
	Text  string
}

// DefaultDocuments returns the built-in room rulebook/FAQ used to bootstrap the
// RAG knowledge base. Operators can extend it at runtime via the knowledge API.
func DefaultDocuments() []doc {
	return []doc{
		{
			ID:    "rule-overview",
			Title: "直播间基本规则",
			Text:  "本音频直播间禁止发布政治、色情、暴力、赌博、诈骗等违法违规内容。禁止刷屏、恶意引战、人身攻击与歧视性言论。首次违规将收到房管警告，多次违规将被禁言或移出直播间。",
		},
		{
			ID:    "rule-mute",
			Title: "禁言处理流程",
			Text:  "当观众存在刷屏、辱骂、广告引流等行为时，房管可对其实行禁言，时长通常为 5 到 30 分钟。禁言期间该用户无法发送弹幕，但可继续收听。重复违规可再次禁言或延长时长。",
		},
		{
			ID:    "rule-gift",
			Title: "礼物与打赏",
			Text:  "观众可以通过礼物表达对主播的支持，礼物金额会计入礼物榜。礼物榜按日、周、月以及总榜统计。打赏是观众自愿行为，平台不承诺任何回报。",
		},
		{
			ID:    "rule-pk",
			Title: "连麦与 PK 说明",
			Text:  "主播可发起跨房间 PK，两个房间的礼物总价值决定胜负。PK 期间请文明应援，避免互喷。PK 结束后由房管公布结果。",
		},
		{
			ID:    "faq-ai",
			Title: "AI 房管能做什么",
			Text:  "我是本直播间的 AI 房管，可以实时审核弹幕内容、回答关于房间状态与礼物榜单的问题、对违规观众执行禁言，并通过房间公告发布提醒。如有疑问可以直接向我提问。",
		},
	}
}

// SeedDefaultKnowledge indexes the built-in documents into the knowledge base.
func SeedDefaultKnowledge(ctx context.Context, s *Service) error {
	for _, d := range DefaultDocuments() {
		if err := s.Index(ctx, d.ID, d.Title, d.Text); err != nil {
			return err
		}
	}
	return nil
}
