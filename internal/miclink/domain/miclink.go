// Package domain holds the miclink bounded context's entities, aggregates,
// the PK state machine, and the ports (interfaces) that infrastructure must
// implement. The domain layer has no dependency on any external framework.
package domain

import "time"

// MicLinkStatus is the lifecycle state of an intra-room co-host session.
type MicLinkStatus string

const (
	// MicLinkStatusRequesting means a guest asked to join the host's mic and
	// is waiting for the host to accept or reject.
	MicLinkStatusRequesting MicLinkStatus = "requesting"
	// MicLinkStatusConnected means the co-host session is live.
	MicLinkStatusConnected MicLinkStatus = "connected"
	// MicLinkStatusClosed means the co-host session ended.
	MicLinkStatusClosed MicLinkStatus = "closed"
)

// MicLink models an intra-room co-host (连麦) session. The host is the room's
// anchor and the guest is the audience member who joined the mic. Audio
// transport itself is external (WebRTC/SFU); this aggregate only owns the
// signaling/state contract.
type MicLink struct {
	ID        string
	RoomID    string
	HostID    string
	GuestID   string
	Status    MicLinkStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  *time.Time
}

// NewMicLink creates a requesting co-host session.
func NewMicLink(id, roomID, hostID, guestID string) *MicLink {
	now := time.Now()
	return &MicLink{
		ID:        id,
		RoomID:    roomID,
		HostID:    hostID,
		GuestID:   guestID,
		Status:    MicLinkStatusRequesting,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Accept moves the session to connected.
func (m *MicLink) Accept() {
	m.Status = MicLinkStatusConnected
	m.UpdatedAt = time.Now()
}

// Close ends the session.
func (m *MicLink) Close() {
	now := time.Now()
	m.Status = MicLinkStatusClosed
	m.ClosedAt = &now
	m.UpdatedAt = now
}

// IsActive reports whether the session is still open for signaling.
func (m *MicLink) IsActive() bool {
	return m.Status == MicLinkStatusRequesting || m.Status == MicLinkStatusConnected
}
