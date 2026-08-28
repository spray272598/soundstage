package domain

import (
	"fmt"
	"time"
)

// Status represents the lifecycle state of a live room.
type Status string

const (
	StatusPending Status = "pending"
	StatusLive    Status = "live"
	StatusClosed  Status = "closed"
)

// Room is the aggregate root of the room bounded context.
type Room struct {
	ID        string
	AnchorID  string
	Title     string
	Status    Status
	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time
}

// NewRoom creates a new pending room.
func NewRoom(id, anchorID, title string) *Room {
	now := time.Now().UTC()
	return &Room{
		ID:        id,
		AnchorID:  anchorID,
		Title:     title,
		Status:    StatusPending,
		CreatedAt: now,
	}
}

// Open transitions the room from pending to live.
func (r *Room) Open() error {
	if r.Status != StatusPending {
		return fmt.Errorf("room %s cannot be opened from status %s", r.ID, r.Status)
	}
	now := time.Now().UTC()
	r.Status = StatusLive
	r.StartedAt = &now
	return nil
}

// Close transitions the room from live to closed.
func (r *Room) Close() error {
	if r.Status != StatusLive {
		return fmt.Errorf("room %s cannot be closed from status %s", r.ID, r.Status)
	}
	now := time.Now().UTC()
	r.Status = StatusClosed
	r.EndedAt = &now
	return nil
}
