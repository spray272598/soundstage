package application

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// MicLinkService owns the intra-room co-host (连麦) use cases. Audio transport
// is external (WebRTC/SFU); this service only manages the signaling/state
// contract and relays WebRTC offer/answer/ice between the host and guest.
type MicLinkService struct {
	micRepo     domain.MicLinkRepository
	relay       domain.SignalingRelay
	broadcaster domain.Broadcaster
}

// NewMicLinkService constructs a MicLinkService from its ports.
func NewMicLinkService(
	micRepo domain.MicLinkRepository,
	relay domain.SignalingRelay,
	broadcaster domain.Broadcaster,
) *MicLinkService {
	return &MicLinkService{
		micRepo:     micRepo,
		relay:       relay,
		broadcaster: broadcaster,
	}
}

// GetState returns the active co-host session for a room, or nil when none is
// live. Used by read-only queries (e.g. the AI room moderator status tool).
func (s *MicLinkService) GetState(ctx context.Context, roomID string) (*domain.MicLink, error) {
	link, err := s.micRepo.GetActiveByRoom(ctx, roomID)
	if err == domain.ErrMicLinkNotFound {
		return nil, nil
	}
	return link, err
}

// Request opens a requesting co-host session and notifies the host. The guest
// (audience member) initiates; the host (room anchor) receives the request.
func (s *MicLinkService) Request(ctx context.Context, roomID, hostID, guestID string) (*domain.MicLink, error) {
	if _, err := s.micRepo.GetActiveByRoom(ctx, roomID); err == nil {
		return nil, domain.ErrMicLinkConflict
	} else if err != domain.ErrMicLinkNotFound {
		return nil, err
	}

	link := domain.NewMicLink(id.New(), roomID, hostID, guestID)
	if err := s.micRepo.Create(ctx, link); err != nil {
		return nil, err
	}
	metrics.MicLinkRequestsTotal.WithLabelValues("requested").Inc()

	payload, _ := json.Marshal(micLinkBroadcast{
		SessionID: link.ID,
		RoomID:    link.RoomID,
		HostID:    link.HostID,
		GuestID:   link.GuestID,
		Status:    string(link.Status),
	})
	// Broadcast to the room; clients filter by host_id.
	if err := s.broadcaster.Broadcast(ctx, roomID, "miclink_request", payload); err != nil {
		logger.L().Warn("broadcast miclink_request failed", zap.Error(err))
	}
	return link, nil
}

// Accept moves a requesting session to connected and tells the room.
func (s *MicLinkService) Accept(ctx context.Context, roomID, guestID, hostID string) (*domain.MicLink, error) {
	link, err := s.micRepo.GetActiveByRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if link.GuestID != guestID || link.HostID != hostID {
		return nil, domain.ErrMicLinkNotFound
	}
	if link.Status != domain.MicLinkStatusRequesting {
		return link, nil
	}
	link.Accept()
	if err := s.micRepo.Update(ctx, link); err != nil {
		return nil, err
	}
	metrics.MicLinkRequestsTotal.WithLabelValues("accepted").Inc()

	payload, _ := json.Marshal(micLinkBroadcast{
		SessionID: link.ID,
		RoomID:    link.RoomID,
		HostID:    link.HostID,
		GuestID:   link.GuestID,
		Status:    string(link.Status),
	})
	if err := s.broadcaster.Broadcast(ctx, roomID, "miclink_connected", payload); err != nil {
		logger.L().Warn("broadcast miclink_connected failed", zap.Error(err))
	}
	return link, nil
}

// Close ends the co-host session.
func (s *MicLinkService) Close(ctx context.Context, roomID string) error {
	link, err := s.micRepo.GetActiveByRoom(ctx, roomID)
	if err != nil {
		return err
	}
	link.Close()
	if err := s.micRepo.Update(ctx, link); err != nil {
		return err
	}
	payload, _ := json.Marshal(micLinkBroadcast{
		SessionID: link.ID,
		RoomID:    link.RoomID,
		HostID:    link.HostID,
		GuestID:   link.GuestID,
		Status:    string(link.Status),
	})
	if err := s.broadcaster.Broadcast(ctx, roomID, "miclink_closed", payload); err != nil {
		logger.L().Warn("broadcast miclink_closed failed", zap.Error(err))
	}
	return nil
}

// RelaySignal forwards a WebRTC signal to the target user. The session must be
// active; this keeps stray signaling from reaching unrelated users.
func (s *MicLinkService) RelaySignal(ctx context.Context, roomID, fromUserID, toUserID, signalType string, data json.RawMessage) error {
	if _, err := s.micRepo.GetActiveByRoom(ctx, roomID); err != nil {
		return err
	}
	if err := s.relay.Relay(ctx, roomID, toUserID, fromUserID, signalType, data); err != nil {
		return err
	}
	metrics.SignalingRelayedTotal.Inc()
	return nil
}

// micLinkBroadcast is the client-facing payload for co-host state changes.
type micLinkBroadcast struct {
	SessionID string `json:"session_id"`
	RoomID    string `json:"room_id"`
	HostID    string `json:"host_id"`
	GuestID   string `json:"guest_id"`
	Status    string `json:"status"`
}
