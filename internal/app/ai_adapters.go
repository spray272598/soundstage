package app

import (
	"context"
	"encoding/json"
	"time"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	connDomain "github.com/spray272598/soundstage/internal/connection/domain"
	interactiondomain "github.com/spray272598/soundstage/internal/interaction/domain"
	miclinkapp "github.com/spray272598/soundstage/internal/miclink/application"
	miclinkdomain "github.com/spray272598/soundstage/internal/miclink/domain"
	roomapp "github.com/spray272598/soundstage/internal/room/application"
)

// roomStatusAdapter exposes room + miclink/PK state to the AI agent without the
// ai context importing those contexts. It implements aidomain.RoomStatusProvider.
type roomStatusAdapter struct {
	rooms  *roomapp.RoomService
	hub    connDomain.Hub
	micSvc *miclinkapp.MicLinkService
	pkSvc  *miclinkapp.PKService
}

func (a *roomStatusAdapter) Status(ctx context.Context, roomID string) (*aidomain.RoomStatus, error) {
	room, err := a.rooms.Get(ctx, roomID)
	if err != nil {
		return nil, err
	}
	st := &aidomain.RoomStatus{
		RoomID:      room.ID,
		Title:       room.Title,
		AnchorID:    room.AnchorID,
		Status:      string(room.Status),
		OnlineCount: a.hub.RoomUserCount(roomID),
	}
	if link, err := a.micSvc.GetState(ctx, roomID); err == nil && link != nil {
		active := link.Status == miclinkdomain.MicLinkStatusConnected ||
			link.Status == miclinkdomain.MicLinkStatusRequesting
		st.MicLink = &aidomain.MicLinkState{
			Active:  active,
			HostID:  link.HostID,
			GuestID: link.GuestID,
		}
	}
	if pk, err := a.pkSvc.GetStateByRoom(ctx, roomID); err == nil && pk != nil {
		st.PK = &aidomain.PKState{
			SessionID: pk.ID,
			Status:    string(pk.Status),
			RoomA:     pk.RoomAID,
			RoomB:     pk.RoomBID,
			ScoreA:    pk.ScoreA,
			ScoreB:    pk.ScoreB,
		}
	}
	return st, nil
}

// leaderboardAdapter maps the gift leaderboard to the AI agent's port.
type leaderboardAdapter struct {
	rank interactiondomain.RankStore
}

func (a *leaderboardAdapter) TopGifts(ctx context.Context, roomID string, period string, n int) ([]aidomain.LeaderboardEntry, error) {
	p := interactiondomain.Period(period)
	switch p {
	case interactiondomain.PeriodDay, interactiondomain.PeriodWeek, interactiondomain.PeriodMonth:
	default:
		p = interactiondomain.PeriodDay
	}
	entries, err := a.rank.TopN(ctx, roomID, p, n)
	if err != nil {
		return nil, err
	}
	out := make([]aidomain.LeaderboardEntry, 0, len(entries))
	for i, e := range entries {
		out = append(out, aidomain.LeaderboardEntry{
			UserID: e.UserID,
			Amount: e.Amount,
			Rank:   i + 1,
		})
	}
	return out, nil
}

// roomModeratorAdapter drives interaction's Muter on behalf of the agent.
type roomModeratorAdapter struct {
	muter interactiondomain.Muter
}

func (a *roomModeratorAdapter) Mute(ctx context.Context, roomID, userID string, duration time.Duration) error {
	return a.muter.Mute(ctx, roomID, userID, duration)
}

func (a *roomModeratorAdapter) Unmute(ctx context.Context, roomID, userID string) error {
	return a.muter.Unmute(ctx, roomID, userID)
}

// aiBroadcasterAdapter adapts the interaction Broadcaster (json.RawMessage
// payload) to the ai domain Broadcaster ([]byte payload), decoupling the ai
// context from the interaction wire format.
type aiBroadcasterAdapter struct {
	inner interactiondomain.Broadcaster
}

func (a aiBroadcasterAdapter) Broadcast(ctx context.Context, roomID, msgType string, payload []byte) error {
	return a.inner.Broadcast(ctx, roomID, msgType, json.RawMessage(payload))
}

// Compile-time checks.
var (
	_ aidomain.RoomStatusProvider  = (*roomStatusAdapter)(nil)
	_ aidomain.LeaderboardProvider = (*leaderboardAdapter)(nil)
	_ aidomain.RoomModerator       = (*roomModeratorAdapter)(nil)
	_ aidomain.Broadcaster         = aiBroadcasterAdapter{}
)
