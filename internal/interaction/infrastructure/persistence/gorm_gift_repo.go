package persistence

import (
	"context"
	"errors"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"gorm.io/gorm"
)

// GormGiftRepository implements domain.GiftRepository with GORM.
type GormGiftRepository struct {
	db *gorm.DB
}

// NewGormGiftRepository creates a new GormGiftRepository.
func NewGormGiftRepository(db *gorm.DB) *GormGiftRepository {
	return &GormGiftRepository{db: db}
}

// ListCatalog returns every gift, active or not.
func (r *GormGiftRepository) ListCatalog(ctx context.Context) ([]*domain.Gift, error) {
	var models []GiftModel
	if err := r.db.WithContext(ctx).Order("price ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	gifts := make([]*domain.Gift, len(models))
	for i, m := range models {
		gifts[i] = giftToDomain(&m)
	}
	return gifts, nil
}

// GetByID returns a gift by id.
func (r *GormGiftRepository) GetByID(ctx context.Context, id string) (*domain.Gift, error) {
	var m GiftModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrGiftNotFound
		}
		return nil, err
	}
	return giftToDomain(&m), nil
}

// Compile-time check.
var _ domain.GiftRepository = (*GormGiftRepository)(nil)
