package application

import (
	"context"

	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/room/domain"
)

// RoomService implements the room use cases.
type RoomService struct {
	repo domain.RoomRepository
}

// NewRoomService creates a new RoomService.
func NewRoomService(repo domain.RoomRepository) *RoomService {
	return &RoomService{repo: repo}
}

// CreateRoomRequest is the input for creating a room.
type CreateRoomRequest struct {
	AnchorID string
	Title    string
}

// Create creates a new pending room.
func (s *RoomService) Create(ctx context.Context, req CreateRoomRequest) (*domain.Room, error) {
	room := domain.NewRoom(id.New(), req.AnchorID, req.Title)
	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// Open opens an existing pending room.
func (s *RoomService) Open(ctx context.Context, roomID string) (*domain.Room, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if err := room.Open(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// Close closes an existing live room.
func (s *RoomService) Close(ctx context.Context, roomID string) (*domain.Room, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if err := room.Close(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// Get retrieves a room by ID.
func (s *RoomService) Get(ctx context.Context, roomID string) (*domain.Room, error) {
	return s.repo.GetByID(ctx, roomID)
}

// List retrieves a paginated list of rooms.
func (s *RoomService) List(ctx context.Context, limit, offset int) ([]*domain.Room, error) {
	return s.repo.List(ctx, limit, offset)
}
