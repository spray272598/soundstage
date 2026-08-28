package transport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spray272598/soundstage/internal/room/application"
	"github.com/spray272598/soundstage/internal/room/domain"
)

// RoomHandler exposes room HTTP endpoints.
type RoomHandler struct {
	svc *application.RoomService
}

// NewRoomHandler creates a new RoomHandler.
func NewRoomHandler(svc *application.RoomService) *RoomHandler {
	return &RoomHandler{svc: svc}
}

// Register mounts room routes on the given gin engine.
func (h *RoomHandler) Register(r *gin.Engine) {
	api := r.Group("/api/v1/rooms")
	{
		api.POST("", h.create)
		api.POST("/:id/open", h.open)
		api.POST("/:id/close", h.close)
		api.GET("/:id", h.get)
		api.GET("", h.list)
	}
}

type createRoomRequest struct {
	AnchorID string `json:"anchor_id" binding:"required"`
	Title    string `json:"title" binding:"required"`
}

type roomResponse struct {
	ID        string  `json:"id"`
	AnchorID  string  `json:"anchor_id"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	StartedAt *string `json:"started_at,omitempty"`
	EndedAt   *string `json:"ended_at,omitempty"`
}

func (h *RoomHandler) create(c *gin.Context) {
	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	room, err := h.svc.Create(c.Request.Context(), application.CreateRoomRequest{
		AnchorID: req.AnchorID,
		Title:    req.Title,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toResponse(room))
}

func (h *RoomHandler) open(c *gin.Context) {
	room, err := h.svc.Open(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == domain.ErrRoomNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toResponse(room))
}

func (h *RoomHandler) close(c *gin.Context) {
	room, err := h.svc.Close(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == domain.ErrRoomNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toResponse(room))
}

func (h *RoomHandler) get(c *gin.Context) {
	room, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if err == domain.ErrRoomNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toResponse(room))
}

func (h *RoomHandler) list(c *gin.Context) {
	// TODO: parse limit/offset from query params.
	rooms, err := h.svc.List(c.Request.Context(), 20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := make([]roomResponse, len(rooms))
	for i, r := range rooms {
		resp[i] = toResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"items": resp})
}

func toResponse(r *domain.Room) roomResponse {
	resp := roomResponse{
		ID:        r.ID,
		AnchorID:  r.AnchorID,
		Title:     r.Title,
		Status:    string(r.Status),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
	if r.StartedAt != nil {
		s := r.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &s
	}
	if r.EndedAt != nil {
		e := r.EndedAt.Format(time.RFC3339)
		resp.EndedAt = &e
	}
	return resp
}
