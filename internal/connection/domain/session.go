package domain

import "github.com/gorilla/websocket"

// Session represents a single WebSocket connection in a room.
type Session struct {
	ID     string
	UserID string
	RoomID string
	Conn   *websocket.Conn
	Send   chan []byte
}
