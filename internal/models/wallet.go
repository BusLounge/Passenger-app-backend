package models

import (
	"time"
	"github.com/google/uuid"
)

// Wallet represents a user's digital E-Wallet balance
type Wallet struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Balance   float64   `json:"balance" db:"balance"`
	Status    string    `json:"status" db:"status"` // 'ACTIVE', 'FROZEN'
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
