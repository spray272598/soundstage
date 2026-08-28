package errors

import "errors"

// Common errors used across bounded contexts.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("resource conflict")
	ErrInternal     = errors.New("internal error")
)

// DomainError wraps a domain-level error with context.
type DomainError struct {
	Op  string
	Err error
}

func (e *DomainError) Error() string {
	return e.Op + ": " + e.Err.Error()
}

func (e *DomainError) Unwrap() error {
	return e.Err
}
