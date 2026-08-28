package domain

import "errors"

// Interaction-domain errors.
var (
	// ErrGiftNotFound is returned when a gift id is unknown.
	ErrGiftNotFound = errors.New("gift not found")
	// ErrGiftInactive is returned when a gift is taken off the shelf.
	ErrGiftInactive = errors.New("gift is not active")
	// ErrRateLimited is returned when a user exceeds the danmaku rate limit.
	ErrRateLimited = errors.New("rate limited")
	// ErrRejected is returned when a message is blocked by the moderator.
	ErrRejected = errors.New("message rejected by moderator")
	// ErrOrderNotFound is returned when a gift order id is unknown.
	ErrOrderNotFound = errors.New("gift order not found")
	// ErrInvalidPeriod is returned when an unknown rank period is requested.
	ErrInvalidPeriod = errors.New("invalid rank period")
)
