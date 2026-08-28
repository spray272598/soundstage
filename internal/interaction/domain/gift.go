package domain

import "time"

// GiftStatus represents the on/off-shelf state of a catalog gift.
type GiftStatus string

const (
	// GiftStatusActive means the gift can be sent.
	GiftStatusActive GiftStatus = "active"
	// GiftStatusInactive means the gift is taken down and cannot be sent.
	GiftStatusInactive GiftStatus = "inactive"
)

// Gift is a platform-wide catalog item. Gifts are configured centrally by
// operations; individual rooms only decide whether gifting is enabled.
type Gift struct {
	ID     string
	Name   string
	Price  int64 // unit price in cents
	Icon   string
	Status GiftStatus
}

// GiftOrderStatus represents the settlement lifecycle of a gift order.
type GiftOrderStatus string

const (
	// GiftOrderStatusCreated is the initial state right after the gift is sent.
	GiftOrderStatusCreated GiftOrderStatus = "created"
	// GiftOrderStatusSettled means the order is reconciled and the rank updated.
	GiftOrderStatusSettled GiftOrderStatus = "settled"
	// GiftOrderStatusFailed means settlement failed after retries.
	GiftOrderStatusFailed GiftOrderStatus = "failed"
)

// GiftOrder is an immutable-ish record of a single gifting action. Using an
// order (instead of a plain record) gives us a lifecycle, idempotency and
// reconciliation — the same gift send must never be settled twice.
type GiftOrder struct {
	ID             string
	RoomID         string
	SenderID       string
	GiftID         string
	GiftName       string
	Count          int
	UnitPrice      int64
	TotalAmount    int64
	Status         GiftOrderStatus
	IdempotencyKey string
	CreatedAt      time.Time
	SettledAt      *time.Time
}

// NewGiftOrder builds a created gift order and computes the total amount.
func NewGiftOrder(id, roomID, senderID, giftID, giftName string, count int, unitPrice int64, idempotencyKey string) *GiftOrder {
	return &GiftOrder{
		ID:             id,
		RoomID:         roomID,
		SenderID:       senderID,
		GiftID:         giftID,
		GiftName:       giftName,
		Count:          count,
		UnitPrice:      unitPrice,
		TotalAmount:    int64(count) * unitPrice,
		Status:         GiftOrderStatusCreated,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
	}
}

// MarkSettled transitions the order to settled and stamps the settlement time.
func (o *GiftOrder) MarkSettled() {
	now := time.Now().UTC()
	o.Status = GiftOrderStatusSettled
	o.SettledAt = &now
}

// MarkFailed transitions the order to failed.
func (o *GiftOrder) MarkFailed() {
	o.Status = GiftOrderStatusFailed
}
