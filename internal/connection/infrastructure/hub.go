package infrastructure

import (
	"sync"

	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
)

// Hub is an in-memory implementation of domain.Hub.
type Hub struct {
	rooms map[string]*roomBucket
	mu    sync.RWMutex
}

type roomBucket struct {
	sessions map[string]*domain.Session
	mu       sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*roomBucket)}
}

// Register adds a session to its room.
func (h *Hub) Register(s *domain.Session) {
	h.bucket(s.RoomID).add(s)
	metrics.WSConnections.WithLabelValues(s.RoomID).Inc()
}

// Unregister removes a session from its room.
func (h *Hub) Unregister(s *domain.Session) {
	h.bucket(s.RoomID).remove(s.ID)
	metrics.WSConnections.WithLabelValues(s.RoomID).Dec()
}

// Broadcast sends a message to all sessions in a room.
func (h *Hub) Broadcast(roomID string, msg []byte) {
	h.bucket(roomID).broadcast(msg)
}

// SendToUser delivers a message to every session of a specific user in a room.
func (h *Hub) SendToUser(roomID string, userID string, msg []byte) {
	h.bucket(roomID).sendToUser(userID, msg)
}

// RoomUserCount returns the number of connected sessions in a room.
func (h *Hub) RoomUserCount(roomID string) int {
	return h.bucket(roomID).count()
}

func (h *Hub) bucket(roomID string) *roomBucket {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, ok := h.rooms[roomID]
	if !ok {
		b = &roomBucket{sessions: make(map[string]*domain.Session)}
		h.rooms[roomID] = b
	}
	return b
}

func (b *roomBucket) add(s *domain.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessions[s.ID] = s
}

func (b *roomBucket) remove(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, id)
}

func (b *roomBucket) broadcast(msg []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.sessions {
		select {
		case s.Send <- msg:
		default:
			// Drop message if send buffer is full to avoid blocking.
		}
	}
}

// sendToUser delivers msg to every session whose UserID matches in this room.
func (b *roomBucket) sendToUser(userID string, msg []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.sessions {
		if s.UserID != userID {
			continue
		}
		select {
		case s.Send <- msg:
		default:
			// Drop message if send buffer is full to avoid blocking.
		}
	}
}

func (b *roomBucket) count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessions)
}

// Compile-time check.
var _ domain.Hub = (*Hub)(nil)

// Close releases resources (no-op for in-memory hub).
func (h *Hub) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, bucket := range h.rooms {
		bucket.mu.Lock()
		for _, s := range bucket.sessions {
			close(s.Send)
		}
		bucket.sessions = nil
		bucket.mu.Unlock()
	}
	h.rooms = nil
	return nil
}
