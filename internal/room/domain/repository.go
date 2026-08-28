package domain

import "context"

// RoomRepository is the outbound port for room persistence.
type RoomRepository interface {
	Create(ctx context.Context, room *Room) error
	GetByID(ctx context.Context, id string) (*Room, error)
	Update(ctx context.Context, room *Room) error
	List(ctx context.Context, limit, offset int) ([]*Room, error)
}
