package idgen

import (
	"github.com/google/uuid"
)

// NewUUID generates a time-ordered UUIDv7.
func NewUUID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to random UUID if clock error occurs
		return uuid.New()
	}
	return id
}

// New is an alias for NewUUID.
func New() uuid.UUID {
	return NewUUID()
}
