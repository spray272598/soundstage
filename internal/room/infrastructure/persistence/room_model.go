package persistence

import "time"

// RoomModel is the GORM model for the rooms table.
type RoomModel struct {
	ID        string `gorm:"primaryKey;size:32"`
	AnchorID  string `gorm:"size:32;index"`
	Title     string `gorm:"size:255"`
	Status    string `gorm:"size:16;index"`
	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time
}

// TableName returns the table name for RoomModel.
func (RoomModel) TableName() string {
	return "rooms"
}
