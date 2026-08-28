package domain

import "context"

// MicLinkRepository persists co-host (mic-link) sessions.
type MicLinkRepository interface {
	Create(ctx context.Context, m *MicLink) error
	GetByID(ctx context.Context, id string) (*MicLink, error)
	// GetActiveByRoom returns the open session (requesting/connected) for a
	// room, or ErrMicLinkNotFound when none is active.
	GetActiveByRoom(ctx context.Context, roomID string) (*MicLink, error)
	Update(ctx context.Context, m *MicLink) error
}

// PKSessionRepository persists cross-room PK battles.
type PKSessionRepository interface {
	Create(ctx context.Context, p *PKSession) error
	GetByID(ctx context.Context, id string) (*PKSession, error)
	// GetActiveByRoom returns the ongoing PK that involves the given room, or
	// ErrPKNotFound when the room is not currently battling.
	GetActiveByRoom(ctx context.Context, roomID string) (*PKSession, error)
	Update(ctx context.Context, p *PKSession) error
}
