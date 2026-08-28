// Package task defines the asynq task types and payloads shared by the miclink
// enqueuer (application/infrastructure) and the worker. Keeping them here
// avoids an infrastructure -> application import cycle.
package task

// Asynq task type names for the miclink context.
const (
	// TypePKSettle finalizes an ongoing PK battle when its countdown expires.
	TypePKSettle = "miclink:pk_settle"
	// TypePKCountdown warns clients that the PK deadline is near.
	TypePKCountdown = "miclink:pk_countdown"
)

// PKSettlePayload is the payload for TypePKSettle.
type PKSettlePayload struct {
	SessionID string
}

// PKCountdownPayload is the payload for TypePKCountdown.
type PKCountdownPayload struct {
	SessionID string
}
