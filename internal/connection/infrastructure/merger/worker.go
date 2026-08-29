package merger

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// MergerType represents the type of message merging.
type MergerType int

const (
	// MergerTypeRoom merges messages per room.
	MergerTypeRoom MergerType = iota
	// MergerTypeBroadcast merges broadcast messages (to all rooms).
	MergerTypeBroadcast
)

// PushContext represents a message to be merged.
type PushContext struct {
	RoomID  string          // empty = broadcast
	Type    string          // message type (danmaku, gift, like, etc.)
	Payload json.RawMessage // message payload
}

// PushBatch represents a batch of messages to be sent together.
type PushBatch struct {
	RoomID     string
	MergerType MergerType
	Items      []PushContext
	Timer      *time.Timer
}

// MergeWorker processes messages for a specific merger type.
type MergeWorker struct {
	mergerType   MergerType
	contextChan  chan PushContext
	timeoutChan  chan *PushBatch
	roomBatches  map[string]*PushBatch // roomID -> batch
	broadcastBatch *PushBatch          // for broadcast type
	mu           sync.Mutex
	workerIdx    int
	maxBatchSize int
	maxDelay     time.Duration

	// Callbacks for sending merged messages
	sendToRoomFn  func(ctx context.Context, roomID string, msg []byte) error
	sendBroadcastFn func(ctx context.Context, msg []byte) error
}

// NewMergeWorker creates a new merge worker.
func NewMergeWorker(workerIdx int, mergerType MergerType, cfg config.MergerConfig) *MergeWorker {
	return &MergeWorker{
		workerIdx:    workerIdx,
		mergerType:   mergerType,
		contextChan:  make(chan PushContext, cfg.ChannelSize),
		timeoutChan:  make(chan *PushBatch, cfg.ChannelSize),
		roomBatches:  make(map[string]*PushBatch),
		maxBatchSize: cfg.MaxBatchSize,
		maxDelay:     cfg.MaxDelay,
	}
}

// Start begins the merge worker loop.
func (w *MergeWorker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Push adds a message to the merge queue.
func (w *MergeWorker) Push(ctx context.Context, msg PushContext) error {
	select {
	case w.contextChan <- msg:
		metrics.MergerPendingTotal.WithLabelValues(w.mergerType.String()).Inc()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		metrics.MergerChannelFullTotal.WithLabelValues(w.mergerType.String()).Inc()
		return ErrChannelFull
	}
}

func (w *MergeWorker) run(ctx context.Context) {
	for {
		select {
		case msg := <-w.contextChan:
			metrics.MergerPendingTotal.WithLabelValues(w.mergerType.String()).Dec()
			w.handleMessage(msg)

		case batch := <-w.timeoutChan:
			w.handleTimeout(batch)

		case <-ctx.Done():
			w.flushAll()
			return
		}
	}
}

func (w *MergeWorker) handleMessage(msg PushContext) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var batch *PushBatch
	isNewBatch := false

	if w.mergerType == MergerTypeRoom {
		// Room-specific batching
		var existed bool
		batch, existed = w.roomBatches[msg.RoomID]
		if !existed {
			batch = &PushBatch{
				RoomID:     msg.RoomID,
				MergerType: MergerTypeRoom,
				Items:      make([]PushContext, 0, w.maxBatchSize),
			}
			w.roomBatches[msg.RoomID] = batch
			isNewBatch = true
		}
	} else {
		// Broadcast batching (single batch for all broadcast messages)
		batch = w.broadcastBatch
		if batch == nil {
			batch = &PushBatch{
				MergerType: MergerTypeBroadcast,
				Items:      make([]PushContext, 0, w.maxBatchSize),
			}
			w.broadcastBatch = batch
			isNewBatch = true
		}
	}

	// Add message to batch
	batch.Items = append(batch.Items, msg)

	// Start timer for new batch
	if isNewBatch {
		batch.Timer = time.AfterFunc(w.maxDelay, func() {
			w.timeoutChan <- batch
		})
	}

	// Flush if batch is full
	if len(batch.Items) >= w.maxBatchSize {
		if batch.Timer != nil {
			batch.Timer.Stop()
		}
		w.flushBatchLocked(batch)
	}
}

func (w *MergeWorker) handleTimeout(batch *PushBatch) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify batch still exists and matches
	if w.mergerType == MergerTypeRoom {
		existing, ok := w.roomBatches[batch.RoomID]
		if !ok || existing != batch {
			return // Already flushed or replaced
		}
		delete(w.roomBatches, batch.RoomID)
	} else {
		if w.broadcastBatch != batch {
			return
		}
		w.broadcastBatch = nil
	}

	w.flushBatchLocked(batch)
}

func (w *MergeWorker) flushBatchLocked(batch *PushBatch) {
	if len(batch.Items) == 0 {
		return
	}

	// Build merged message
	merged := buildMergedMessage(batch.Items)
	if merged == nil {
		return
	}

	// Send based on type
	var err error
	if batch.MergerType == MergerTypeRoom {
		err = w.sendToRoomFn(context.Background(), batch.RoomID, merged)
		metrics.MergerRoomTotal.WithLabelValues("total").Add(float64(len(batch.Items)))
		if err != nil {
			metrics.MergerRoomTotal.WithLabelValues("failed").Add(float64(len(batch.Items)))
		}
	} else {
		err = w.sendBroadcastFn(context.Background(), merged)
		metrics.MergerBroadcastTotal.WithLabelValues("total").Add(float64(len(batch.Items)))
		if err != nil {
			metrics.MergerBroadcastTotal.WithLabelValues("failed").Add(float64(len(batch.Items)))
		}
	}

	if err != nil {
		logger.L().Error("merger flush failed",
			zap.String("type", batch.MergerType.String()),
			zap.String("room", batch.RoomID),
			zap.Int("count", len(batch.Items)),
			zap.Error(err))
	}
}

func (w *MergeWorker) flushAll() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush room batches
	for _, batch := range w.roomBatches {
		if batch.Timer != nil {
			batch.Timer.Stop()
		}
		w.flushBatchLocked(batch)
	}
	w.roomBatches = nil

	// Flush broadcast batch
	if w.broadcastBatch != nil {
		if w.broadcastBatch.Timer != nil {
			w.broadcastBatch.Timer.Stop()
		}
		w.flushBatchLocked(w.broadcastBatch)
		w.broadcastBatch = nil
	}
}

// buildMergedMessage creates a single message containing all batched items.
func buildMergedMessage(items []PushContext) json.RawMessage {
	if len(items) == 1 {
		// Single item, send as-is
		return items[0].Payload
	}

	// Multiple items: wrap in batch envelope
	type BatchEnvelope struct {
		Batch []PushContext `json:"batch"`
	}
	env := BatchEnvelope{Batch: items}
	data, _ := json.Marshal(env)
	return data
}

func (t MergerType) String() string {
	if t == MergerTypeRoom {
		return "room"
	}
	return "broadcast"
}

// SetSendFuncs sets the callback functions for sending merged messages.
func (w *MergeWorker) SetSendFuncs(
	sendToRoom func(ctx context.Context, roomID string, msg []byte) error,
	sendBroadcast func(ctx context.Context, msg []byte) error,
) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sendToRoomFn = sendToRoom
	w.sendBroadcastFn = sendBroadcast
}

// ErrChannelFull is returned when the merger channel is full.
var ErrChannelFull = &mergerError{"merger channel full"}

type mergerError struct {
	msg string
}

func (e *mergerError) Error() string { return e.msg }