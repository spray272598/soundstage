package persistence

import (
	"time"

	"github.com/spray272598/soundstage/internal/miclink/domain"
)

// PKSessionModel is the GORM representation of a cross-room PK battle.
type PKSessionModel struct {
	ID         string    `gorm:"primaryKey;size:32"`
	RoomAID    string    `gorm:"index:idx_pk_room_a;size:32"`
	RoomBID    string    `gorm:"index:idx_pk_room_b;size:32"`
	AnchorAID  string    `gorm:"size:32"`
	AnchorBID  string    `gorm:"size:32"`
	Status     string    `gorm:"index:idx_pk_status;size:16"`
	ScoreA     int64
	ScoreB     int64
	StartedAt  *time.Time
	EndsAt     *time.Time
	Winner     string    `gorm:"size:8"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}

// TableName overrides the default pluralization.
func (PKSessionModel) TableName() string { return "pk_sessions" }

func pkToModel(p *domain.PKSession) *PKSessionModel {
	return &PKSessionModel{
		ID:         p.ID,
		RoomAID:    p.RoomAID,
		RoomBID:    p.RoomBID,
		AnchorAID:  p.AnchorAID,
		AnchorBID:  p.AnchorBID,
		Status:     string(p.Status),
		ScoreA:     p.ScoreA,
		ScoreB:     p.ScoreB,
		StartedAt:  p.StartedAt,
		EndsAt:     p.EndsAt,
		Winner:     string(p.Winner),
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
		FinishedAt: p.FinishedAt,
	}
}

func pkToDomain(m *PKSessionModel) *domain.PKSession {
	return &domain.PKSession{
		ID:         m.ID,
		RoomAID:    m.RoomAID,
		RoomBID:    m.RoomBID,
		AnchorAID:  m.AnchorAID,
		AnchorBID:  m.AnchorBID,
		Status:     domain.PKStatus(m.Status),
		ScoreA:     m.ScoreA,
		ScoreB:     m.ScoreB,
		StartedAt:  m.StartedAt,
		EndsAt:     m.EndsAt,
		Winner:     domain.PKWinner(m.Winner),
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		FinishedAt: m.FinishedAt,
	}
}
