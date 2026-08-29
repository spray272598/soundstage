package merger

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// Sender is the interface for sending merged messages.
type Sender interface {
	SendToRoom(ctx context.Context, roomID string, msg []byte) error
	Broadcast(ctx context.Context, msg []byte) error
}

// Merger coordinates multiple merge workers for room and broadcast messages.
type Merger struct {
	sender       Sender
	roomWorkers  []*MergeWorker
	broadcastWorker *MergeWorker
	workerCount  int
	mu           sync.RWMutex
	closed       bool
}

// NewMerger creates a new Merger with the given configuration.
func NewMerger(sender Sender, cfg config.MergerConfig) *Merger {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.ChannelSize <= 0 {
		cfg.ChannelSize = 1024
	}
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 100
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 10 * time.Millisecond
	}

	m := &Merger{
		sender:      sender,
		workerCount: cfg.WorkerCount,
		roomWorkers: make([]*MergeWorker, cfg.WorkerCount),
	}

	// Create room workers (each handles a subset of rooms via consistent hashing)
	for i := 0; i < cfg.WorkerCount; i++ {
		m.roomWorkers[i] = NewMergeWorker(i, MergerTypeRoom, cfg)
	}

	// Create single broadcast worker
	m.broadcastWorker = NewMergeWorker(0, MergerTypeBroadcast, cfg)

	return m
}

// Start begins all merge workers.
func (m *Merger) Start(ctx context.Context) {
	for _, w := range m.roomWorkers {
		w.Start(ctx)
	}
	m.broadcastWorker.Start(ctx)
	logger.L().Info("merger started", zap.Int("workers", m.workerCount))
}

// Stop gracefully shuts down all workers.
func (m *Merger) Stop() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.mu.Unlock()

	for range m.roomWorkers {
		// Workers flush on context cancellation
	}
	m.broadcastWorker = nil
	logger.L().Info("merger stopped")
}

// PushRoom queues a message for a specific room.
func (m *Merger) PushRoom(ctx context.Context, roomID string, msgType string, payload []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMergerClosed
	}

	worker := m.getRoomWorker(roomID)
	msg := PushContext{
		RoomID:  roomID,
		Type:    msgType,
		Payload: payload,
	}
	return worker.Push(ctx, msg)
}

// PushBroadcast queues a broadcast message (to all rooms).
func (m *Merger) PushBroadcast(ctx context.Context, msgType string, payload []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMergerClosed
	}

	msg := PushContext{
		RoomID:  "",
		Type:    msgType,
		Payload: payload,
	}
	return m.broadcastWorker.Push(ctx, msg)
}

// getRoomWorker returns the worker responsible for a room using consistent hashing.
func (m *Merger) getRoomWorker(roomID string) *MergeWorker {
	h := fnv.New32a()
	h.Write([]byte(roomID))
	idx := int(h.Sum32()) % m.workerCount
	return m.roomWorkers[idx]
}

// SetSenderCallbacks configures the actual send functions.
func (m *Merger) SetSenderCallbacks(
	sendToRoom func(ctx context.Context, roomID string, msg []byte) error,
	broadcast func(ctx context.Context, msg []byte) error,
) {
	for _, w := range m.roomWorkers {
		w.SetSendFuncs(sendToRoom, broadcast)
	}
	m.broadcastWorker.SetSendFuncs(sendToRoom, broadcast)
}

var ErrMergerClosed = &mergerError{"merger closed"}