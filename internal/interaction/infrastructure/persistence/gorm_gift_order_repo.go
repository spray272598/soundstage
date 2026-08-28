package persistence

import (
	"context"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"gorm.io/gorm"
)

// GormGiftOrderRepository implements domain.GiftOrderRepository with GORM.
type GormGiftOrderRepository struct {
	db *gorm.DB
}

// NewGormGiftOrderRepository creates a new GormGiftOrderRepository.
func NewGormGiftOrderRepository(db *gorm.DB) *GormGiftOrderRepository {
	return &GormGiftOrderRepository{db: db}
}

// Create inserts a new gift order. If the idempotency key collides (a
// duplicate send under a race), it returns the already-stored order instead
// of failing, so the application stays idempotent end-to-end.
func (r *GormGiftOrderRepository) Create(ctx context.Context, o *domain.GiftOrder) error {
	err := r.db.WithContext(ctx).Create(orderToModel(o)).Error
	if err == nil {
		return nil
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		existing, gerr := r.GetByIDempotencyKey(ctx, o.IdempotencyKey)
		if gerr == nil {
			*o = *existing
			return nil
		}
	}
	return err
}

// Update persists a status change on a gift order.
func (r *GormGiftOrderRepository) Update(ctx context.Context, o *domain.GiftOrder) error {
	return r.db.WithContext(ctx).Save(orderToModel(o)).Error
}

// GetByID returns a gift order by id.
func (r *GormGiftOrderRepository) GetByID(ctx context.Context, id string) (*domain.GiftOrder, error) {
	var m GiftOrderModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return orderToDomain(&m), nil
}

// GetByIDempotencyKey returns a gift order by its idempotency key.
func (r *GormGiftOrderRepository) GetByIDempotencyKey(ctx context.Context, key string) (*domain.GiftOrder, error) {
	var m GiftOrderModel
	if err := r.db.WithContext(ctx).First(&m, "idempotency_key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return orderToDomain(&m), nil
}

// Compile-time check.
var _ domain.GiftOrderRepository = (*GormGiftOrderRepository)(nil)
