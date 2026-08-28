package persistence

import (
	"context"
	"errors"

	"github.com/spray272598/soundstage/internal/room/domain"
	"gorm.io/gorm"
)

// GormRoomRepository implements domain.RoomRepository with GORM.
type GormRoomRepository struct {
	db *gorm.DB
}

// NewGormRoomRepository creates a new GormRoomRepository.
func NewGormRoomRepository(db *gorm.DB) *GormRoomRepository {
	return &GormRoomRepository{db: db}
}

// Create persists a new room.
func (r *GormRoomRepository) Create(ctx context.Context, room *domain.Room) error {
	return r.db.WithContext(ctx).Create(toModel(room)).Error
}

// GetByID retrieves a room by ID.
func (r *GormRoomRepository) GetByID(ctx context.Context, id string) (*domain.Room, error) {
	var m RoomModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRoomNotFound
		}
		return nil, err
	}
	return toDomain(&m), nil
}

// Update persists changes to a room.
func (r *GormRoomRepository) Update(ctx context.Context, room *domain.Room) error {
	return r.db.WithContext(ctx).Save(toModel(room)).Error
}

// List retrieves a paginated list of rooms ordered by creation time desc.
func (r *GormRoomRepository) List(ctx context.Context, limit, offset int) ([]*domain.Room, error) {
	var models []RoomModel
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, err
	}
	rooms := make([]*domain.Room, len(models))
	for i, m := range models {
		rooms[i] = toDomain(&m)
	}
	return rooms, nil
}

func toModel(r *domain.Room) *RoomModel {
	return &RoomModel{
		ID:        r.ID,
		AnchorID:  r.AnchorID,
		Title:     r.Title,
		Status:    string(r.Status),
		CreatedAt: r.CreatedAt,
		StartedAt: r.StartedAt,
		EndedAt:   r.EndedAt,
	}
}

func toDomain(m *RoomModel) *domain.Room {
	return &domain.Room{
		ID:        m.ID,
		AnchorID:  m.AnchorID,
		Title:     m.Title,
		Status:    domain.Status(m.Status),
		CreatedAt: m.CreatedAt,
		StartedAt: m.StartedAt,
		EndedAt:   m.EndedAt,
	}
}
