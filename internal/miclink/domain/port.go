package domain

import (
	"context"
	"encoding/json"
	"time"
)

// SignalingRelay forwards a WebRTC signaling message (offer/answer/ice) from
// one peer to a specific target user. It is implemented by the Kafka broadcaster
// writing to the broadcast topic with a targeted To recipient.
type SignalingRelay interface {
	Relay(ctx context.Context, roomID string, toUserID string, fromUserID string, signalType string, payload json.RawMessage) error
}

// Broadcaster publishes a client-facing message to a room. It is the same
// contract used by the interaction context, redeclared here so the miclink
// bounded context stays free of a cross-context import.
type Broadcaster interface {
	Broadcast(ctx context.Context, roomID, msgType string, payload json.RawMessage) error
}

// TaskEnqueuer schedules PK background work (settle, countdown notice). Keeping
// it behind a port lets tests run with an in-memory fake.
type TaskEnqueuer interface {
	// EnqueuePKSettle schedules settlement of a PK session at runAt in the future.
	EnqueuePKSettle(ctx context.Context, sessionID string, runAt time.Duration) error
	// EnqueuePKCountdown schedules the last-seconds warning at runAt in the future.
	EnqueuePKCountdown(ctx context.Context, sessionID string, runAt time.Duration) error
}

// Locker guards PK state transitions against concurrent mutations from the
// two participating rooms. Unlock releases the lock.
type Locker interface {
	Lock(ctx context.Context, key string) (func(), error)
}
