// Package asynqworker wires the PK background task handlers (settle, countdown)
// to the shared asynq server mux.
package asynqworker

import (
	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/miclink/task"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// Deps bundles every port the miclink task handlers need.
type Deps struct {
	PKs         domain.PKSessionRepository
	Broadcaster domain.Broadcaster
	Locker      domain.Locker
	RDB         *redis.Client
}

// Register installs the miclink handlers on the shared asynq mux.
func Register(mux *asynq.ServeMux, d Deps) {
	mux.Handle(task.TypePKSettle, asynq.HandlerFunc(d.handlePKSettle))
	mux.Handle(task.TypePKCountdown, asynq.HandlerFunc(d.handlePKCountdown))
}
