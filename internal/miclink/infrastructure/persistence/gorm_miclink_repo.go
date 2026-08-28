package persistence

import (
	"context"
	"errors"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"gorm.io/gorm"
)

// GormMicLinkRepository implements domain.MicLinkRepository with GORM.
type GormMicLinkRepository struct {
	db *gorm.DB
}

// NewGormMicLinkRepository creates a new GormMicLinkRepository.
func NewGormMicLinkRepository(db *gorm.DB) *GormMicLinkRepository {
	return &GormMicLinkRepository{db: db}
}

// Create inserts a new co-host session.
func (r *GormMicLinkRepository) Create(ctx context.Context, m *domain.MicLink) error {
	return r.db.WithContext(ctx).Create(micLinkToModel(m)).Error
}

// GetByID returns a session by id.
func (r *GormMicLinkRepository) GetByID(ctx context.Context, id string) (*domain.MicLink, error) {
	var m MicLinkModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrMicLinkNotFound
		}
		return nil, err
	}
	return micLinkToDomain(&m), nil
}

// GetActiveByRoom returns the open session for a room, or ErrMicLinkNotFound.
func (r *GormMicLinkRepository) GetActiveByRoom(ctx context.Context, roomID string) (*domain.MicLink, error) {
	var m MicLinkModel
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND status IN ?", roomID, []string{
			string(domain.MicLinkStatusRequesting),
			string(domain.MicLinkStatusConnected),
		}).
		Order("created_at DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrMicLinkNotFound
		}
		return nil, err
	}
	return micLinkToDomain(&m), nil
}

// Update persists changes to an existing session.
func (r *GormMicLinkRepository) Update(ctx context.Context, m *domain.MicLink) error {
	return r.db.WithContext(ctx).Save(micLinkToModel(m)).Error
}

// Compile-time check.
var _ domain.MicLinkRepository = (*GormMicLinkRepository)(nil)
