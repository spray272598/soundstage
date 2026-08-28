package asynqworker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/miclink/task"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// handlePKSettle finalizes an ongoing PK battle when its countdown expires.
// Guarded by the distributed lock so concurrent triggers from both rooms or a
// retry cannot settle twice.
func (d Deps) handlePKSettle(ctx context.Context, t *asynq.Task) error {
	var p task.PKSettlePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}

	pk, err := d.PKs.GetByID(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if !pk.IsOngoing() {
		// Already finished (or not started); treat as idempotent no-op.
		return nil
	}

	unlock, err := d.Locker.Lock(ctx, "pk:"+pk.ID)
	if err != nil {
		return err // lock held elsewhere; asynq retries later
	}
	defer unlock()

	// Re-read under lock to avoid a race with a manual finish.
	pk, err = d.PKs.GetByID(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if !pk.IsOngoing() {
		return nil
	}
	if err := pk.Finish(); err != nil {
		return err
	}
	if err := d.PKs.Update(ctx, pk); err != nil {
		return err
	}

	metrics.PKSessionsTotal.WithLabelValues(string(domain.PKStatusFinished)).Inc()
	d.broadcastFinish(pk)
	logger.L().Info("pk settled",
		zap.String("session", pk.ID),
		zap.String("winner", string(pk.Winner)),
		zap.Int64("score_a", pk.ScoreA),
		zap.Int64("score_b", pk.ScoreB))
	return nil
}

// handlePKCountdown warns clients that the PK deadline is near.
func (d Deps) handlePKCountdown(ctx context.Context, t *asynq.Task) error {
	var p task.PKCountdownPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	pk, err := d.PKs.GetByID(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if !pk.IsOngoing() {
		return nil
	}
	payload, err := json.Marshal(pkCountdownBroadcast{SessionID: pk.ID})
	if err != nil {
		return err
	}
	for _, room := range []string{pk.RoomAID, pk.RoomBID} {
		if berr := d.Broadcaster.Broadcast(ctx, room, "pk_countdown", payload); berr != nil {
			logger.L().Warn("broadcast pk_countdown failed", zap.Error(berr), zap.String("room", room))
		}
	}
	return nil
}

// broadcastFinish pushes the final PK state to both rooms.
func (d Deps) broadcastFinish(pk *domain.PKSession) {
	payload, err := json.Marshal(pkFinishBroadcast{
		SessionID: pk.ID,
		Status:    string(pk.Status),
		ScoreA:    pk.ScoreA,
		ScoreB:    pk.ScoreB,
		Winner:    string(pk.Winner),
		EndedAt:   time.Now(),
	})
	if err != nil {
		logger.L().Error("marshal pk_finish failed", zap.Error(err))
		return
	}
	for _, room := range []string{pk.RoomAID, pk.RoomBID} {
		if berr := d.Broadcaster.Broadcast(context.Background(), room, "pk_finish", payload); berr != nil {
			logger.L().Warn("broadcast pk_finish failed", zap.Error(berr), zap.String("room", room))
		}
	}
}

type pkCountdownBroadcast struct {
	SessionID string `json:"session_id"`
}

type pkFinishBroadcast struct {
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	ScoreA    int64     `json:"score_a"`
	ScoreB    int64     `json:"score_b"`
	Winner    string    `json:"winner"`
	EndedAt   time.Time `json:"ended_at"`
}
