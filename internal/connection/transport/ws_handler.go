package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spray272598/soundstage/internal/connection/application"
	"github.com/spray272598/soundstage/internal/connection/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
)

// WSHandler handles WebSocket upgrades.
type WSHandler struct {
	svc      *application.ConnectionService
	upgrader websocket.Upgrader
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(svc *application.ConnectionService) *WSHandler {
	return &WSHandler{
		svc: svc,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
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

	session := &domain.Session{
		ID:     id.New(),
		UserID: userID,
		RoomID: roomID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}
	h.svc.Handle(session)
}

func (h *WSHandler) onlineCount(c *gin.Context) {
	count := h.svc.RoomUserCount(c.Param("room_id"))
	c.JSON(http.StatusOK, gin.H{"count": count})
}
