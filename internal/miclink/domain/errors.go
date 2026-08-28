package domain

import "errors"

// Domain errors for the miclink bounded context.
var (
	// ErrMicLinkNotFound is returned when a co-host session id is unknown.
	ErrMicLinkNotFound = errors.New("mic-link session not found")
	// ErrMicLinkConflict is returned when a room already has an active co-host.
	ErrMicLinkConflict = errors.New("room already has an active mic-link")
	// ErrPKNotFound is returned when a PK session id is unknown.
	ErrPKNotFound = errors.New("pk session not found")
	// ErrPKConflict is returned when a room is already in an active PK.
	ErrPKConflict = errors.New("room already in a pk battle")
	// ErrInvalidTransition is returned on an illegal PK state-machine move.
	ErrInvalidTransition = errors.New("invalid pk state transition")
	// ErrInvalidSide is returned for an unknown PK side.
	ErrInvalidSide = errors.New("invalid pk side")
	// ErrNotPKParticipant is returned when a room is not part of a PK.
	ErrNotPKParticipant = errors.New("room is not a participant of this pk")
)
