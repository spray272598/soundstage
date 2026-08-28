package domain

import "errors"

// ErrRoomNotFound is returned when a requested room does not exist.
var ErrRoomNotFound = errors.New("room not found")
