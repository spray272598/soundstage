package messaging

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/interaction/infrastructure/asynqworker"
	"github.com/spray272598/soundstage/internal/interaction/task"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
)

// AsynqTaskEnqueuer schedules background work through asynq. The synchronous
// hot path only enqueues; the actual persistence and settlement happen in the
// worker, so a slow MySQL never blocks a WebSocket pump.
type AsynqTaskEnqueuer struct {
	client         *asynq.Client
	batchPersister *asynqworker.BatchPersister // optional batch persister for high throughput
}

// NewAsynqTaskEnqueuer creates a new AsynqTaskEnqueuer.
func NewAsynqTaskEnqueuer(client *asynq.Client) *AsynqTaskEnqueuer {
	return &AsynqTaskEnqueuer{client: client}
}

// WithBatchPersister enables batch enqueueing for high throughput.
func (e *AsynqTaskEnqueuer) WithBatchPersister(bp *asynqworker.BatchPersister) *AsynqTaskEnqueuer {
	e.batchPersister = bp
	return e
}

// EnqueuePersistDanmaku schedules async persistence of a danmaku.
func (e *AsynqTaskEnqueuer) EnqueuePersistDanmaku(ctx context.Context, d *domain.Danmaku) error {
	if e.batchPersister != nil {
		e.batchPersister.AddDanmaku(d)
		return nil
	}
	p := task.PersistDanmakuPayload{
		ID:        d.ID,
		RoomID:    d.RoomID,
		UserID:    d.UserID,
		Content:   d.Content,
		Status:    string(d.Status),
		CreatedAt: d.CreatedAt,
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	metrics.AsynqTaskEnqueuedTotal.WithLabelValues(task.TypePersistDanmaku).Inc()
	_, err = e.client.Enqueue(asynq.NewTask(task.TypePersistDanmaku, payload))
	return err
}

// EnqueueSettleGift schedules async settlement of a gift order.
func (e *AsynqTaskEnqueuer) EnqueueSettleGift(ctx context.Context, orderID string) error {
	if e.batchPersister != nil {
		e.batchPersister.AddGift(orderID)
		return nil
	}
	p := task.SettleGiftPayload{OrderID: orderID}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	metrics.AsynqTaskEnqueuedTotal.WithLabelValues(task.TypeSettleGift).Inc()
	_, err = e.client.Enqueue(asynq.NewTask(task.TypeSettleGift, payload))
	return err
}

// Compile-time check.
var _ domain.TaskEnqueuer = (*AsynqTaskEnqueuer)(nil)
