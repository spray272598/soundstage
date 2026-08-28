package application

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// ConnectionService manages the lifecycle of a WebSocket connection.
type ConnectionService struct {
	hub domain.Hub
}

// NewConnectionService creates a new ConnectionService.
func NewConnectionService(hub domain.Hub) *ConnectionService {
	return &ConnectionService{hub: hub}
}

// Handle runs the read and write pumps for a session.
func (s *ConnectionService) Handle(session *domain.Session) {
	s.hub.Register(session)

	go s.writePump(session)
	s.readPump(session)
}

// RoomUserCount returns the number of connected sessions in a room.
func (s *ConnectionService) RoomUserCount(roomID string) int {
	return s.hub.RoomUserCount(roomID)
}

func (s *ConnectionService) readPump(session *domain.Session) {
	defer func() {
		s.hub.Unregister(session)
		session.Conn.Close()
	}()

	session.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	session.Conn.SetPongHandler(func(string) error {
		session.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, payload, err := session.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.L().Warn("websocket unexpected close",
					zap.Error(err),
					zap.String("session", session.ID),
					zap.String("room", session.RoomID))
			}
			break
		}

		var msg domain.Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "heartbeat":
			// Pong handler already refreshed the deadline.
		case "chat":
			// TODO: validate and broadcast via kafka/hub in Phase 1.4 / Phase 2.
			broadcast, _ := json.Marshal(domain.Message{
				Type:    "chat",
				Payload: msg.Payload,
			})
			s.hub.Broadcast(session.RoomID, broadcast)
		}
	}
}

func (s *ConnectionService) writePump(session *domain.Session) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		session.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-session.Send:
			session.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = session.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := session.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			session.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := session.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
