package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// PKServiceConfig holds tunables for PK battles.
type PKServiceConfig struct {
	// Duration is the default length of a battle.
	Duration time.Duration
	// CountdownNotice fires this long before the deadline.
	CountdownNotice time.Duration
}

// PKService owns the cross-room PK (对战) use cases. It only manages the state
// machine and scoring; the live audio mix between the two rooms is external.
type PKService struct {
	pkRepo      domain.PKSessionRepository
	broadcaster domain.Broadcaster
	tasks       domain.TaskEnqueuer
	locker      domain.Locker
	cfg         PKServiceConfig
}

// NewPKService constructs a PKService from its ports.
func NewPKService(
	pkRepo domain.PKSessionRepository,
	broadcaster domain.Broadcaster,
	tasks domain.TaskEnqueuer,
	locker domain.Locker,
	cfg PKServiceConfig,
) *PKService {
	return &PKService{
		pkRepo:      pkRepo,
		broadcaster: broadcaster,
		tasks:       tasks,
		locker:      locker,
		cfg:         cfg,
	}
}

// Challenge opens a pending PK between two rooms. The challenging room is side
// A, the opponent is side B. Both rooms must be free of an active battle.
func (s *PKService) Challenge(ctx context.Context, roomA, anchorA, roomB, anchorB string) (*domain.PKSession, error) {
	if roomA == roomB {
		return nil, domain.ErrPKConflict
	}
	unlock, err := s.locker.Lock(ctx, "pk:challenge:"+roomA+":"+roomB)
	if err != nil {
		return nil, err
	}
	defer unlock()

	for _, room := range []string{roomA, roomB} {
		if _, gerr := s.pkRepo.GetActiveByRoom(ctx, room); gerr == nil {
			return nil, domain.ErrPKConflict
		} else if gerr != domain.ErrPKNotFound {
			return nil, gerr
		}
	}

	pk := domain.NewPKSession(id.New(), roomA, roomB, anchorA, anchorB)
	if err := s.pkRepo.Create(ctx, pk); err != nil {
		return nil, err
	}
	metrics.PKSessionsTotal.WithLabelValues(string(domain.PKStatusPending)).Inc()

	s.broadcast(ctx, pk.RoomAID, "pk_challenge", pkStateBroadcast(pk))
	s.broadcast(ctx, pk.RoomBID, "pk_invite", pkInviteBroadcast(pk))
	return pk, nil
}

// Accept moves a pending PK to matched. Either participant may accept.
func (s *PKService) Accept(ctx context.Context, sessionID, roomID string) (*domain.PKSession, error) {
	pk, err := s.loadParticipant(ctx, sessionID, roomID)
	if err != nil {
		return nil, err
	}
	unlock, err := s.locker.Lock(ctx, "pk:"+pk.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	pk, err = s.pkRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := pk.Accept(); err != nil {
		return nil, err
	}
	if err := s.pkRepo.Update(ctx, pk); err != nil {
		return nil, err
	}
	metrics.PKSessionsTotal.WithLabelValues(string(domain.PKStatusMatched)).Inc()
	s.broadcast(ctx, pk.RoomAID, "pk_matched", pkStateBroadcast(pk))
	s.broadcast(ctx, pk.RoomBID, "pk_matched", pkStateBroadcast(pk))
	return pk, nil
}

// Start begins the battle and arms the countdown. Enqueues settle (at deadline)
// and countdown notice (before deadline) as asynq delayed tasks.
func (s *PKService) Start(ctx context.Context, sessionID, roomID string) (*domain.PKSession, error) {
	pk, err := s.loadParticipant(ctx, sessionID, roomID)
	if err != nil {
		return nil, err
	}
	unlock, err := s.locker.Lock(ctx, "pk:"+pk.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	pk, err = s.pkRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := pk.Start(s.cfg.Duration); err != nil {
		return nil, err
	}
	if err := s.pkRepo.Update(ctx, pk); err != nil {
		return nil, err
	}
	metrics.PKSessionsTotal.WithLabelValues(string(domain.PKStatusOngoing)).Inc()

	if err := s.tasks.EnqueuePKSettle(ctx, pk.ID, s.cfg.Duration); err != nil {
		logger.L().Error("enqueue pk settle failed", zap.Error(err))
	}
	if s.cfg.CountdownNotice > 0 {
		noticeAt := s.cfg.Duration - s.cfg.CountdownNotice
		if noticeAt > 0 {
			if err := s.tasks.EnqueuePKCountdown(ctx, pk.ID, noticeAt); err != nil {
				logger.L().Error("enqueue pk countdown failed", zap.Error(err))
			}
		}
	}

	payload := pkStartBroadcast(pk, int64(s.cfg.Duration.Seconds()))
	s.broadcast(ctx, pk.RoomAID, "pk_start", payload)
	s.broadcast(ctx, pk.RoomBID, "pk_start", payload)
	return pk, nil
}

// Score adds points to the side that the given room belongs to. Used both by
// the explicit REST/WS entry points and by the gift-feed in the ingest path.
func (s *PKService) Score(ctx context.Context, roomID string, amount int64) (*domain.PKSession, error) {
	pk, err := s.pkRepo.GetActiveByRoom(ctx, roomID)
	if err != nil {
		// Room not in an active PK; nothing to score.
		if errors.Is(err, domain.ErrPKNotFound) {
			return nil, nil
		}
		return nil, err
	}
	side, ok := pk.SideOfRoom(roomID)
	if !ok {
		return nil, domain.ErrNotPKParticipant
	}
	unlock, err := s.locker.Lock(ctx, "pk:"+pk.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	pk, err = s.pkRepo.GetByID(ctx, pk.ID)
	if err != nil {
		return nil, err
	}
	if err := pk.AddScore(side, amount); err != nil {
		return nil, err
	}
	if err := s.pkRepo.Update(ctx, pk); err != nil {
		return nil, err
	}
	metrics.PKScoreTotal.WithLabelValues(string(side)).Add(float64(amount))

	payload := pkScoreBroadcast(pk)
	s.broadcast(ctx, pk.RoomAID, "pk_score", payload)
	s.broadcast(ctx, pk.RoomBID, "pk_score", payload)
	return pk, nil
}

// Finish ends the battle early and decides the winner.
func (s *PKService) Finish(ctx context.Context, sessionID, roomID string) (*domain.PKSession, error) {
	pk, err := s.loadParticipant(ctx, sessionID, roomID)
	if err != nil {
		return nil, err
	}
	unlock, err := s.locker.Lock(ctx, "pk:"+pk.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	pk, err = s.pkRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := pk.Finish(); err != nil {
		return nil, err
	}
	if err := s.pkRepo.Update(ctx, pk); err != nil {
		return nil, err
	}
	metrics.PKSessionsTotal.WithLabelValues(string(domain.PKStatusFinished)).Inc()
	s.broadcast(ctx, pk.RoomAID, "pk_finish", pkFinishBroadcast(pk))
	s.broadcast(ctx, pk.RoomBID, "pk_finish", pkFinishBroadcast(pk))
	return pk, nil
}

// GetState returns the current PK session.
func (s *PKService) GetState(ctx context.Context, sessionID string) (*domain.PKSession, error) {
	return s.pkRepo.GetByID(ctx, sessionID)
}

// GetStateByRoom returns the active PK battle involving roomID, or nil when the
// room is not currently battling. Used by read-only queries (e.g. the AI room
// moderator status tool).
func (s *PKService) GetStateByRoom(ctx context.Context, roomID string) (*domain.PKSession, error) {
	pk, err := s.pkRepo.GetActiveByRoom(ctx, roomID)
	if err == domain.ErrPKNotFound {
		return nil, nil
	}
	return pk, err
}

// loadParticipant loads a session and verifies the room is a participant.
func (s *PKService) loadParticipant(ctx context.Context, sessionID, roomID string) (*domain.PKSession, error) {
	pk, err := s.pkRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if _, ok := pk.SideOfRoom(roomID); !ok {
		return nil, domain.ErrNotPKParticipant
	}
	return pk, nil
}

func (s *PKService) broadcast(ctx context.Context, roomID, msgType string, payload json.RawMessage) {
	if err := s.broadcaster.Broadcast(ctx, roomID, msgType, payload); err != nil {
		logger.L().Warn("broadcast pk event failed", zap.Error(err), zap.String("room", roomID), zap.String("type", msgType))
	}
}

func pkStateBroadcast(pk *domain.PKSession) json.RawMessage {
	b, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
		RoomA     string `json:"room_a"`
		RoomB     string `json:"room_b"`
	}{SessionID: pk.ID, Status: string(pk.Status), RoomA: pk.RoomAID, RoomB: pk.RoomBID})
	return b
}

func pkInviteBroadcast(pk *domain.PKSession) json.RawMessage {
	b, _ := json.Marshal(struct {
		SessionID  string `json:"session_id"`
		FromRoom   string `json:"from_room"`
		FromAnchor string `json:"from_anchor"`
		ToRoom     string `json:"to_room"`
		ToAnchor   string `json:"to_anchor"`
	}{SessionID: pk.ID, FromRoom: pk.RoomAID, FromAnchor: pk.AnchorAID, ToRoom: pk.RoomBID, ToAnchor: pk.AnchorBID})
	return b
}

func pkStartBroadcast(pk *domain.PKSession, durationSec int64) json.RawMessage {
	b, _ := json.Marshal(struct {
		SessionID string    `json:"session_id"`
		EndsAt    time.Time `json:"ends_at"`
		Duration  int64     `json:"duration_sec"`
	}{SessionID: pk.ID, EndsAt: *pk.EndsAt, Duration: durationSec})
	return b
}

func pkScoreBroadcast(pk *domain.PKSession) json.RawMessage {
	b, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
		ScoreA    int64  `json:"score_a"`
		ScoreB    int64  `json:"score_b"`
	}{SessionID: pk.ID, ScoreA: pk.ScoreA, ScoreB: pk.ScoreB})
	return b
}

func pkFinishBroadcast(pk *domain.PKSession) json.RawMessage {
	b, _ := json.Marshal(struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
		ScoreA    int64  `json:"score_a"`
		ScoreB    int64  `json:"score_b"`
		Winner    string `json:"winner"`
	}{SessionID: pk.ID, Status: string(pk.Status), ScoreA: pk.ScoreA, ScoreB: pk.ScoreB, Winner: string(pk.Winner)})
	return b
}
