package database

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// nullableUUID converts a *string UUID pointer to nil if the value is empty.
// This prevents PostgreSQL from receiving an empty string "" for a UUID column,
// which would cause: invalid input syntax for type uuid: ""
func nullableUUID(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// nullableUUIDStr converts a plain string UUID to nil pointer if the value is empty.
func nullableUUIDStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// AppBookingRepository handles booking database operations
type AppBookingRepository struct {
	db *sqlx.DB
}

// NewAppBookingRepository creates a new AppBookingRepository
func NewAppBookingRepository(db *sqlx.DB) *AppBookingRepository {
	return &AppBookingRepository{db: db}
}

// ============================================================================
// REFERENCE/QR GENERATION FUNCTIONS
// ============================================================================

// GenerateBookingReference generates a unique booking reference
// Format: BL-YYYYMMDD-XXXXXX (6 char alphanumeric)
// Example: BL-20251206-A1B2C3
func (r *AppBookingRepository) GenerateBookingReference() (string, error) {
	todayStr := time.Now().Format("20060102")

	for attempts := 0; attempts < 10; attempts++ {
		// Generate 6 random bytes and take first 6 hex chars
		randomBytes := make([]byte, 3)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		randomStr := strings.ToUpper(hex.EncodeToString(randomBytes))

		newRef := fmt.Sprintf("BL-%s-%s", todayStr, randomStr)

		// Check if exists
		var count int
		err := r.db.Get(&count, `SELECT COUNT(*) FROM bookings WHERE booking_reference = $1`, newRef)
		if err != nil {
			return "", fmt.Errorf("failed to check reference uniqueness: %w", err)
		}

		if count == 0 {
			return newRef, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique booking reference after 10 attempts")
}

// GenerateBusBookingQR generates a unique QR code for bus booking
// Format: QR-YYYYMMDDHHMMSS-XXXXXXXX (8 char alphanumeric)
// Example: QR-20251206143022-A1B2C3D4
func (r *AppBookingRepository) GenerateBusBookingQR() (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		// Generate 8 random bytes and take first 8 hex chars
		randomBytes := make([]byte, 4)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		randomStr := strings.ToUpper(hex.EncodeToString(randomBytes))

		timestampStr := time.Now().Format("20060102150405")
		qrData := fmt.Sprintf("QR-%s-%s", timestampStr, randomStr)

		// Check if exists - QR is stored on master bookings table, not bus_bookings
		var count int
		err := r.db.Get(&count, `SELECT COUNT(*) FROM bookings WHERE qr_code_data = $1`, qrData)
		if err != nil {
			return "", fmt.Errorf("failed to check QR uniqueness: %w", err)
		}

		if count == 0 {
			return qrData, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique QR code after 10 attempts")
}

// ============================================================================
// MASTER BOOKING OPERATIONS
// ============================================================================

// CreateBooking creates a new master booking and multiple bus bookings and seats in a transaction
func (r *AppBookingRepository) CreateBooking(
	booking *models.MasterBooking,
	busBookings []*models.BusBooking,
	multipleSeats [][]models.BusBookingSeat,
	tripSeatRepo *TripSeatRepository,
) (*models.BookingResponse, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Generate booking reference (use Go function, not DB function)
	bookingRef, err := r.GenerateBookingReference()
	if err != nil {
		return nil, fmt.Errorf("failed to generate booking reference: %w", err)
	}
	booking.BookingReference = bookingRef

	// 2. Insert master booking
	var deviceInfoJSON interface{}
	if booking.DeviceInfo != nil && len(booking.DeviceInfo) > 0 {
		jsonBytes, err := json.Marshal(booking.DeviceInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize device_info: %w", err)
		}
		deviceInfoJSON = string(jsonBytes)
	}

	// 3. Generate Master QR Code
	qrCode, err := r.GenerateBusBookingQR() // Reusing existing func
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}
	now := time.Now()
	booking.QRCodeData = &qrCode
	booking.QRGeneratedAt = &now

	bookingQuery := `
		INSERT INTO bookings (
			booking_reference, user_id, booking_intent_id, booking_type,
			subtotal, discount_amount, tax_amount, total_amount,
			promo_code, promo_discount_type, promo_discount_value,
			payment_status, payment_method, payment_reference, payment_gateway, paid_at, booking_status,
			passenger_name, passenger_phone, passenger_email,
			booking_source, device_info, notes,
			search_from_lounge, search_to_lounge, qr_code_data, qr_generated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, 
			$21, $22, $23, $24, $25, $26, $27
		) RETURNING id, created_at, updated_at`

	err = tx.QueryRowx(bookingQuery,
		booking.BookingReference, booking.UserID, booking.BookingIntentID, booking.BookingType,
		booking.Subtotal, booking.DiscountAmount, booking.TaxAmount, booking.TotalAmount,
		booking.PromoCode, booking.PromoDiscountType, booking.PromoDiscountValue,
		booking.PaymentStatus, booking.PaymentMethod, booking.PaymentReference, booking.PaymentGateway, booking.PaidAt, booking.BookingStatus,
		booking.PassengerName, booking.PassengerPhone, booking.PassengerEmail,
		booking.BookingSource, deviceInfoJSON, booking.Notes,
		booking.SearchFromLounge, booking.SearchToLounge,
		booking.QRCodeData, booking.QRGeneratedAt,
	).Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	allCreatedSeats := make([]models.BusBookingSeat, 0)
	// We no longer track firstQRCode here since it's on MasterBooking.

	for idx, busBooking := range busBookings {
		seats := multipleSeats[idx]

		busBooking.BookingID = booking.ID

		if busBooking.ScheduledTripID == "" {
			return nil, fmt.Errorf("failed to create bus booking %d: scheduled_trip_id is empty", idx)
		}

		busBookingQuery := `
			INSERT INTO bus_bookings (
				booking_id, scheduled_trip_id,
				boarding_stop_id, alighting_stop_id,
				number_of_seats, fare_per_seat, total_fare,
				status, special_requests, is_return
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			) RETURNING id, created_at, updated_at`

		err = tx.QueryRowx(busBookingQuery,
			busBooking.BookingID, busBooking.ScheduledTripID,
			nullableUUID(busBooking.BoardingStopID), nullableUUID(busBooking.AlightingStopID),
			busBooking.NumberOfSeats, busBooking.FarePerSeat, busBooking.TotalFare,
			busBooking.Status, busBooking.SpecialRequests, busBooking.IsReturn,
		).Scan(&busBooking.ID, &busBooking.CreatedAt, &busBooking.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to create bus booking %d: %w", idx, err)
		}

		for i := range seats {
			seats[i].BusBookingID = busBooking.ID
			seats[i].ScheduledTripID = busBooking.ScheduledTripID

			if seats[i].ScheduledTripID == "" {
				return nil, fmt.Errorf("failed to create seat booking for seat %s: scheduled_trip_id is empty", seats[i].SeatNumber)
			}

			seatQuery := `
				INSERT INTO bus_booking_seats (
					bus_booking_id, scheduled_trip_id, trip_seat_id,
					passenger_name, passenger_phone, passenger_email,
					passenger_gender, passenger_nic,
					is_primary_passenger, status
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
				) RETURNING id, created_at, updated_at`

			err = tx.QueryRowx(seatQuery,
				seats[i].BusBookingID, seats[i].ScheduledTripID, nullableUUID(seats[i].TripSeatID),
				seats[i].PassengerName, seats[i].PassengerPhone, seats[i].PassengerEmail,
				seats[i].PassengerGender, seats[i].PassengerNIC,
				seats[i].IsPrimaryPassenger, seats[i].Status,
			).Scan(&seats[i].ID, &seats[i].CreatedAt, &seats[i].UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to create seat booking for seat %s: %w", seats[i].SeatNumber, err)
			}

			if seats[i].TripSeatID != nil {
				_, err = tx.Exec(`
					UPDATE trip_seats 
					SET status = 'booked', 
					    booking_type = 'app', 
					    bus_booking_seat_id = $1,
					    updated_at = now()
					WHERE id = $2`,
					seats[i].ID, *seats[i].TripSeatID)
				if err != nil {
					return nil, fmt.Errorf("failed to update trip seat %s: %w", seats[i].SeatNumber, err)
				}
			}
			allCreatedSeats = append(allCreatedSeats, seats[i])
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	var firstBusBooking *models.BusBooking
	if len(busBookings) > 0 {
		firstBusBooking = busBookings[0]
	}

	return &models.BookingResponse{
		Booking:     booking,
		BusBooking:  firstBusBooking,
		BusBookings: busBookings,
		Seats:       allCreatedSeats,
		QRCode:      *booking.QRCodeData,
	}, nil
}

// GetBookingByID retrieves a booking by ID with all related data
func (r *AppBookingRepository) GetBookingByID(bookingID string) (*models.MasterBooking, error) {
	booking := &models.MasterBooking{}
	query := `
		SELECT id, booking_reference, user_id, booking_intent_id, booking_type,
		       subtotal, discount_amount, tax_amount, total_amount,
		       promo_code, promo_discount_type, promo_discount_value,
		       payment_status, payment_method, payment_reference, payment_gateway, paid_at,
		       booking_status, passenger_name, passenger_phone, passenger_email,
		       confirmed_at, cancelled_at, cancellation_reason, cancelled_by_user_id,
		       completed_at, qr_code_data, qr_generated_at,
		       booking_source, device_info, notes, search_from_lounge, search_to_lounge, created_at, updated_at
		FROM bookings WHERE id = $1`

	err := r.db.Get(booking, query, bookingID)
	if err != nil {
		return nil, err
	}

	// Get first bus booking for backward compatibility
	busBooking, err := r.GetBusBookingByBookingID(bookingID)
	if err == nil {
		booking.BusBooking = busBooking
	}

	// Get all bus bookings (for transit/return trips)
	busBookings, err := r.GetBusBookingsByBookingID(bookingID)
	if err == nil && len(busBookings) > 0 {
		booking.BusBookings = busBookings
	}

	// Get lounge bookings if exists
	loungeBookings, err := r.GetLoungeBookingsByBookingID(bookingID)
	if err == nil && len(loungeBookings) > 0 {
		booking.LoungeBookings = loungeBookings
	}

	// Get transport bookings if exists
	var transportBookings []models.TransportBooking
	queryTransport := `
		SELECT 
			tb.id, tb.booking_id, tb.user_id, tb.lounge_id, tb.pickup_location_id,
			tb.vehicle_type, tb.vehicle_quantity, tb.transport_price, tb.transport_date, tb.transport_time,
			tb.estimated_duration_minutes, tb.booking_reference, tb.status, tb.payment_status,
			tb.payment_reference, tb.driver_id, tb.driver_assigned_at, tb.cancellation_reason,
			tb.refund_status, tb.refund_amount, tb.created_at, tb.updated_at,
			l.lounge_name, ptl.location as pickup_location_name
		FROM transport_bookings tb
		LEFT JOIN lounges l ON tb.lounge_id = l.id
		LEFT JOIN lounge_transport_locations ptl ON tb.pickup_location_id = ptl.id
		WHERE tb.booking_id = $1 OR tb.lounge_id IN (SELECT id FROM lounge_bookings WHERE master_booking_id = $1)`
	err = r.db.Select(&transportBookings, queryTransport, bookingID)
	if err == nil && len(transportBookings) > 0 {
		booking.TransportBookings = transportBookings
	}

	return booking, nil
}

// GetBookingByReference retrieves a booking by reference
func (r *AppBookingRepository) GetBookingByReference(reference string) (*models.MasterBooking, error) {
	booking := &models.MasterBooking{}
	query := `
		SELECT id, booking_reference, user_id, booking_type,
		       bus_total, lounge_total, lounge_transport_total, pre_order_total,
		       subtotal, discount_amount, tax_amount, total_amount,
		       promo_code, promo_discount_type, promo_discount_value,
		       payment_status, payment_method, payment_reference, payment_gateway, paid_at,
		       booking_status, passenger_name, passenger_phone, passenger_email,
		       confirmed_at, cancelled_at, cancellation_reason, cancelled_by_user_id,
		       completed_at, refund_amount, refund_reference, refunded_at,
		       booking_source, device_info, notes, search_from_lounge, search_to_lounge,
		       qr_code_data, qr_generated_at, created_at, updated_at
		FROM bookings WHERE booking_reference = $1`

	err := r.db.Get(booking, query, reference)
	if err != nil {
		return nil, err
	}

	// Get first bus booking for backward compatibility
	busBooking, err := r.GetBusBookingByBookingID(booking.ID)
	if err == nil {
		booking.BusBooking = busBooking
	}

	// Get all bus bookings (for transit/return trips)
	busBookings, err := r.GetBusBookingsByBookingID(booking.ID)
	if err == nil && len(busBookings) > 0 {
		booking.BusBookings = busBookings
	}

	// Get lounge bookings if exists
	loungeBookings, err := r.GetLoungeBookingsByBookingID(booking.ID)
	if err == nil && len(loungeBookings) > 0 {
		booking.LoungeBookings = loungeBookings
	}

	// Get transport bookings if exists
	transportRepo := NewTransportBookingRepository(r.db)
	transportBookings, err := transportRepo.GetTransportBookingsByBookingID(booking.ID)
	if err == nil && len(transportBookings) > 0 {
		booking.TransportBookings = transportBookings
	}

	return booking, nil
}

// GetBookingsByUserID retrieves all bookings for a user
func (r *AppBookingRepository) GetBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			COALESCE(b.total_amount, 0) as total_amount, 
			COALESCE(b.payment_status, 'pending') as payment_status, 
			COALESCE(b.booking_status, 'pending') as booking_status,
			COALESCE(b.passenger_name, '') as passenger_name, b.created_at,
			COALESCE(
				lf.lounge_name, 
				b.search_from_lounge,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as search_from_lounge, 
			COALESCE(lt.lounge_name, b.search_to_lounge) as search_to_lounge,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as route_name, 
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			) as departure_datetime, 
			COALESCE(
				bb.number_of_seats,
				(SELECT lb.number_of_guests FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as number_of_seats,
			bb.status as bus_status, b.qr_code_data,
			NULL::uuid as master_booking_id,
			bb.scheduled_trip_id, bb.boarding_stop_id,
			EXISTS(SELECT 1 FROM transport_bookings tb WHERE tb.booking_id = b.id) as has_transport,
			(SELECT tb.status FROM transport_bookings tb WHERE tb.booking_id = b.id ORDER BY tb.created_at DESC LIMIT 1) as transport_status
		FROM bookings b
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) *
			FROM bus_bookings
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetUpcomingBookingsByUserID retrieves upcoming bookings for a user
// Only returns bookings where scheduled_trips.departure_datetime > NOW() and not cancelled/completed
func (r *AppBookingRepository) GetUpcomingBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			COALESCE(b.total_amount, 0) as total_amount, 
			COALESCE(b.payment_status, 'pending') as payment_status, 
			COALESCE(b.booking_status, 'pending') as booking_status,
			COALESCE(b.passenger_name, '') as passenger_name, b.created_at,
			COALESCE(
				lf.lounge_name, 
				b.search_from_lounge,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as search_from_lounge, 
			COALESCE(lt.lounge_name, b.search_to_lounge) as search_to_lounge,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as route_name, 
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			) as departure_datetime, 
			COALESCE(
				bb.number_of_seats,
				(SELECT lb.number_of_guests FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as number_of_seats,
			bb.status as bus_status, b.qr_code_data,
			NULL::uuid as master_booking_id,
			bb.scheduled_trip_id, bb.boarding_stop_id,
			EXISTS(SELECT 1 FROM transport_bookings tb WHERE tb.booking_id = b.id) as has_transport,
			(SELECT tb.status FROM transport_bookings tb WHERE tb.booking_id = b.id ORDER BY tb.created_at DESC LIMIT 1) as transport_status
		FROM bookings b
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) * 
			FROM bus_bookings 
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		  AND b.booking_status NOT IN ('cancelled', 'completed', 'partial_cancel')
		  AND (bb.status IS NULL OR bb.status NOT IN ('cancelled', 'completed', 'no_show'))
		  AND (
			  COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			  ) IS NULL
			  OR
			  COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			  ) > NOW() AT TIME ZONE 'Asia/Colombo' - INTERVAL '24 hours'
		  )
		ORDER BY departure_datetime ASC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetCompletedBookingsByUserID retrieves completed bookings for a user
func (r *AppBookingRepository) GetCompletedBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.total_amount, b.payment_status, b.booking_status,
			b.passenger_name, b.created_at,
			COALESCE(
				lf.lounge_name, 
				b.search_from_lounge,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as search_from_lounge, 
			COALESCE(lt.lounge_name, b.search_to_lounge) as search_to_lounge,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as route_name, 
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			) as departure_datetime, 
			COALESCE(
				bb.number_of_seats,
				(SELECT lb.number_of_guests FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as number_of_seats,
			bb.status as bus_status, b.qr_code_data,
			NULL::uuid as master_booking_id,
			bb.scheduled_trip_id, bb.boarding_stop_id,
			EXISTS(SELECT 1 FROM transport_bookings tb WHERE tb.booking_id = b.id) as has_transport,
			(SELECT tb.status FROM transport_bookings tb WHERE tb.booking_id = b.id ORDER BY tb.created_at DESC LIMIT 1) as transport_status
		FROM bookings b
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) * 
			FROM bus_bookings 
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		  AND (
			b.booking_status = 'completed'
			OR bb.status = 'completed'
			OR (b.booking_type = 'lounge_only' AND EXISTS(SELECT 1 FROM lounge_bookings lb WHERE lb.master_booking_id = b.id AND lb.status = 'completed'))
		  )
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetExpiredOrCancelledBookingsByUserID retrieves cancelled bookings AND
// expired bookings where scheduled_trips.departure_datetime < NOW() and not completed.
// This is used for the Cancelled/Expired tab in the passenger app.
func (r *AppBookingRepository) GetExpiredOrCancelledBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.total_amount, b.payment_status, b.booking_status,
			b.passenger_name, b.created_at,
			COALESCE(
				lf.lounge_name, 
				b.search_from_lounge,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as search_from_lounge, 
			COALESCE(lt.lounge_name, b.search_to_lounge) as search_to_lounge,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as route_name, 
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			) as departure_datetime, 
			COALESCE(
				bb.number_of_seats,
				(SELECT lb.number_of_guests FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as number_of_seats,
			bb.status as bus_status, b.qr_code_data,
			NULL::uuid as master_booking_id,
			bb.scheduled_trip_id, bb.boarding_stop_id,
			EXISTS(SELECT 1 FROM transport_bookings tb WHERE tb.booking_id = b.id) as has_transport,
			(SELECT tb.status FROM transport_bookings tb WHERE tb.booking_id = b.id ORDER BY tb.created_at DESC LIMIT 1) as transport_status
		FROM bookings b
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) *
			FROM bus_bookings
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		  AND b.booking_status NOT IN ('completed')
		  AND (bb.status IS NULL OR bb.status NOT IN ('completed'))
		  AND (
			-- Explicitly cancelled by user or system
			b.booking_status IN ('cancelled', 'partial_cancel')
			OR bb.status IN ('cancelled', 'no_show')
		  )
		ORDER BY b.created_at DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// GetNotCompletedBookingsByUserID retrieves past confirmed bookings where
// scheduled_trips.departure_datetime < NOW() and not completed.
// This is used for the Not Completed tab in the passenger app.
func (r *AppBookingRepository) GetNotCompletedBookingsByUserID(userID string, limit, offset int) ([]models.BookingListItem, error) {
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.total_amount, b.payment_status, b.booking_status,
			b.passenger_name, b.created_at,
			COALESCE(
				lf.lounge_name, 
				b.search_from_lounge,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as search_from_lounge, 
			COALESCE(lt.lounge_name, b.search_to_lounge) as search_to_lounge,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as route_name, 
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			) as departure_datetime, 
			COALESCE(
				bb.number_of_seats,
				(SELECT lb.number_of_guests FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1)
			) as number_of_seats,
			bb.status as bus_status, b.qr_code_data,
			NULL::uuid as master_booking_id,
			bb.scheduled_trip_id, bb.boarding_stop_id,
			EXISTS(SELECT 1 FROM transport_bookings tb WHERE tb.booking_id = b.id) as has_transport,
			(SELECT tb.status FROM transport_bookings tb WHERE tb.booking_id = b.id ORDER BY tb.created_at DESC LIMIT 1) as transport_status
		FROM bookings b
		INNER JOIN (
			SELECT DISTINCT ON (booking_id) * 
			FROM bus_bookings 
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		INNER JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		  AND b.booking_status NOT IN ('cancelled', 'completed', 'partial_cancel')
		  AND bb.status NOT IN ('cancelled', 'completed', 'no_show')
		  AND st.departure_datetime <= NOW() AT TIME ZONE 'Asia/Colombo'
		ORDER BY st.departure_datetime DESC
		LIMIT $2 OFFSET $3`

	var bookings []models.BookingListItem
	err := r.db.Select(&bookings, query, userID, limit, offset)
	return bookings, err
}

// UpdatePaymentStatus updates payment status for a booking
func (r *AppBookingRepository) UpdatePaymentStatus(
	bookingID string,
	status models.MasterPaymentStatus,
	method, reference, gateway *string,
) error {
	query := `
		UPDATE bookings 
		SET payment_status = $1,
		    payment_method = COALESCE($2, payment_method),
		    payment_reference = COALESCE($3, payment_reference),
		    payment_gateway = COALESCE($4, payment_gateway),
		    paid_at = CASE WHEN $1 = 'paid' THEN NOW() ELSE paid_at END,
		    confirmed_at = CASE WHEN $1 = 'paid' THEN COALESCE(confirmed_at, NOW()) ELSE confirmed_at END,
		    booking_status = CASE WHEN $1 = 'paid' THEN 'confirmed' ELSE booking_status END,
		    updated_at = NOW()
		WHERE id = $5`

	_, err := r.db.Exec(query, status, method, reference, gateway, bookingID)
	return err
}

// CancelBooking cancels a booking and releases seats with refund processing
func (r *AppBookingRepository) CancelBooking(bookingID, userID string, reason *string, cancelLegs []string, refundAmount float64, refundDestination string, requestRefund bool) (*models.Refund, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Resolve master booking ID if lounge_booking id was passed
	actualBookingID := bookingID
	var exists bool
	_ = tx.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookings WHERE id = $1)", bookingID)
	if !exists {
		var mbID *string
		err = tx.Get(&mbID, "SELECT master_booking_id FROM lounge_bookings WHERE id = $1", bookingID)
		if err == nil && mbID != nil && *mbID != "" {
			actualBookingID = *mbID
		}
	}

	// Fetch booking details for reference
	var bRef string
	var bTotal float64
	var bStatus string
	_ = tx.QueryRowx("SELECT booking_reference, total_amount, booking_status FROM bookings WHERE id = $1", actualBookingID).Scan(&bRef, &bTotal, &bStatus)

	var singleLegUUID *uuid.UUID
	reasonStr := ""
	if reason != nil {
		reasonStr = *reason
	}

	if len(cancelLegs) == 0 {
		// Full cancellation of all components
		_, err = tx.Exec(`
			UPDATE bookings 
			SET booking_status = 'cancelled',
			    cancelled_at = NOW(),
			    cancelled_by_user_id = $1,
			    cancellation_reason = $2,
			    updated_at = NOW()
			WHERE id = $3`,
			userID, reason, actualBookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel booking: %w", err)
		}

		// Cancel bus bookings
		_, err = tx.Exec(`
			UPDATE bus_bookings 
			SET status = 'cancelled',
			    cancelled_at = NOW(),
			    cancellation_reason = $1,
			    updated_at = NOW()
			WHERE booking_id = $2`,
			reason, actualBookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel bus booking: %w", err)
		}

		// Cancel seats
		_, err = tx.Exec(`
			UPDATE bus_booking_seats 
			SET status = 'cancelled',
			    cancelled_at = NOW(),
			    updated_at = NOW()
			WHERE bus_booking_id IN (SELECT id FROM bus_bookings WHERE booking_id = $1)`,
			actualBookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel seat bookings: %w", err)
		}

		// Release trip seats
		_, err = tx.Exec(`
			UPDATE trip_seats 
			SET status = 'available',
			    booking_type = NULL,
			    bus_booking_seat_id = NULL,
			    updated_at = NOW()
			WHERE bus_booking_seat_id IN (
				SELECT bbs.id FROM bus_booking_seats bbs
				JOIN bus_bookings bb ON bb.id = bbs.bus_booking_id
				WHERE bb.booking_id = $1
			)`,
			actualBookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to release trip seats: %w", err)
		}

		// Cancel lounge bookings
		_, err = tx.Exec(`
			UPDATE lounge_bookings 
			SET status = 'cancelled',
			    cancelled_at = NOW(),
			    cancellation_reason = $1,
			    updated_at = NOW()
			WHERE master_booking_id = $2 OR id = $2`,
			reason, actualBookingID)
		if err != nil {
			return nil, fmt.Errorf("failed to cancel lounge bookings: %w", err)
		}

		// Cancel transport bookings
		_, _ = tx.Exec(`
			UPDATE transport_bookings 
			SET status = 'cancelled',
			    cancellation_reason = $1,
			    updated_at = NOW()
			WHERE booking_id = $2`,
			reason, actualBookingID)
	} else {
		// Partial cancellation of selected legs
		for _, legID := range cancelLegs {
			legUUID, errUUID := uuid.Parse(legID)
			if errUUID == nil && len(cancelLegs) == 1 {
				singleLegUUID = &legUUID
			}

			// 1. Check if leg is bus booking
			res, errBus := tx.Exec(`
				UPDATE bus_bookings 
				SET status = 'cancelled',
				    cancelled_at = NOW(),
				    cancellation_reason = $1,
				    updated_at = NOW()
				WHERE (id::text = $2 OR booking_id::text = $2) AND status != 'cancelled'`,
				reason, legID)
			if errBus == nil {
				if rows, _ := res.RowsAffected(); rows > 0 {
					// Release seats
					_, _ = tx.Exec(`
						UPDATE bus_booking_seats 
						SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW()
						WHERE bus_booking_id::text = $1 OR bus_booking_id IN (SELECT id FROM bus_bookings WHERE booking_id::text = $1)`,
						legID)
					_, _ = tx.Exec(`
						UPDATE trip_seats 
						SET status = 'available', booking_type = NULL, bus_booking_seat_id = NULL, updated_at = NOW()
						WHERE bus_booking_seat_id IN (
							SELECT id FROM bus_booking_seats 
							WHERE bus_booking_id::text = $1 OR bus_booking_id IN (SELECT id FROM bus_bookings WHERE booking_id::text = $1)
						)`,
						legID)
				}
			}

			// 2. Check if leg is lounge booking
			_, _ = tx.Exec(`
				UPDATE lounge_bookings 
				SET status = 'cancelled',
				    cancelled_at = NOW(),
				    cancellation_reason = $1,
				    updated_at = NOW()
				WHERE id::text = $2 AND status != 'cancelled'`,
				reason, legID)

			// 3. Check if leg is transport booking
			_, _ = tx.Exec(`
				UPDATE transport_bookings 
				SET status = 'cancelled',
				    cancellation_reason = $1,
				    updated_at = NOW()
				WHERE id::text = $2 AND status != 'cancelled'`,
				reason, legID)
		}

		// Check remaining active legs
		var remainingLegs int
		err = tx.Get(&remainingLegs, `
			SELECT 
			  (SELECT COUNT(*) FROM bus_bookings WHERE booking_id = $1 AND status != 'cancelled') +
			  (SELECT COUNT(*) FROM lounge_bookings WHERE (master_booking_id = $1 OR id = $1) AND status != 'cancelled') +
			  (SELECT COUNT(*) FROM transport_bookings WHERE booking_id = $1 AND status != 'cancelled')
		`, actualBookingID)
		if err != nil {
			remainingLegs = 0
		}

		if remainingLegs == 0 {
			_, err = tx.Exec(`
				UPDATE bookings 
				SET booking_status = 'cancelled',
				    cancelled_at = NOW(),
				    cancelled_by_user_id = $1,
				    cancellation_reason = $2,
				    updated_at = NOW()
				WHERE id = $3`,
				userID, reason, actualBookingID)
			if err != nil {
				return nil, fmt.Errorf("failed to cancel master booking: %w", err)
			}
		} else {
			_, err = tx.Exec(`
				UPDATE bookings 
				SET booking_status = 'partial_cancel',
				    cancellation_reason = $1,
				    updated_at = NOW()
				WHERE id = $2`,
				reason, actualBookingID)
			if err != nil {
				return nil, fmt.Errorf("failed to update partial cancel on master booking: %w", err)
			}
		}
	}

	var refund *models.Refund
	refRef := fmt.Sprintf("REF-%s", strings.ToUpper(uuid.New().String()[:8]))

	// Only insert into refunds table if user requested a refund
	if requestRefund && refundAmount > 0 {
		refundStatus := "pending" // Always 'pending' so Admin panel gets the refund request

		destPrefix := "[Wallet]"
		if refundDestination == "original" {
			destPrefix = "[Original Payment Method]"
		}
		formattedReason := fmt.Sprintf("%s %s", destPrefix, strings.TrimSpace(reasonStr))

		refund = &models.Refund{
			Amount:          refundAmount,
			Reason:          formattedReason,
			Status:          refundStatus,
			RefundReference: &refRef,
			RefundedAt:      nil,
		}

		actualBookingUUID, parseErr := uuid.Parse(actualBookingID)
		if parseErr == nil {
			refund.BookingID = actualBookingUUID
			refund.LegID = singleLegUUID

			err = tx.QueryRowx(`
				INSERT INTO refunds (
					id, booking_id, leg_id, transaction_id, amount, reason, status, refund_reference, created_at, refunded_at
				) VALUES (
					gen_random_uuid(), $1, $2, NULL, $3, $4, $5, $6, NOW(), NULL
				) RETURNING id, created_at
			`, refund.BookingID, refund.LegID, refund.Amount, refund.Reason, refund.Status, refund.RefundReference).StructScan(refund)
			if err != nil {
				return nil, fmt.Errorf("failed to create refund record: %w", err)
			}
		}
	}

	// If refund to wallet, credit the user's wallet
	if requestRefund && refundDestination == "wallet" && refundAmount > 0 {
		userUUID, uErr := uuid.Parse(userID)
		if uErr == nil {
			// Ensure wallet exists
			_, _ = tx.Exec(`
				INSERT INTO wallets_passenger (user_id, balance, status)
				VALUES ($1, 0.00, 'ACTIVE')
				ON CONFLICT (user_id) DO NOTHING
			`, userUUID)

			// Credit balance
			_, err = tx.Exec(`
				UPDATE wallets_passenger 
				SET balance = balance + $1, updated_at = NOW() 
				WHERE user_id = $2
			`, refundAmount, userUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to credit wallet balance: %w", err)
			}

			// Insert wallet transaction
			desc := fmt.Sprintf("Refund for booking #%s", bRef)
			_, err = tx.Exec(`
				INSERT INTO wallet_transactions 
				(wallet_id, amount, balance_before, balance_after, transaction_type, reference_type, gateway_reference, description, status)
				SELECT id, $1, balance - $1, balance, 'credit', 'refund', $2, $3, 'completed'
				FROM wallets_passenger
				WHERE user_id = $4
			`, refundAmount, refRef, desc, userUUID)
			if err != nil {
				return nil, fmt.Errorf("failed to insert wallet transaction: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return refund, nil
}

// CancelTransportBooking cancels a specific transport booking
func (r *AppBookingRepository) CancelTransportBooking(transportID string, userID string, reason *string) error {
	query := `
		UPDATE transport_bookings
		SET status = 'cancelled',
		    cancellation_reason = $1,
		    updated_at = NOW()
		WHERE id = $2 AND user_id = $3`

	res, err := r.db.Exec(query, reason, transportID, userID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("transport booking not found or unauthorized")
	}

	return nil
}

// ============================================================================
// BUS BOOKING OPERATIONS
// ============================================================================

// GetBusBookingByID retrieves bus booking by its own ID (bus_bookings.id)
func (r *AppBookingRepository) GetBusBookingByID(busBookingID string) (*models.BusBooking, error) {
	busBooking := &models.BusBooking{}
	query := `
		SELECT bb.id, bb.booking_id, bb.scheduled_trip_id,
		       bb.boarding_stop_id, bb.alighting_stop_id,
		       bb.number_of_seats, bb.fare_per_seat, bb.total_fare,
		       bb.status, bb.checked_in_at, bb.checked_in_by_user_id,
		       bb.boarded_at, bb.boarded_by_user_id, bb.completed_at,
		       bb.cancelled_at, bb.cancellation_reason,
		       bb.special_requests,
		       bb.created_at, bb.updated_at
		FROM bus_bookings bb
		WHERE bb.id = $1`

	err := r.db.Get(busBooking, query, busBookingID)
	if err != nil {
		return nil, err
	}

	// Get denormalized data via JOINs
	r.populateBusBookingDetails(busBooking)

	// Get seats
	seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
	if err == nil {
		busBooking.Seats = seats
	}

	return busBooking, nil
}

// GetBusBookingByBookingID retrieves the first bus booking by master booking ID with JOINs for denormalized data
func (r *AppBookingRepository) GetBusBookingByBookingID(bookingID string) (*models.BusBooking, error) {
	busBooking := &models.BusBooking{}
	query := `
		SELECT bb.id, bb.booking_id, bb.scheduled_trip_id,
		       bb.boarding_stop_id, bb.alighting_stop_id,
		       bb.number_of_seats, bb.fare_per_seat, bb.total_fare,
		       bb.status, bb.checked_in_at, bb.checked_in_by_user_id,
		       bb.boarded_at, bb.boarded_by_user_id, bb.completed_at,
		       bb.cancelled_at, bb.cancellation_reason,
		       bb.special_requests,
		       bb.created_at, bb.updated_at, COALESCE(bb.is_return, false) as is_return
		FROM bus_bookings bb
		WHERE bb.booking_id = $1
		ORDER BY bb.created_at ASC
		LIMIT 1`

	err := r.db.Get(busBooking, query, bookingID)
	if err != nil {
		return nil, err
	}

	// Get denormalized data via JOINs
	r.populateBusBookingDetails(busBooking)

	// Get seats
	seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
	if err == nil {
		busBooking.Seats = seats
	}

	return busBooking, nil
}

// GetBusBookingsByBookingID retrieves ALL bus bookings by master booking ID with JOINs for denormalized data
func (r *AppBookingRepository) GetBusBookingsByBookingID(bookingID string) ([]*models.BusBooking, error) {
	var busBookings []*models.BusBooking
	query := `
		SELECT bb.id, bb.booking_id, bb.scheduled_trip_id,
		       bb.boarding_stop_id, bb.alighting_stop_id,
		       bb.number_of_seats, bb.fare_per_seat, bb.total_fare,
		       bb.status, bb.checked_in_at, bb.checked_in_by_user_id,
		       bb.boarded_at, bb.boarded_by_user_id, bb.completed_at,
		       bb.cancelled_at, bb.cancellation_reason,
		       bb.special_requests,
		       bb.created_at, bb.updated_at, COALESCE(bb.is_return, false) as is_return
		FROM bus_bookings bb
		WHERE bb.booking_id = $1
		ORDER BY bb.created_at ASC`

	err := r.db.Select(&busBookings, query, bookingID)
	if err != nil {
		return nil, err
	}

	for _, busBooking := range busBookings {
		// Get denormalized data via JOINs
		r.populateBusBookingDetails(busBooking)

		// Get seats
		seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
		if err == nil {
			busBooking.Seats = seats
		}
	}

	return busBookings, nil
}

// GetBusBookingByQRCode retrieves bus booking by QR code
func (r *AppBookingRepository) GetBusBookingByQRCode(qrCode string) (*models.BusBooking, error) {
	busBooking := &models.BusBooking{}
	query := `
		SELECT bb.id, bb.booking_id, bb.scheduled_trip_id,
		       bb.boarding_stop_id, bb.alighting_stop_id,
		       bb.number_of_seats, bb.fare_per_seat, bb.total_fare,
		       bb.status, bb.checked_in_at, bb.checked_in_by_user_id,
		       bb.boarded_at, bb.boarded_by_user_id, bb.completed_at,
		       bb.cancelled_at, bb.cancellation_reason,
		       bb.special_requests,
		       bb.created_at, bb.updated_at
		FROM bus_bookings bb
		JOIN bookings b ON b.id = bb.booking_id
		WHERE b.qr_code_data = $1`

	err := r.db.Get(busBooking, query, qrCode)
	if err != nil {
		return nil, err
	}

	// Get denormalized data via JOINs
	r.populateBusBookingDetails(busBooking)

	// Get seats
	seats, err := r.GetSeatsByBusBookingID(busBooking.ID)
	if err == nil {
		busBooking.Seats = seats
	}

	return busBooking, nil
}

// GetLoungeBookingsByBookingID retrieves all lounge bookings for a master booking ID
func (r *AppBookingRepository) GetLoungeBookingsByBookingID(bookingID string) ([]models.LoungeBooking, error) {
	var bookings []models.LoungeBooking
	query := `
		SELECT 
			lb.id, lb.booking_reference, lb.user_id, lb.lounge_id, lb.master_booking_id, lb.bus_booking_id,
			lb.booking_type, lb.scheduled_arrival, lb.scheduled_departure, lb.actual_arrival, lb.actual_departure,
			lb.number_of_guests, lb.pricing_type, lb.base_price, lb.pre_order_total,
			lb.discount_amount, lb.total_amount, lb.status, lb.payment_status,
			lb.primary_guest_name, lb.primary_guest_phone, lb.promo_code, lb.special_requests,
			lb.internal_notes, lb.cancelled_at, lb.cancellation_reason, lb.created_at, lb.updated_at,
			lb.qr_code_data,
			l.lounge_name, l.address as lounge_address
		FROM lounge_bookings lb
		JOIN lounges l ON lb.lounge_id = l.id
		WHERE lb.master_booking_id = $1
		ORDER BY lb.booking_type ASC, lb.created_at ASC
	`

	rows, err := r.db.Query(query, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var booking models.LoungeBooking
		err := rows.Scan(
			&booking.ID, &booking.BookingReference, &booking.UserID, &booking.LoungeID,
			&booking.MasterBookingID, &booking.BusBookingID, &booking.BookingType,
			&booking.ScheduledArrival, &booking.ScheduledDeparture, &booking.ActualArrival, &booking.ActualDeparture,
			&booking.NumberOfGuests, &booking.PricingType, &booking.BasePrice, &booking.PreOrderTotal,
			&booking.DiscountAmount, &booking.TotalAmount, &booking.Status, &booking.PaymentStatus,
			&booking.PrimaryGuestName, &booking.PrimaryGuestPhone, &booking.PromoCode, &booking.SpecialRequests,
			&booking.InternalNotes, &booking.CancelledAt, &booking.CancellationReason, &booking.CreatedAt, &booking.UpdatedAt,
			&booking.QRCodeData,
			&booking.LoungeName, &booking.LoungeAddress,
		)
		if err != nil {
			return nil, err
		}

		// Get guests
		var guests []models.LoungeBookingGuest
		guestQuery := `
			SELECT id, lounge_booking_id, guest_name, guest_phone, is_primary_guest, checked_in_at, created_at
			FROM lounge_booking_guests
			WHERE lounge_booking_id = $1
			ORDER BY is_primary_guest DESC, created_at ASC
		`
		err = r.db.Select(&guests, guestQuery, booking.ID)
		if err == nil {
			booking.Guests = guests
		}

		// Get pre-orders
		var preOrders []models.LoungeBookingPreOrder
		preOrderQuery := `
			SELECT id, lounge_booking_id, product_id, product_name, product_type, product_image_url, quantity, unit_price, total_price, created_at
			FROM lounge_booking_pre_orders
			WHERE lounge_booking_id = $1
			ORDER BY created_at ASC
		`
		err = r.db.Select(&preOrders, preOrderQuery, booking.ID)
		if err == nil {
			booking.PreOrders = preOrders
		}

		bookings = append(bookings, booking)
	}

	return bookings, rows.Err()
}

// GetBusBookingsByTripID retrieves all bus bookings for a scheduled trip
func (r *AppBookingRepository) GetBusBookingsByTripID(tripID string) ([]models.BusBooking, error) {
	query := `
		SELECT bb.id, bb.booking_id, bb.scheduled_trip_id,
		       bb.boarding_stop_id, bb.alighting_stop_id,
		       bb.number_of_seats, bb.fare_per_seat, bb.total_fare,
		       bb.status, bb.checked_in_at, bb.checked_in_by_user_id,
		       bb.boarded_at, bb.boarded_by_user_id, bb.completed_at,
		       bb.cancelled_at, bb.cancellation_reason,
		       bb.special_requests,
		       bb.created_at, bb.updated_at
		FROM bus_bookings bb
		WHERE bb.scheduled_trip_id = $1 AND bb.status != 'cancelled'
		ORDER BY bb.created_at DESC`

	var bookings []models.BusBooking
	err := r.db.Select(&bookings, query, tripID)
	if err != nil {
		return nil, err
	}

	// Populate denormalized data for each booking
	for i := range bookings {
		r.populateBusBookingDetails(&bookings[i])
	}

	return bookings, err
}

// populateBusBookingDetails fetches denormalized data via JOINs
func (r *AppBookingRepository) populateBusBookingDetails(bb *models.BusBooking) {
	// Get route name, bus info, stop names, departure time
	var details struct {
		RouteName         string    `db:"route_name"`
		BusNumber         string    `db:"bus_number"`
		BusType           string    `db:"bus_type"`
		BoardingStopName  string    `db:"boarding_stop_name"`
		AlightingStopName string    `db:"alighting_stop_name"`
		DepartureDatetime time.Time `db:"departure_datetime"`
	}

	query := `
		SELECT 
			COALESCE(mr.route_name, bor.custom_route_name, 'Unknown Route') as route_name,
			COALESCE(b.bus_number, '') as bus_number,
			COALESCE(b.bus_type, '') as bus_type,
			COALESCE(mrs_board.stop_name, '') as boarding_stop_name,
			COALESCE(mrs_alight.stop_name, '') as alighting_stop_name,
			st.departure_datetime
		FROM bus_bookings bb
		JOIN scheduled_trips st ON bb.scheduled_trip_id = st.id
		LEFT JOIN bus_owner_routes bor ON st.bus_owner_route_id = bor.id
		LEFT JOIN master_routes mr ON bor.master_route_id = mr.id
		LEFT JOIN route_permits rp ON st.permit_id = rp.id
		LEFT JOIN buses b ON b.permit_id = rp.id
		LEFT JOIN master_route_stops mrs_board ON bb.boarding_stop_id = mrs_board.id
		LEFT JOIN master_route_stops mrs_alight ON bb.alighting_stop_id = mrs_alight.id
		WHERE bb.id = $1`

	err := r.db.Get(&details, query, bb.ID)
	if err == nil {
		bb.RouteName = details.RouteName
		bb.BusNumber = details.BusNumber
		bb.BusType = details.BusType
		bb.BoardingStopName = details.BoardingStopName
		bb.AlightingStopName = details.AlightingStopName
		bb.DepartureDatetime = &details.DepartureDatetime
	}
}

// UpdateBusBookingStatus updates bus booking status (check-in, board, complete)
func (r *AppBookingRepository) UpdateBusBookingStatus(
	busBookingID string,
	status models.BusBookingStatus,
	staffUserID *string,
) error {
	var query string
	switch status {
	case models.BusBookingCheckedIn:
		query = `UPDATE bus_bookings SET status = $1, checked_in_at = NOW(), checked_in_by_user_id = $2, updated_at = NOW() WHERE id = $3`
	case models.BusBookingBoarded:
		query = `UPDATE bus_bookings SET status = $1, boarded_at = NOW(), boarded_by_user_id = $2, updated_at = NOW() WHERE id = $3`
	case models.BusBookingCompleted:
		query = `UPDATE bus_bookings SET status = $1, completed_at = NOW(), updated_at = NOW() WHERE id = $3`
	default:
		query = `UPDATE bus_bookings SET status = $1, updated_at = NOW() WHERE id = $3`
	}

	_, err := r.db.Exec(query, status, staffUserID, busBookingID)
	return err
}

// ============================================================================
// SEAT OPERATIONS
// ============================================================================

// GetSeatsByBusBookingID retrieves seats for a bus booking with JOINs for seat info
func (r *AppBookingRepository) GetSeatsByBusBookingID(busBookingID string) ([]models.BusBookingSeat, error) {
	query := `
		SELECT bbs.id, bbs.bus_booking_id, bbs.scheduled_trip_id, bbs.trip_seat_id,
		       bbs.passenger_name, bbs.passenger_phone, bbs.passenger_email,
		       bbs.passenger_gender, bbs.passenger_nic,
		       bbs.is_primary_passenger, bbs.status,
		       bbs.cancelled_at, bbs.created_at, bbs.updated_at,
		       ts.seat_number, ts.seat_type, ts.seat_price
		FROM bus_booking_seats bbs
		LEFT JOIN trip_seats ts ON bbs.trip_seat_id = ts.id
		WHERE bbs.bus_booking_id = $1
		ORDER BY ts.seat_number`

	// Custom struct for scanning with JOINed data
	type seatWithDetails struct {
		models.BusBookingSeat
		SeatNumberDB string  `db:"seat_number"`
		SeatTypeDB   string  `db:"seat_type"`
		SeatPriceDB  float64 `db:"seat_price"`
	}

	var rawSeats []seatWithDetails
	err := r.db.Select(&rawSeats, query, busBookingID)
	if err != nil {
		return nil, err
	}

	// Convert to BusBookingSeat with denormalized fields
	seats := make([]models.BusBookingSeat, len(rawSeats))
	for i, raw := range rawSeats {
		seats[i] = raw.BusBookingSeat
		seats[i].SeatNumber = raw.SeatNumberDB
		seats[i].SeatType = raw.SeatTypeDB
		seats[i].SeatPrice = raw.SeatPriceDB
	}

	return seats, nil
}

// CheckSeatAvailability checks if seats are available for booking
func (r *AppBookingRepository) CheckSeatAvailability(tripSeatIDs []string) ([]models.TripSeat, error) {
	if len(tripSeatIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, scheduled_trip_id, seat_number, seat_type, row_number, position,
		       seat_price, status, booking_type
		FROM trip_seats
		WHERE id = ANY($1)`

	var seats []models.TripSeat
	err := r.db.Select(&seats, query, tripSeatIDs)
	if err != nil {
		return nil, err
	}

	// Check all seats are available
	for _, seat := range seats {
		if seat.Status != models.TripSeatStatusAvailable {
			return nil, fmt.Errorf("seat %s is not available (status: %s)", seat.SeatNumber, seat.Status)
		}
	}

	return seats, nil
}

// CountBookingsByTripID counts confirmed bookings for a trip
func (r *AppBookingRepository) CountBookingsByTripID(tripID string) (int, error) {
	var count int
	err := r.db.Get(&count, `
		SELECT COUNT(*) FROM bus_bookings 
		WHERE scheduled_trip_id = $1 AND status NOT IN ('cancelled')`,
		tripID)
	return count, err
}

// CountSeatsByTripID counts booked seats for a trip
func (r *AppBookingRepository) CountSeatsByTripID(tripID string) (int, error) {
	var count int
	err := r.db.Get(&count, `
		SELECT COUNT(*) FROM bus_booking_seats 
		WHERE scheduled_trip_id = $1 AND status NOT IN ('cancelled')`,
		tripID)
	return count, err
}

// ============================================================================
// STAFF OPERATIONS (for conductor/driver app)
// ============================================================================

// GetBookingsForTrip retrieves all bookings for a trip (for staff to manage)
func (r *AppBookingRepository) GetBookingsForTrip(tripID string) ([]models.BusBooking, error) {
	return r.GetBusBookingsByTripID(tripID)
}

// CheckInBusBooking marks a bus booking as checked in (all seats)
func (r *AppBookingRepository) CheckInBusBooking(busBookingID, staffUserID string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update bus booking
	_, err = tx.Exec(`
		UPDATE bus_bookings 
		SET status = 'checked_in', 
		    checked_in_at = NOW(),
		    checked_in_by_user_id = $1,
		    updated_at = NOW()
		WHERE id = $2`,
		staffUserID, busBookingID)
	if err != nil {
		return err
	}

	// Update all seats
	_, err = tx.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'checked_in',
		    checked_in_at = NOW(),
		    updated_at = NOW()
		WHERE bus_booking_id = $1 AND status = 'booked'`,
		busBookingID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CheckInPassenger marks a specific seat as checked in
func (r *AppBookingRepository) CheckInPassenger(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'checked_in',
		    checked_in_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}

// BoardPassenger marks a specific seat as boarded
func (r *AppBookingRepository) BoardPassenger(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'boarded',
		    boarded_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}

// MarkNoShow marks a specific seat as no-show
func (r *AppBookingRepository) MarkNoShow(seatID, staffUserID string) error {
	_, err := r.db.Exec(`
		UPDATE bus_booking_seats 
		SET status = 'no_show',
		    updated_at = NOW()
		WHERE id = $1`,
		seatID)
	return err
}

// GetCancelledBookingsWithRefundsByUserID retrieves cancelled/partial_cancel bookings with their refund records
func (r *AppBookingRepository) GetCancelledBookingsWithRefundsByUserID(userID string, limit, offset int) ([]models.CancelledBookingWithRefund, error) {
	query := `
		SELECT 
			b.id,
			b.booking_reference,
			b.booking_type,
			CAST(b.total_amount AS FLOAT) as total_amount,
			b.booking_status,
			b.cancelled_at,
			b.cancellation_reason,
			b.created_at,
			COALESCE(
				bor.custom_route_name,
				(SELECT lb.lounge_name FROM lounge_bookings lb WHERE lb.master_booking_id = b.id ORDER BY lb.created_at ASC LIMIT 1),
				'Trip Booking'
			) as route_name,
			COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id),
				b.created_at
			) as departure_datetime,
			r.id as refund_id,
			CAST(COALESCE(r.amount, 0) AS FLOAT) as refund_amount,
			COALESCE(r.status, 'none') as refund_status,
			r.refund_reference,
			r.reason as refund_reason,
			r.created_at as refund_created_at,
			r.refunded_at
		FROM bookings b
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) *
			FROM bus_bookings
			ORDER BY booking_id, created_at ASC
		) bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN (
			SELECT DISTINCT ON (booking_id) *
			FROM refunds
			ORDER BY booking_id, created_at DESC
		) r ON r.booking_id = b.id
		WHERE b.user_id = $1
		  AND (b.booking_status IN ('cancelled', 'partial_cancel') OR r.id IS NOT NULL)
		ORDER BY COALESCE(b.cancelled_at, b.updated_at, b.created_at) DESC
		LIMIT $2 OFFSET $3`

	var list []models.CancelledBookingWithRefund
	err := r.db.Select(&list, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return list, nil
}
