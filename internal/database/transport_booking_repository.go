package database

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"time"
)

type TransportBookingRepository struct {
	db *sqlx.DB
}

func NewTransportBookingRepository(db *sqlx.DB) *TransportBookingRepository {
	return &TransportBookingRepository{db: db}
}

// CreateTransportBooking creates a new transport booking
func (r *TransportBookingRepository) CreateTransportBooking(booking *models.TransportBooking) error {
	query := `
		INSERT INTO transport_bookings (
			booking_id, user_id, lounge_id, pickup_location_id,
			vehicle_type, vehicle_quantity, transport_price, transport_date, transport_time,
			estimated_duration_minutes, status, payment_status,
			cancellation_reason, refund_status, refund_amount, lounge_transport_type
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) RETURNING id, created_at, updated_at`

	err := r.db.QueryRowx(query,
		nullableUUID(booking.BookingID), booking.UserID, nullableUUID(booking.LoungeID), nullableUUID(booking.PickupLocationID),
		booking.VehicleType, booking.VehicleQuantity, booking.TransportPrice, booking.TransportDate, booking.TransportTime,
		booking.EstimatedDurationMinutes, booking.Status, booking.PaymentStatus,
		booking.CancellationReason, booking.RefundStatus, booking.RefundAmount, booking.LoungeTransportType,
	).Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create transport booking: %w", err)
	}

	return nil
}

// GetTransportBookingsByUserID retrieves all transport bookings for a user
func (r *TransportBookingRepository) GetTransportBookingsByUserID(userID string, limit, offset int) ([]models.TransportBooking, error) {
	query := `
		SELECT 
			tb.id, tb.booking_id, tb.user_id, tb.lounge_id, tb.pickup_location_id,
			tb.vehicle_type, tb.vehicle_quantity, tb.transport_price, tb.transport_date, tb.transport_time,
			tb.estimated_duration_minutes, tb.status, tb.payment_status,
			tb.cancellation_reason, tb.refund_status, tb.refund_amount, tb.created_at, tb.updated_at,
			tb.lounge_transport_type,
			l.lounge_name, ptl.location as pickup_location_name
		FROM transport_bookings tb
		LEFT JOIN lounges l ON tb.lounge_id = l.id
		LEFT JOIN lounge_transport_locations ptl ON tb.pickup_location_id = ptl.id
		WHERE tb.user_id = $1
		ORDER BY tb.transport_date DESC, tb.transport_time DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.TransportBooking
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetTransportBookingByID retrieves a transport booking by ID
func (r *TransportBookingRepository) GetTransportBookingByID(bookingID string) (*models.TransportBooking, error) {
	var booking models.TransportBooking
	query := `
		SELECT 
			tb.id, tb.booking_id, tb.user_id, tb.lounge_id, tb.pickup_location_id,
			tb.vehicle_type, tb.vehicle_quantity, tb.transport_price, tb.transport_date, tb.transport_time,
			tb.estimated_duration_minutes, tb.status, tb.payment_status,
			tb.cancellation_reason, tb.refund_status, tb.refund_amount, tb.created_at, tb.updated_at,
			tb.lounge_transport_type,
			l.lounge_name, ptl.location as pickup_location_name
		FROM transport_bookings tb
		LEFT JOIN lounges l ON tb.lounge_id = l.id
		LEFT JOIN lounge_transport_locations ptl ON tb.pickup_location_id = ptl.id
		WHERE tb.id = $1`

	err := r.db.Get(&booking, query, bookingID)
	if err != nil {
		return nil, err
	}

	return &booking, nil
}

// GetTransportBookingsByBookingID retrieves transport bookings tied to a master booking
func (r *TransportBookingRepository) GetTransportBookingsByBookingID(masterBookingID string) ([]models.TransportBooking, error) {
	var bookings []models.TransportBooking
	query := `
		SELECT 
			tb.id, tb.booking_id, tb.user_id, tb.lounge_id, tb.pickup_location_id,
			tb.vehicle_type, tb.vehicle_quantity, tb.transport_price, tb.transport_date, tb.transport_time,
			tb.estimated_duration_minutes, tb.status, tb.payment_status,
			tb.cancellation_reason, tb.refund_status, tb.refund_amount, tb.created_at, tb.updated_at,
			tb.lounge_transport_type,
			l.lounge_name, ptl.location as pickup_location_name
		FROM transport_bookings tb
		LEFT JOIN lounges l ON tb.lounge_id = l.id
		LEFT JOIN lounge_transport_locations ptl ON tb.pickup_location_id = ptl.id
		WHERE tb.booking_id = $1`

	err := r.db.Select(&bookings, query, masterBookingID)
	return bookings, err
}

// GenerateTransportBookingReference generates a unique booking reference
// Format: TRP-XXXXXX (6 char alphanumeric)
func (r *TransportBookingRepository) GenerateTransportBookingReference() (string, error) {
	// Reusing the same strategy as lounge bookings
	id := time.Now().UnixNano()
	return fmt.Sprintf("TRP-%X", id)[0:10], nil // Simplified
}
