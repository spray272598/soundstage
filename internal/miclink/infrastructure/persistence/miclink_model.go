package persistence

import (
	"time"

	"github.com/spray272598/soundstage/internal/miclink/domain"
)

// MicLinkModel is the GORM representation of a co-host session.
type MicLinkModel struct {
	ID        string     `gorm:"primaryKey;size:32"`
	RoomID    string     `gorm:"index:idx_miclink_room;size:32"`
	HostID    string     `gorm:"size:32"`
	GuestID   string     `gorm:"size:32"`
	Status    string     `gorm:"index:idx_miclink_room;size:16"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  *time.Time
}

// TableName overrides the default pluralization.
func (MicLinkModel) TableName() string { return "mic_links" }

func micLinkToModel(m *domain.MicLink) *MicLinkModel {
	return &MicLinkModel{
		ID:        m.ID,
		RoomID:    m.RoomID,
		HostID:    m.HostID,
		GuestID:   m.GuestID,
		Status:    string(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		ClosedAt:  m.ClosedAt,
	}
}

func micLinkToDomain(m *MicLinkModel) *domain.MicLink {
	return &domain.MicLink{
		ID:        m.ID,
		RoomID:    m.RoomID,
		HostID:    m.HostID,
		GuestID:   m.GuestID,
		Status:    domain.MicLinkStatus(m.Status),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		ClosedAt:  m.ClosedAt,
	}
}
