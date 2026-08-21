package models

import (
	"time"
	"github.com/google/uuid"
)

// Wallet represents a passenger's electronic wallet
type Wallet struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Balance   float64   `json:"balance" db:"balance"`
	Status    string    `json:"status" db:"status"` // 'ACTIVE', 'FROZEN'
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// WalletTransaction represents a transaction (credit or debit) on a wallet
type WalletTransaction struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	WalletID         uuid.UUID  `json:"wallet_id" db:"wallet_id"`
	Amount           float64    `json:"amount" db:"amount"`
	TransactionType  string     `json:"transaction_type" db:"transaction_type"`     // CREDIT, DEBIT
	ReferenceType    *string    `json:"reference_type,omitempty" db:"reference_type"` // e.g., TOPUP, BOOKING
	ReferenceID      *uuid.UUID `json:"reference_id,omitempty" db:"reference_id"`
	GatewayReference *string    `json:"gateway_reference,omitempty" db:"gateway_reference"`
	Description      *string    `json:"description,omitempty" db:"description"`
	Status           string     `json:"status" db:"status"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}
