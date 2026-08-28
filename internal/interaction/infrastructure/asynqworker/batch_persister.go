package asynqworker

import (
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/interaction/task"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// BatchPersister collects danmaku/gifts and flushes them in batches.
type BatchPersister struct {
	dmBuffer    []*domain.Danmaku
	giftBuffer  []string // order IDs
	mu          sync.Mutex
	client      *asynq.Client
	flushInterval time.Duration
	maxBatchSize  int
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// BatchConfig holds batch persister configuration.
type BatchConfig struct {
	FlushInterval time.Duration
	MaxBatchSize  int
}

// DefaultBatchConfig returns sensible defaults.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		FlushInterval: 100 * time.Millisecond,
		MaxBatchSize:  100,
	}
}

// NewBatchPersister creates a new batch persister.
func NewBatchPersister(client *asynq.Client, cfg BatchConfig) *BatchPersister {
	bp := &BatchPersister{
		client:        client,
		flushInterval: cfg.FlushInterval,
		maxBatchSize:  cfg.MaxBatchSize,
		stopCh:        make(chan struct{}),
	}
	bp.wg.Add(1)
	go bp.flushLoop()
	return bp
}

// AddDanmaku adds a danmaku to the batch buffer.
func (bp *BatchPersister) AddDanmaku(d *domain.Danmaku) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.dmBuffer = append(bp.dmBuffer, d)
	if len(bp.dmBuffer) >= bp.maxBatchSize {
		bp.flushLocked()
	}
}

// AddGift adds a gift order ID to the batch buffer.
func (bp *BatchPersister) AddGift(orderID string) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.giftBuffer = append(bp.giftBuffer, orderID)
	if len(bp.giftBuffer) >= bp.maxBatchSize {
		bp.flushLocked()
	}
}

func (bp *BatchPersister) flushLoop() {
	defer bp.wg.Done()
	ticker := time.NewTicker(bp.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			bp.Flush()
		case <-bp.stopCh:
			bp.Flush() // final flush
			return
		}
	}
}

// Flush forces a flush of all buffered items.
func (bp *BatchPersister) Flush() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.flushLocked()
}

func (bp *BatchPersister) flushLocked() {
	if len(bp.dmBuffer) > 0 {
		bp.flushDanmakuLocked(bp.dmBuffer)
		bp.dmBuffer = bp.dmBuffer[:0]
	}
	if len(bp.giftBuffer) > 0 {
		bp.flushGiftsLocked(bp.giftBuffer)
		bp.giftBuffer = bp.giftBuffer[:0]
	}
}

func (bp *BatchPersister) flushDanmakuLocked(items []*domain.Danmaku) {
	for _, d := range items {
		p := task.PersistDanmakuPayload{
			ID:        d.ID,
			RoomID:    d.RoomID,
			UserID:    d.UserID,
			Content:   d.Content,
			Status:    string(d.Status),
			CreatedAt: d.CreatedAt,
		}
		payload, _ := p.MarshalJSON()
		if _, err := bp.client.Enqueue(asynq.NewTask(task.TypePersistDanmaku, payload)); err != nil {
			logger.L().Error("batch enqueue danmaku failed", zap.Error(err), zap.String("id", d.ID))
		}
	}
}

func (bp *BatchPersister) flushGiftsLocked(orderIDs []string) {
	for _, id := range orderIDs {
		p := task.SettleGiftPayload{OrderID: id}
		payload, _ := p.MarshalJSON()
		if _, err := bp.client.Enqueue(asynq.NewTask(task.TypeSettleGift, payload)); err != nil {
			logger.L().Error("batch enqueue gift failed", zap.Error(err), zap.String("order_id", id))
		}
	}
}

// Stop stops the batch persister and flushes remaining items.
func (bp *BatchPersister) Stop() {
	close(bp.stopCh)
	bp.wg.Wait()
}