package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// RedisHub is a distributed implementation of domain.Hub using Redis for
// session registry and Pub/Sub for cross-gateway message routing.
type RedisHub struct {
	rdb       *redis.Client
	gatewayID string
	localHub  *Hub // local in-memory hub for sessions on this gateway
	pubsub    *redis.PubSub
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	closed    bool
}

// SessionMeta stores session routing info in Redis.
type SessionMeta struct {
	GatewayID   string `json:"gateway_id"`
	RoomID      string `json:"room_id"`
	UserID      string `json:"user_id"`
	SessionID   string `json:"session_id"`
	ConnectedAt int64  `json:"connected_at"`
}

// BroadcastMsg is the message format for Redis Pub/Sub.
type BroadcastMsg struct {
	GatewayID string          `json:"gateway_id"` // origin gateway; receivers skip their own
	RoomID    string          `json:"room_id"`
	Type      string          `json:"type"`         // original domain.Message type (e.g. "danmaku")
	To        string          `json:"to,omitempty"` // empty = broadcast, set = unicast to user
	Payload   json.RawMessage `json:"payload"`      // raw domain.Message bytes
}

// NewRedisHub creates a new distributed Hub.
func NewRedisHub(rdb *redis.Client, gatewayID string) *RedisHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisHub{
		rdb:       rdb,
		gatewayID: gatewayID,
		localHub:  NewHub(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// broadcastChannel is the single global channel every gateway subscribes to.
// Each gateway delivers only to its own local sessions, so a room's members
// across all gateways each receive the message exactly once.
const broadcastChannel = "soundstage:broadcast"

// Start begins listening for cross-gateway messages.
func (h *RedisHub) Start() error {
	h.pubsub = h.rdb.Subscribe(h.ctx, broadcastChannel)

	h.wg.Add(1)
	go h.listenBroadcast()

	logger.L().Info("redis hub started", zap.String("gateway_id", h.gatewayID), zap.String("channel", broadcastChannel))
	return nil
}

// Stop shuts down the Redis hub.
func (h *RedisHub) Stop() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()

	h.cancel()
	if h.pubsub != nil {
		_ = h.pubsub.Close()
	}
	h.wg.Wait()
	return h.localHub.Close()
}

func (h *RedisHub) listenBroadcast() {
	defer h.wg.Done()
	ch := h.pubsub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.handleRemoteBroadcast(msg.Payload)
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *RedisHub) handleRemoteBroadcast(payload string) {
	var bm BroadcastMsg
	if err := json.Unmarshal([]byte(payload), &bm); err != nil {
		logger.L().Warn("invalid broadcast message", zap.Error(err))
		return
	}

	// Skip messages we published ourselves: we already delivered them locally
	// inside Broadcast/SendToUser, so re-delivering would duplicate.
	if bm.GatewayID == h.gatewayID {
		return
	}

	out, err := json.Marshal(domain.Message{
		Type:    bm.Type,
		Payload: bm.Payload,
	})
	if err != nil {
		logger.L().Error("marshal broadcast payload", zap.Error(err))
		return
	}

	if bm.To != "" {
		h.localHub.SendToUser(bm.RoomID, bm.To, out)
	} else {
		h.localHub.Broadcast(bm.RoomID, out)
	}
}

// Register adds a session to both local hub and Redis registry.
func (h *RedisHub) Register(s *domain.Session) {
	h.localHub.Register(s)

	meta := SessionMeta{
		GatewayID:   h.gatewayID,
		RoomID:      s.RoomID,
		UserID:      s.UserID,
		SessionID:   s.ID,
		ConnectedAt: time.Now().Unix(),
	}
	data, _ := json.Marshal(meta)
	key := fmt.Sprintf("soundstage:session:%s", s.ID)
	h.rdb.Set(h.ctx, key, data, 24*time.Hour)

	// Add to room index for RoomUserCount
	roomKey := fmt.Sprintf("soundstage:room:%s:sessions", s.RoomID)
	h.rdb.SAdd(h.ctx, roomKey, s.ID)
	h.rdb.Expire(h.ctx, roomKey, 25*time.Hour)

	metrics.WSConnections.WithLabelValues(s.RoomID).Inc()
}

// Unregister removes a session from both local hub and Redis registry.
func (h *RedisHub) Unregister(s *domain.Session) {
	h.localHub.Unregister(s)

	key := fmt.Sprintf("soundstage:session:%s", s.ID)
	h.rdb.Del(h.ctx, key)

	roomKey := fmt.Sprintf("soundstage:room:%s:sessions", s.RoomID)
	h.rdb.SRem(h.ctx, roomKey, s.ID)

	metrics.WSConnections.WithLabelValues(s.RoomID).Dec()
}

// Broadcast sends a message to all sessions in a room across all gateways.
func (h *RedisHub) Broadcast(roomID string, msg []byte) {
	// Deliver locally first
	h.localHub.Broadcast(roomID, msg)

	// Publish to other gateways via Redis Pub/Sub
	h.publishBroadcast(roomID, "", msg)
}

// SendToUser delivers a message to a specific user in a room across all gateways.
func (h *RedisHub) SendToUser(roomID string, userID string, msg []byte) {
	// Deliver locally first
	h.localHub.SendToUser(roomID, userID, msg)

	// Publish to other gateways
	h.publishBroadcast(roomID, userID, msg)
}

func (h *RedisHub) publishBroadcast(roomID, to string, payload []byte) {
	// Preserve the original event type so remote gateways forward it intact.
	var m domain.Message
	_ = json.Unmarshal(payload, &m)

	bm := BroadcastMsg{
		GatewayID: h.gatewayID,
		RoomID:    roomID,
		Type:      m.Type,
		To:        to,
		Payload:   payload,
	}
	data, _ := json.Marshal(bm)

	// Publish to the single global channel; every gateway subscribes and
	// delivers only to its local sessions (skipping its own echoes).
	if err := h.rdb.Publish(h.ctx, broadcastChannel, data).Err(); err != nil {
		logger.L().Error("publish broadcast failed", zap.Error(err), zap.String("room", roomID))
	}
}

// RoomUserCount returns total connected sessions in a room across all gateways.
func (h *RedisHub) RoomUserCount(roomID string) int {
	// Fast path: local count (used as a fallback when Redis is unavailable).
	localCount := h.localHub.RoomUserCount(roomID)

	// The room session set is shared across every gateway and already includes
	// this gateway's local sessions, so SCARD is the authoritative total.
	// Do NOT add localCount on top, or local sessions get counted twice.
	roomKey := fmt.Sprintf("soundstage:room:%s:sessions", roomID)
	total, err := h.rdb.SCard(h.ctx, roomKey).Result()
	if err != nil {
		logger.L().Warn("redis scard failed", zap.Error(err), zap.String("room", roomID))
		return localCount
	}

	return int(total)
}

// LocalHub returns the underlying in-memory hub for direct access.
func (h *RedisHub) LocalHub() *Hub {
	return h.localHub
}

// Close implements domain.Hub.
func (h *RedisHub) Close() error {
	return h.Stop()
}

// Compile-time check.
var _ domain.Hub = (*RedisHub)(nil)
