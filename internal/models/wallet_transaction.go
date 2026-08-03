package models

import (
	"time"

	"github.com/google/uuid"
)

// WalletTransaction tracks every deposit, withdrawal, and payment
type WalletTransaction struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	WalletID         uuid.UUID  `json:"wallet_id" db:"wallet_id"`
	Amount           float64    `json:"amount" db:"amount"`
	TransactionType  string     `json:"transaction_type" db:"transaction_type"` // 'CREDIT' or 'DEBIT'
	ReferenceType    string     `json:"reference_type" db:"reference_type"`     // 'TOPUP', 'BOOKING', 'CASHBACK', 'WITHDRAWAL'
	ReferenceID      NullUUID   `json:"reference_id,omitempty" db:"reference_id"` // E.g., Booking ID
	GatewayReference NullString `json:"gateway_reference,omitempty" db:"gateway_reference"` // PayHere Payment ID
	Description      NullString `json:"description,omitempty" db:"description"`
	Status           string     `json:"status" db:"status"` // 'COMPLETED', 'FAILED', 'PENDING'
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}
