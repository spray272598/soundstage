package transport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spray272598/soundstage/internal/miclink/application"
	"github.com/spray272598/soundstage/internal/miclink/domain"
)

// MiclinkHandler exposes the mic-link (co-host) and PK (cross-room battle)
// REST endpoints. These are an alternative entry point to the WebSocket
// signaling/broadcast path; both call the same services so behavior is
// identical.
type MiclinkHandler struct {
	micSvc *application.MicLinkService
	pkSvc  *application.PKService
}

// NewMiclinkHandler creates a new MiclinkHandler.
func NewMiclinkHandler(micSvc *application.MicLinkService, pkSvc *application.PKService) *MiclinkHandler {
	return &MiclinkHandler{micSvc: micSvc, pkSvc: pkSvc}
}

// Register mounts the miclink routes under /api/v1.
func (h *MiclinkHandler) Register(r *gin.Engine) {
	api := r.Group("/api/v1")
	rooms := api.Group("/rooms/:id")
	{
		// Co-host (mic-link) within a single room.
		rooms.POST("/miclink/request", h.micRequest)
		rooms.POST("/miclink/accept", h.micAccept)
		rooms.POST("/miclink/close", h.micClose)

		// Cross-room PK battle.
		rooms.POST("/pk/challenge", h.pkChallenge)
		pk := rooms.Group("/pk/:session_id")
		{
			pk.POST("/accept", h.pkAccept)
			pk.POST("/start", h.pkStart)
			pk.POST("/score", h.pkScore)
			pk.POST("/finish", h.pkFinish)
			pk.GET("", h.pkState)
		}
	}
}

// --- co-host ---

type micRequestRequest struct {
	HostID string `json:"host_id" binding:"required"`
	GuestID string `json:"guest_id" binding:"required"`
}

func (h *MiclinkHandler) micRequest(c *gin.Context) {
	var req micRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	link, err := h.micSvc.Request(c.Request.Context(), c.Param("id"), req.HostID, req.GuestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toMicLinkResponse(link))
}

type micAcceptRequest struct {
	HostID string `json:"host_id" binding:"required"`
	GuestID string `json:"guest_id" binding:"required"`
}

func (h *MiclinkHandler) micAccept(c *gin.Context) {
	var req micAcceptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	link, err := h.micSvc.Accept(c.Request.Context(), c.Param("id"), req.GuestID, req.HostID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toMicLinkResponse(link))
}

func (h *MiclinkHandler) micClose(c *gin.Context) {
	if err := h.micSvc.Close(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "closed"})
}

// --- PK ---

type pkChallengeRequest struct {
	OpponentRoomID string `json:"opponent_room_id" binding:"required"`
	AnchorID       string `json:"anchor_id" binding:"required"`
	OpponentAnchorID string `json:"opponent_anchor_id" binding:"required"`
}

func (h *MiclinkHandler) pkChallenge(c *gin.Context) {
	var req pkChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pk, err := h.pkSvc.Challenge(c.Request.Context(), c.Param("id"), req.AnchorID, req.OpponentRoomID, req.OpponentAnchorID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

func (h *MiclinkHandler) pkAccept(c *gin.Context) {
	pk, err := h.pkSvc.Accept(c.Request.Context(), c.Param("session_id"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

func (h *MiclinkHandler) pkStart(c *gin.Context) {
	pk, err := h.pkSvc.Start(c.Request.Context(), c.Param("session_id"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

type pkScoreRequest struct {
	Amount int64 `json:"amount" binding:"required"`
}

func (h *MiclinkHandler) pkScore(c *gin.Context) {
	var req pkScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pk, err := h.pkSvc.Score(c.Request.Context(), c.Param("id"), req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if pk == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ignored", "reason": "room not in an active pk"})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

func (h *MiclinkHandler) pkFinish(c *gin.Context) {
	pk, err := h.pkSvc.Finish(c.Request.Context(), c.Param("session_id"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

func (h *MiclinkHandler) pkState(c *gin.Context) {
	pk, err := h.pkSvc.GetState(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toPKResponse(pk))
}

// --- responses ---

func toMicLinkResponse(m *domain.MicLink) gin.H {
	closed := ""
	if m.ClosedAt != nil {
		closed = m.ClosedAt.Format(time.RFC3339)
	}
	return gin.H{
		"session_id": m.ID,
		"room_id":    m.RoomID,
		"host_id":    m.HostID,
		"guest_id":   m.GuestID,
		"status":     string(m.Status),
		"created_at": m.CreatedAt.Format(time.RFC3339),
		"closed_at":  closed,
	}
}

func toPKResponse(p *domain.PKSession) gin.H {
	started, ends := "", ""
	if p.StartedAt != nil {
		started = p.StartedAt.Format(time.RFC3339)
	}
	if p.EndsAt != nil {
		ends = p.EndsAt.Format(time.RFC3339)
	}
	return gin.H{
		"session_id": p.ID,
		"room_a":     p.RoomAID,
		"room_b":     p.RoomBID,
		"status":     string(p.Status),
		"score_a":    p.ScoreA,
		"score_b":    p.ScoreB,
		"winner":     string(p.Winner),
		"started_at": started,
		"ends_at":    ends,
	}
}
