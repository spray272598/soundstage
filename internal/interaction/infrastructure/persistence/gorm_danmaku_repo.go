package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"gorm.io/gorm"
)

// GormDanmakuRepository implements domain.DanmakuRepository with day-sharded tables.
type GormDanmakuRepository struct {
	db *gorm.DB
}

// NewGormDanmakuRepository creates a new GormDanmakuRepository.
func NewGormDanmakuRepository(db *gorm.DB) *GormDanmakuRepository {
	return &GormDanmakuRepository{db: db}
}

// Create persists a danmaku into its day shard, creating the shard if needed.
func (r *GormDanmakuRepository) Create(ctx context.Context, d *domain.Danmaku) error {
	model := danmakuToModel(d)
	if err := r.ensureTable(ctx, model.CreatedAt); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(model).Error
}

// ensureTable creates the day shard if it does not yet exist. Shards are
// created lazily on first write, which keeps the migration step trivial.
func (r *GormDanmakuRepository) ensureTable(ctx context.Context, t time.Time) error {
	table := danmakuTable(t)
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id VARCHAR(32) PRIMARY KEY,
		room_id VARCHAR(32) NOT NULL,
		user_id VARCHAR(32) NOT NULL,
		content VARCHAR(1024) NOT NULL,
		status VARCHAR(16) NOT NULL,
		created_at DATETIME(3) NOT NULL,
		KEY idx_room (room_id),
		KEY idx_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, table)
	return r.db.WithContext(ctx).Exec(stmt).Error
}

// Compile-time check.
var _ domain.DanmakuRepository = (*GormDanmakuRepository)(nil)
