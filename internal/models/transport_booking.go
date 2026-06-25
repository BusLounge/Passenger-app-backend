package models

import (
	"time"
)

// TransportBookingStatus represents the status of a transport booking
type TransportBookingStatus string

const (
	TransportBookingPending   TransportBookingStatus = "pending"
	TransportBookingConfirmed TransportBookingStatus = "confirmed"
	TransportBookingInProgress TransportBookingStatus = "in_progress"
	TransportBookingCompleted TransportBookingStatus = "completed"
	TransportBookingCancelled TransportBookingStatus = "cancelled"
)

// TransportPaymentStatus represents the payment status of a transport booking
type TransportPaymentStatus string

const (
	TransportPaymentPending       TransportPaymentStatus = "pending"
	TransportPaymentPaid          TransportPaymentStatus = "paid"
	TransportPaymentFailed        TransportPaymentStatus = "failed"
	TransportPaymentRefunded      TransportPaymentStatus = "refunded"
	TransportPaymentPartialRefund TransportPaymentStatus = "partial_refund"
)

// TransportBooking represents an independent transport booking
type TransportBooking struct {
	ID                       string                 `json:"id" db:"id"`
	BookingID                *string                `json:"booking_id,omitempty" db:"booking_id"` // Link to master booking (bus/combined)
	UserID                   string                 `json:"user_id" db:"user_id"`
	LoungeID                 *string                `json:"lounge_id,omitempty" db:"lounge_id"` // Link to lounge if applicable
	PickupLocationID         *string                `json:"pickup_location_id,omitempty" db:"pickup_location_id"`
	VehicleType              string                 `json:"vehicle_type" db:"vehicle_type"`
	VehicleQuantity          int                    `json:"vehicle_quantity" db:"vehicle_quantity"`
	TransportPrice           float64                `json:"transport_price" db:"transport_price"`
	TransportDate            time.Time              `json:"transport_date" db:"transport_date"`
	TransportTime            time.Time              `json:"transport_time" db:"transport_time"`
	EstimatedDurationMinutes *int                   `json:"estimated_duration_minutes,omitempty" db:"estimated_duration_minutes"`
	Status                   TransportBookingStatus `json:"status" db:"status"`
	PaymentStatus            TransportPaymentStatus `json:"payment_status" db:"payment_status"`
	LoungeTransportType      *string                `json:"lounge_transport_type,omitempty" db:"lounge_transport_type"`
	CancellationReason       *string                `json:"cancellation_reason,omitempty" db:"cancellation_reason"`
	RefundStatus             *string                `json:"refund_status,omitempty" db:"refund_status"`
	RefundAmount             float64                `json:"refund_amount" db:"refund_amount"`
	CreatedAt                time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time              `json:"updated_at" db:"updated_at"`

	// Joined/Denormalized Fields (not saved in this table)
	PickupLocationName *string `json:"pickup_location_name,omitempty" db:"pickup_location_name"`
	LoungeName         *string `json:"lounge_name,omitempty" db:"lounge_name"`
}
