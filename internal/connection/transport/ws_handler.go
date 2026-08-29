package transport

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spray272598/soundstage/internal/connection/application"
	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/config"
	"github.com/spray272598/soundstage/internal/pkg/id"
)

// WSHandler handles WebSocket upgrades.
type WSHandler struct {
	svc      *application.ConnectionService
	upgrader websocket.Upgrader
	cfg      config.WebSocketConfig
}

// NewWSHandler creates a new WSHandler with production-grade settings.
func NewWSHandler(svc *application.ConnectionService, cfg config.WebSocketConfig) *WSHandler {
	readBufSize := cfg.ReadBufferSize
	if readBufSize == 0 {
		readBufSize = 4096
	}
	writeBufSize := cfg.WriteBufferSize
	if writeBufSize == 0 {
		writeBufSize = 4096
	}

	return &WSHandler{
		svc: svc,
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			ReadBufferSize:    readBufSize,
			WriteBufferSize:   writeBufSize,
			EnableCompression: cfg.EnableCompression,
		},
	}
}

// Register mounts the websocket route.
func (h *WSHandler) Register(r *gin.Engine) {
	r.GET("/ws/:room_id", h.handle)
	r.GET("/api/v1/rooms/:room_id/online", h.onlineCount)
}

func (h *WSHandler) handle(c *gin.Context) {
	roomID := c.Param("room_id")
	userID := c.Query("user_id")
	if userID == "" {
		userID = id.New()
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// Set read limit to prevent OOM from large messages
	conn.SetReadLimit(h.cfg.MaxMessageSize)

	ctx, cancel := context.WithCancel(c.Request.Context())
	session := &domain.Session{
		ID:     id.New(),
		UserID: userID,
		RoomID: roomID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Done:   ctx,
		Cancel: cancel,
	}
	h.svc.Handle(session)
}

func (h *WSHandler) onlineCount(c *gin.Context) {
	count := h.svc.RoomUserCount(c.Param("room_id"))
	c.JSON(http.StatusOK, gin.H{"count": count})
}
