package domain

import "context"

// GiftRepository is the outbound port for the gift catalog.
type GiftRepository interface {
	ListCatalog(ctx context.Context) ([]*Gift, error)
	GetByID(ctx context.Context, id string) (*Gift, error)
}

// GiftOrderRepository is the outbound port for gift orders. Idempotency is
// enforced through IdempotencyKey: GetByIDempotencyKey lets the application
// short-circuit a duplicate send.
type GiftOrderRepository interface {
	Create(ctx context.Context, o *GiftOrder) error
	Update(ctx context.Context, o *GiftOrder) error
	GetByID(ctx context.Context, id string) (*GiftOrder, error)
	GetByIDempotencyKey(ctx context.Context, key string) (*GiftOrder, error)
}

// DanmakuRepository is the outbound port for danmaku persistence. Rows are
// sharded by day, so the implementation owns table routing.
type DanmakuRepository interface {
	Create(ctx context.Context, d *Danmaku) error
}
