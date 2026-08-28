package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/event"
)

// BroadcastHandler consumes Kafka broadcast events and pushes them to local sessions.
type BroadcastHandler struct {
	hub domain.Hub
}

// NewBroadcastHandler creates a new BroadcastHandler.
func NewBroadcastHandler(hub domain.Hub) *BroadcastHandler {
	return &BroadcastHandler{hub: hub}
}

// Handle implements pkgkafka.Handler.
func (h *BroadcastHandler) Handle(ctx context.Context, topic string, key string, payload []byte) error {
	var evt event.BroadcastEnvelope
	if err := json.Unmarshal(payload, &evt); err != nil {
		return err
	}
	out, err := json.Marshal(domain.Message{
		Type:    evt.Type,
		Payload: evt.Payload,
	})
	if err != nil {
		return err
	}
	h.hub.Broadcast(evt.RoomID, out)
	return nil
}
