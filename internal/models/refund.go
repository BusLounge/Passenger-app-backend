package models

import (
	"time"

	"github.com/google/uuid"
)

type Refund struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	BookingID       uuid.UUID  `json:"booking_id" db:"booking_id"`
	LegID           *uuid.UUID `json:"leg_id" db:"leg_id"`
	TransactionID   *uuid.UUID `json:"transaction_id" db:"transaction_id"`
	Amount          float64    `json:"amount" db:"amount"`
	Reason          string     `json:"reason" db:"reason"`
	Status          string     `json:"status" db:"status"` // pending, processing, completed, failed
	RefundReference *string    `json:"refund_reference" db:"refund_reference"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	RefundedAt      *time.Time `json:"refunded_at" db:"refunded_at"`
}
