package persistence

import (
	"context"
	"errors"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"gorm.io/gorm"
)

// GormPKSessionRepository implements domain.PKSessionRepository with GORM.
type GormPKSessionRepository struct {
	db *gorm.DB
}

// NewGormPKSessionRepository creates a new GormPKSessionRepository.
func NewGormPKSessionRepository(db *gorm.DB) *GormPKSessionRepository {
	return &GormPKSessionRepository{db: db}
}

// Create inserts a new PK session.
func (r *GormPKSessionRepository) Create(ctx context.Context, p *domain.PKSession) error {
	return r.db.WithContext(ctx).Create(pkToModel(p)).Error
}

// GetByID returns a PK session by id.
func (r *GormPKSessionRepository) GetByID(ctx context.Context, id string) (*domain.PKSession, error) {
	var m PKSessionModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrPKNotFound
		}
		return nil, err
	}
	return pkToDomain(&m), nil
}

// GetActiveByRoom returns the ongoing PK involving a room, or ErrPKNotFound.
func (r *GormPKSessionRepository) GetActiveByRoom(ctx context.Context, roomID string) (*domain.PKSession, error) {
	var m PKSessionModel
	err := r.db.WithContext(ctx).
		Where("(room_a_id = ? OR room_b_id = ?) AND status = ?", roomID, roomID, string(domain.PKStatusOngoing)).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrPKNotFound
		}
		return nil, err
	}
	return pkToDomain(&m), nil
}

// Update persists changes to an existing PK session.
func (r *GormPKSessionRepository) Update(ctx context.Context, p *domain.PKSession) error {
	return r.db.WithContext(ctx).Save(pkToModel(p)).Error
}

// Compile-time check.
var _ domain.PKSessionRepository = (*GormPKSessionRepository)(nil)
