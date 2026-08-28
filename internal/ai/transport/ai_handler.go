// Package transport exposes the AI room-moderator HTTP + SSE endpoints.
package transport

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	aiapplication "github.com/spray272598/soundstage/internal/ai/application"
	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
)

// Handler serves the AI moderator REST + SSE routes.
type Handler struct {
	svc   *aiapplication.Service
	mode  string // "mock" | "llm"
	model string
}

// NewHandler builds an AI handler.
func NewHandler(svc *aiapplication.Service, mode, model string) *Handler {
	return &Handler{svc: svc, mode: mode, model: model}
}

// Register mounts the routes on the gin router.
func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/rooms/:id/ai")
	g.GET("/chat", h.Chat)
	g.POST("/auto-reply", h.AutoReply)

	r.POST("/ai/knowledge", h.IngestKnowledge)
	r.GET("/ai/health", h.Health)
}

// Chat streams a conversational turn with the AI room moderator over SSE.
//
//	GET /rooms/:id/ai/chat?message=...&user_id=...
func (h *Handler) Chat(c *gin.Context) {
	roomID := c.Param("id")
	message := c.Query("message")
	userID := c.Query("user_id")
	if userID == "" {
		userID = "host"
	}
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	ctx := c.Request.Context()
	metrics.AISSEConnections.Inc()
	defer metrics.AISSEConnections.Dec()

	_, _ = h.svc.Chat(ctx, roomID, userID, message, func(ev aidomain.AgentEvent) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		switch ev.Type {
		case aidomain.EventTextDelta:
			writeFrame(flusher, c.Writer, "text_delta", ev.Text)
		case aidomain.EventToolCall:
			writeFrame(flusher, c.Writer, "tool_call", fmt.Sprintf(`{"tool":%q,"args":%s}`, ev.ToolName, orEmpty(ev.ToolArgs)))
		case aidomain.EventToolResult:
			writeFrame(flusher, c.Writer, "tool_result", fmt.Sprintf(`{"tool":%q,"result":%q}`, ev.ToolName, ev.ToolResult))
		case aidomain.EventError:
			writeFrame(flusher, c.Writer, "error", ev.Text)
		case aidomain.EventDone:
			writeFrame(flusher, c.Writer, "done", ev.Text)
		}
	})
	// Ensure the stream is flushed even if no done event was observed.
	flusher.Flush()
}

// AutoReply returns a short contextual reply to a danmaku (non-streaming).
//
//	POST /rooms/:id/ai/auto-reply  { "user_id": "...", "content": "..." }
func (h *Handler) AutoReply(c *gin.Context) {
	roomID := c.Param("id")
	var body struct {
		UserID  string `json:"user_id"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	reply, err := h.svc.AutoReply(c.Request.Context(), roomID, body.UserID, body.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

// IngestKnowledge adds a document to the RAG knowledge base.
//
//	POST /ai/knowledge  { "title": "...", "text": "..." }
func (h *Handler) IngestKnowledge(c *gin.Context) {
	var body struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}
	if err := h.svc.IngestKnowledge(c.Request.Context(), body.Title, body.Text); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "indexed"})
}

// Health reports the AI mode and model.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"mode":  h.mode,
		"model": h.model,
	})
}

// writeFrame writes a single SSE event and flushes.
func writeFrame(flusher http.Flusher, w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

// orEmpty returns the raw JSON string or "{}" when empty.
func orEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}
