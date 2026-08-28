package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spray272598/soundstage/internal/connection/domain"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"github.com/spray272598/soundstage/internal/pkg/event"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// ConnectionService manages the lifecycle of a WebSocket connection.
type ConnectionService struct {
	hub         domain.Hub
	producer    pkgkafka.Producer
	ingestTopic string
}

// NewConnectionService creates a new ConnectionService.
func NewConnectionService(hub domain.Hub, producer pkgkafka.Producer, ingestTopic string) *ConnectionService {
	return &ConnectionService{
		hub:         hub,
		producer:    producer,
		ingestTopic: ingestTopic,
	}
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

		metrics.WSMessagesTotal.WithLabelValues(msg.Type, "in").Inc()

		switch msg.Type {
		case "heartbeat":
			// Pong handler already refreshed the read deadline.
		case "chat", "gift", "like":
			// Hand interactive messages to the interaction context through the
			// event bus. This keeps the gateway decoupled and lets the same
			// processing path serve both WebSocket and REST entry points.
			s.publishIngest(session, msg.Type, msg.Payload)
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
			metrics.WSMessagesTotal.WithLabelValues("broadcast", "out").Inc()

		case <-ticker.C:
			session.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := session.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// publishIngest forwards an inbound client message to the ingest topic.
func (s *ConnectionService) publishIngest(session *domain.Session, msgType string, payload []byte) {
	env := event.InboundEnvelope{
		Type:    msgType,
		RoomID:  session.RoomID,
		UserID:  session.UserID,
		Payload: payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		logger.L().Error("failed to marshal ingest envelope", zap.Error(err))
		return
	}
	// Fire-and-forget: ingestion failures are logged, never block the pump.
	if err := s.producer.Publish(context.Background(), s.ingestTopic, session.RoomID, data); err != nil {
		logger.L().Error("failed to publish ingest event",
			zap.Error(err),
			zap.String("room", session.RoomID),
			zap.String("type", msgType))
	}
}
