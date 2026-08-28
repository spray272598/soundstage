// Package event defines the wire envelopes exchanged over Kafka between the
// connection gateway and the interaction context. Keeping them in the shared
// kernel avoids import cycles between bounded contexts.
package event

import "encoding/json"

// InboundEnvelope is published by the connection gateway to the ingest topic.
// It carries a raw client message together with the session routing metadata.
type InboundEnvelope struct {
	Type    string          `json:"type"`
	RoomID  string          `json:"room_id"`
	UserID  string          `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

// BroadcastEnvelope is published to the broadcast topic for client fan-out.
// The connection gateway consumes it and pushes the payload to every session
// in the target room.
type BroadcastEnvelope struct {
	RoomID  string          `json:"room_id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
