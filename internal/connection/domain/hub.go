package domain

// Hub manages session registration, unregistration and room broadcast.
type Hub interface {
	Register(session *Session)
	Unregister(session *Session)
	Broadcast(roomID string, message []byte)
	RoomUserCount(roomID string) int
}
