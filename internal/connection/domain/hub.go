package domain

// Hub manages session registration, unregistration and room broadcast.
type Hub interface {
	Register(session *Session)
	Unregister(session *Session)
	Broadcast(roomID string, message []byte)
	// SendToUser delivers a message to every session of a specific user in a
	// room. Used for targeted WebRTC signaling (offer/answer/ice).
	SendToUser(roomID string, userID string, message []byte)
	RoomUserCount(roomID string) int
	// Close releases resources.
	Close() error
}
