package models

import (
	"time"

	"github.com/google/uuid"
)

// Customer represents a customer in the system
type Customer struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	Active       bool      `json:"active" db:"active"`
	PasswordHash string    `json:"-" db:"password_hash"` // Never include in JSON
	Salt         string    `json:"-" db:"salt"`          // Never include in JSON

	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// IsActive returns whether the customer is active
func (c *Customer) IsActive() bool {
	return c.Active && c.DeletedAt == nil
}

// GetID returns the customer ID as a string
func (c *Customer) GetID() string {
	return c.ID.String()
}

// GetUsername returns the customer username
func (c *Customer) GetUsername() string {
	return c.Username
}
