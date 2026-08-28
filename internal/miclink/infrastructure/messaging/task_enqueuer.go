package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/miclink/task"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
)

// AsynqTaskEnqueuer schedules PK background work through asynq with a delay.
// The settle task fires when the PK countdown expires; the countdown notice
// fires shortly before to warn clients.
type AsynqTaskEnqueuer struct {
	client *asynq.Client
}

// NewAsynqTaskEnqueuer creates a new AsynqTaskEnqueuer.
func NewAsynqTaskEnqueuer(client *asynq.Client) *AsynqTaskEnqueuer {
	return &AsynqTaskEnqueuer{client: client}
}

// EnqueuePKSettle schedules settlement of a PK session at runAt in the future.
func (e *AsynqTaskEnqueuer) EnqueuePKSettle(ctx context.Context, sessionID string, runAt time.Duration) error {
	p := task.PKSettlePayload{SessionID: sessionID}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	metrics.AsynqTaskEnqueuedTotal.WithLabelValues(task.TypePKSettle).Inc()
	_, err = e.client.Enqueue(asynq.NewTask(task.TypePKSettle, payload), asynq.ProcessIn(runAt))
	return err
}

// EnqueuePKCountdown schedules the last-seconds warning at runAt in the future.
func (e *AsynqTaskEnqueuer) EnqueuePKCountdown(ctx context.Context, sessionID string, runAt time.Duration) error {
	p := task.PKCountdownPayload{SessionID: sessionID}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	metrics.AsynqTaskEnqueuedTotal.WithLabelValues(task.TypePKCountdown).Inc()
	_, err = e.client.Enqueue(asynq.NewTask(task.TypePKCountdown, payload), asynq.ProcessIn(runAt))
	return err
}

// Compile-time check.
var _ domain.TaskEnqueuer = (*AsynqTaskEnqueuer)(nil)
