package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// RoomStatsModel is the cold-storage snapshot for per-room counters
// (likes, cumulative gift value). It is written by the periodic flush job.
type RoomStatsModel struct {
	RoomID     string `gorm:"primaryKey;size:32"`
	Likes      int64
	GiftsTotal int64
	UpdatedAt  time.Time
}

// TableName returns the table name for RoomStatsModel.
func (RoomStatsModel) TableName() string { return "room_stats" }

// GormRoomStatsRepository maintains the room_stats snapshot.
type GormRoomStatsRepository struct {
	db *gorm.DB
}

// NewGormRoomStatsRepository creates a new GormRoomStatsRepository.
func NewGormRoomStatsRepository(db *gorm.DB) *GormRoomStatsRepository {
	return &GormRoomStatsRepository{db: db}
}

// UpsertLikes overwrites the like tally for a room, bumping UpdatedAt.
func (r *GormRoomStatsRepository) UpsertLikes(ctx context.Context, roomID string, likes int64) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Session(&gorm.Session{}).
		Where("room_id = ?", roomID).
		Assign(RoomStatsModel{Likes: likes, UpdatedAt: now}).
		FirstOrCreate(&RoomStatsModel{RoomID: roomID}).Error
}

// Compile-time check.
var _ domainLikeStatsRepo = (*GormRoomStatsRepository)(nil)

// domainLikeStatsRepo is the minimal port used by the like-flush task.
type domainLikeStatsRepo interface {
	UpsertLikes(ctx context.Context, roomID string, likes int64) error
}
