package persistence

import (
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
)

// GiftModel is the GORM model for the gifts (catalog) table.
type GiftModel struct {
	ID     string `gorm:"primaryKey;size:32"`
	Name   string `gorm:"size:64"`
	Price  int64
	Icon   string `gorm:"size:255"`
	Status string `gorm:"size:16;index"`
}

// TableName returns the table name for GiftModel.
func (GiftModel) TableName() string { return "gifts" }

// GiftOrderModel is the GORM model for the gift_orders table.
type GiftOrderModel struct {
	ID             string `gorm:"primaryKey;size:32"`
	RoomID         string `gorm:"size:32;index"`
	SenderID       string `gorm:"size:32;index"`
	GiftID         string `gorm:"size:32;index"`
	GiftName       string `gorm:"size:64"`
	Count          int
	UnitPrice      int64
	TotalAmount    int64
	Status         string `gorm:"size:16;index"`
	IdempotencyKey string `gorm:"size:64;uniqueIndex"`
	CreatedAt      time.Time
	SettledAt      *time.Time
}

// TableName returns the table name for GiftOrderModel.
func (GiftOrderModel) TableName() string { return "gift_orders" }

func giftToModel(g *domain.Gift) *GiftModel {
	return &GiftModel{
		ID:     g.ID,
		Name:   g.Name,
		Price:  g.Price,
		Icon:   g.Icon,
		Status: string(g.Status),
	}
}

func giftToDomain(m *GiftModel) *domain.Gift {
	return &domain.Gift{
		ID:     m.ID,
		Name:   m.Name,
		Price:  m.Price,
		Icon:   m.Icon,
		Status: domain.GiftStatus(m.Status),
	}
}

func orderToModel(o *domain.GiftOrder) *GiftOrderModel {
	return &GiftOrderModel{
		ID:             o.ID,
		RoomID:         o.RoomID,
		SenderID:       o.SenderID,
		GiftID:         o.GiftID,
		GiftName:       o.GiftName,
		Count:          o.Count,
		UnitPrice:      o.UnitPrice,
		TotalAmount:    o.TotalAmount,
		Status:         string(o.Status),
		IdempotencyKey: o.IdempotencyKey,
		CreatedAt:      o.CreatedAt,
		SettledAt:      o.SettledAt,
	}
}

func orderToDomain(m *GiftOrderModel) *domain.GiftOrder {
	return &domain.GiftOrder{
		ID:             m.ID,
		RoomID:         m.RoomID,
		SenderID:       m.SenderID,
		GiftID:         m.GiftID,
		GiftName:       m.GiftName,
		Count:          m.Count,
		UnitPrice:      m.UnitPrice,
		TotalAmount:    m.TotalAmount,
		Status:         domain.GiftOrderStatus(m.Status),
		IdempotencyKey: m.IdempotencyKey,
		CreatedAt:      m.CreatedAt,
		SettledAt:      m.SettledAt,
	}
}
