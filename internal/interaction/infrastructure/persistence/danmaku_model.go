package persistence

import (
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
)

// DanmakuModel is the GORM model for a single danmaku row. Rows are sharded by
// day, so the table name is derived from CreatedAt (see TableName).
type DanmakuModel struct {
	ID        string `gorm:"primaryKey;size:32"`
	RoomID    string `gorm:"size:32;index"`
	UserID    string `gorm:"size:32;index"`
	Content   string `gorm:"type:varchar(1024)"`
	Status    string `gorm:"size:16"`
	CreatedAt time.Time
}

// TableName returns the day-sharded table name, e.g. danmaku_20260828.
func (m *DanmakuModel) TableName() string {
	return danmakuTable(m.CreatedAt)
}

// danmakuTable builds the shard name for the given time.
func danmakuTable(t time.Time) string {
	return "danmaku_" + t.Format("20060102")
}

func danmakuToModel(d *domain.Danmaku) *DanmakuModel {
	return &DanmakuModel{
		ID:        d.ID,
		RoomID:    d.RoomID,
		UserID:    d.UserID,
		Content:   d.Content,
		Status:    string(d.Status),
		CreatedAt: d.CreatedAt,
	}
}
