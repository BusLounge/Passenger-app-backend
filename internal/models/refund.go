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

type CancelledBookingWithRefund struct {
	BookingID          uuid.UUID  `json:"booking_id" db:"id"`
	BookingReference   string     `json:"booking_reference" db:"booking_reference"`
	BookingType        string     `json:"booking_type" db:"booking_type"`
	TotalAmount        float64    `json:"total_amount" db:"total_amount"`
	BookingStatus      string     `json:"booking_status" db:"booking_status"`
	CancelledAt        *time.Time `json:"cancelled_at" db:"cancelled_at"`
	CancellationReason *string    `json:"cancellation_reason" db:"cancellation_reason"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	RouteName          string     `json:"route_name" db:"route_name"`
	DepartureDatetime  *time.Time `json:"departure_datetime" db:"departure_datetime"`
	RefundID           *uuid.UUID `json:"refund_id,omitempty" db:"refund_id"`
	RefundAmount       float64    `json:"refund_amount" db:"refund_amount"`
	RefundStatus       string     `json:"refund_status" db:"refund_status"`
	RefundReference    *string    `json:"refund_reference,omitempty" db:"refund_reference"`
	RefundReason       *string    `json:"refund_reason,omitempty" db:"refund_reason"`
	RefundCreatedAt    *time.Time `json:"refund_created_at,omitempty" db:"refund_created_at"`
	RefundedAt         *time.Time `json:"refunded_at,omitempty" db:"refunded_at"`
}

type CancelledBookingsSummary struct {
	TotalCancelled     int     `json:"total_cancelled"`
	TotalRefunded      float64 `json:"total_refunded"`
	TotalPendingRefund float64 `json:"total_pending_refund"`
}
