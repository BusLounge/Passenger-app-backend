package models

import (
	"time"

	"github.com/google/uuid"
)

// PaymentAttemptStatus defines the result of a specific payment try
type PaymentAttemptStatus string

const (
	AttemptStatusPending    PaymentAttemptStatus = "pending"
	AttemptStatusProcessing PaymentAttemptStatus = "processing"
	AttemptStatusSuccess    PaymentAttemptStatus = "success"
	AttemptStatusFailed     PaymentAttemptStatus = "failed"
	AttemptStatusRefunded   PaymentAttemptStatus = "refunded"
)

// PaymentAttempt records a single attempt to pay for a booking intent
type PaymentAttempt struct {
	ID               uuid.UUID            `json:"id" db:"id"`
	BookingIntentID  uuid.UUID            `json:"booking_intent_id" db:"booking_intent_id"`
	PaymentGateway   string               `json:"payment_gateway" db:"payment_gateway"`
	
	PaymentReference *string              `json:"payment_reference,omitempty" db:"payment_reference"`
	PaymentUID       *string              `json:"payment_uid,omitempty" db:"payment_uid"`
	
	Status           PaymentAttemptStatus `json:"status" db:"status"`
	StatusIndicator  *string              `json:"status_indicator,omitempty" db:"status_indicator"`
	
	InitiatedAt      time.Time            `json:"initiated_at" db:"initiated_at"`
	ResolvedAt       *time.Time           `json:"resolved_at,omitempty" db:"resolved_at"`
}
