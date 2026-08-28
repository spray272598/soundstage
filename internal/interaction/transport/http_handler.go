package transport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spray272598/soundstage/internal/interaction/application"
	"github.com/spray272598/soundstage/internal/interaction/domain"
)

// InteractionHandler exposes the interaction REST endpoints. These are an
// alternative entry point to the WebSocket ingest path; both call the same
// InterService so behavior is identical.
type InteractionHandler struct {
	svc *application.InterService
}

// NewInteractionHandler creates a new InteractionHandler.
func NewInteractionHandler(svc *application.InterService) *InteractionHandler {
	return &InteractionHandler{svc: svc}
}

// Register mounts the interaction routes under /api/v1.
func (h *InteractionHandler) Register(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		api.GET("/gifts", h.listGifts)
		rooms := api.Group("/rooms/:id")
		{
			rooms.POST("/danmaku", h.sendDanmaku)
			rooms.POST("/gifts", h.sendGift)
			rooms.POST("/like", h.sendLike)
			rooms.GET("/rank", h.rank)
			rooms.GET("/likes", h.likes)
		}
	}
}

func (h *InteractionHandler) listGifts(c *gin.Context) {
	gifts, err := h.svc.ListGifts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]giftResponse, len(gifts))
	for i, g := range gifts {
		items[i] = toGiftResponse(g)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type sendDanmakuRequest struct {
	Text   string `json:"text" binding:"required"`
	UserID string `json:"user_id"`
}

func (h *InteractionHandler) sendDanmaku(c *gin.Context) {
	var req sendDanmakuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := req.UserID
	if userID == "" {
		userID = c.Query("user_id")
	}
	d, err := h.svc.ProcessDanmaku(c.Request.Context(), c.Param("id"), userID, req.Text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         d.ID,
		"room_id":    d.RoomID,
		"user_id":    d.UserID,
		"content":    d.Content,
		"status":     string(d.Status),
		"created_at": d.CreatedAt.Format(time.RFC3339),
	})
}

type sendGiftRequest struct {
	GiftID         string `json:"gift_id" binding:"required"`
	Count          int    `json:"count" binding:"required"`
	UserID         string `json:"user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *InteractionHandler) sendGift(c *gin.Context) {
	var req sendGiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := req.UserID
	if userID == "" {
		userID = c.Query("user_id")
	}
	order, err := h.svc.ProcessGift(c.Request.Context(), c.Param("id"), userID, req.GiftID, req.Count, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"order_id":     order.ID,
		"room_id":      order.RoomID,
		"gift_id":      order.GiftID,
		"gift_name":    order.GiftName,
		"count":        order.Count,
		"total_amount": order.TotalAmount,
		"status":       string(order.Status),
	})
}

func (h *InteractionHandler) sendLike(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		userID = c.PostForm("user_id")
	}
	if err := h.svc.ProcessLike(c.Request.Context(), c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *InteractionHandler) rank(c *gin.Context) {
	period, err := parsePeriod(c.Query("period"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entries, err := h.svc.RankTopN(c.Request.Context(), c.Param("id"), period, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]rankResponse, len(entries))
	for i, e := range entries {
		items[i] = rankResponse{UserID: e.UserID, Amount: e.Amount}
	}
	c.JSON(http.StatusOK, gin.H{"period": string(period), "items": items})
}

func (h *InteractionHandler) likes(c *gin.Context) {
	n, err := h.svc.LikeCount(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

func parsePeriod(s string) (domain.Period, error) {
	switch domain.Period(s) {
	case domain.PeriodDay, domain.PeriodWeek, domain.PeriodMonth:
		return domain.Period(s), nil
	case "":
		return domain.PeriodDay, nil
	default:
		return "", domain.ErrInvalidPeriod
	}
}

type giftResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Icon   string `json:"icon"`
	Status string `json:"status"`
}

type rankResponse struct {
	UserID string `json:"user_id"`
	Amount int64  `json:"amount"`
}

func toGiftResponse(g *domain.Gift) giftResponse {
	return giftResponse{
		ID:     g.ID,
		Name:   g.Name,
		Price:  g.Price,
		Icon:   g.Icon,
		Status: string(g.Status),
	}
}
