package domain

// BroadcastEvent is the event published to Kafka for cross-node fan-out.
type BroadcastEvent struct {
	RoomID  string `json:"room_id"`
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}
