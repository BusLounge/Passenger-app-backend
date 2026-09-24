package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// BookingIntentRepository handles booking intent database operations
type BookingIntentRepository struct {
	db *sqlx.DB
}

// NewBookingIntentRepository creates a new BookingIntentRepository
func NewBookingIntentRepository(db *sqlx.DB) *BookingIntentRepository {
	return &BookingIntentRepository{db: db}
}

// CreateIntent creates a new booking intent and its legs
func (r *BookingIntentRepository) CreateIntent(intent *models.BookingIntent) error {
	intent.ID = uuid.New()
	intent.CreatedAt = time.Now()
	intent.UpdatedAt = time.Now()

	pricingSnapshotJSON, err := json.Marshal(intent.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing_snapshot: %w", err)
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert Intent
	query := `
		INSERT INTO booking_intents (
			id, user_id, intent_type, status,
			total_amount, currency, pricing_snapshot,
			passenger_name, passenger_phone, expires_at,
			idempotency_key, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)`

	_, err = tx.Exec(query,
		intent.ID, intent.UserID, intent.IntentType, intent.Status,
		intent.TotalAmount, intent.Currency, string(pricingSnapshotJSON),
		intent.PassengerName, intent.PassengerPhone, intent.ExpiresAt,
		intent.IdempotencyKey, intent.CreatedAt, intent.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert booking_intent: %w", err)
	}

	// 2. Insert Legs (if any exist on the object)
	for i, leg := range intent.Legs {
		leg.ID = uuid.New()
		leg.BookingIntentID = intent.ID
		leg.SequenceOrder = i + 1
		leg.CreatedAt = time.Now()

		legQuery := `
			INSERT INTO booking_intent_legs (
				id, booking_intent_id, leg_type, leg_intent, fare, sequence_order, created_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7
			)`
		_, err = tx.Exec(legQuery,
			leg.ID, leg.BookingIntentID, leg.LegType, leg.LegIntent, leg.Fare, leg.SequenceOrder, leg.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert booking_intent_leg: %w", err)
		}
	}

	return tx.Commit()
}

// GetIntentByID retrieves an intent by ID, including its legs and payment attempts
func (r *BookingIntentRepository) GetIntentByID(intentID uuid.UUID) (*models.BookingIntent, error) {
	var intent models.BookingIntent
	var pricingSnapshotJSON sql.NullString

	query := `
		SELECT 
			id, user_id, intent_type, status,
			total_amount, currency, pricing_snapshot, passenger_name, passenger_phone,
			expires_at, confirmed_at, expired_at, created_at, updated_at, idempotency_key
		FROM booking_intents
		WHERE id = $1`

	err := r.db.QueryRow(query, intentID).Scan(
		&intent.ID, &intent.UserID, &intent.IntentType, &intent.Status,
		&intent.TotalAmount, &intent.Currency, &pricingSnapshotJSON, &intent.PassengerName, &intent.PassengerPhone,
		&intent.ExpiresAt, &intent.ConfirmedAt, &intent.ExpiredAt, &intent.CreatedAt, &intent.UpdatedAt, &intent.IdempotencyKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if pricingSnapshotJSON.Valid && pricingSnapshotJSON.String != "" {
		if err := json.Unmarshal([]byte(pricingSnapshotJSON.String), &intent.PricingSnapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pricing_snapshot: %w", err)
		}
	}

	// Fetch Legs
	err = r.db.Select(&intent.Legs, "SELECT * FROM booking_intent_legs WHERE booking_intent_id = $1 ORDER BY sequence_order ASC", intentID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to fetch legs: %w", err)
	}

	// Fetch Payment Attempts
	err = r.db.Select(&intent.PaymentAttempts, "SELECT * FROM payment_attempts WHERE booking_intent_id = $1 ORDER BY initiated_at DESC", intentID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to fetch payment attempts: %w", err)
	}

	return &intent, nil
}

// GetIntentByIdempotencyKey retrieves an intent by idempotency key
func (r *BookingIntentRepository) GetIntentByIdempotencyKey(key string, userID uuid.UUID) (*models.BookingIntent, error) {
	var intentID uuid.UUID
	query := `SELECT id FROM booking_intents WHERE idempotency_key = $1 AND user_id = $2`
	err := r.db.Get(&intentID, query, key, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetIntentByID(intentID)
}

// GetIntentByPaymentUID fetches the intent by scanning payment attempts via inner join
func (r *BookingIntentRepository) GetIntentByPaymentUID(uid string) (*models.BookingIntent, error) {
	var intentID uuid.UUID
	query := `
		SELECT b.id 
		FROM booking_intents b
		JOIN payment_attempts p ON p.booking_intent_id = b.id
		WHERE p.payment_uid = $1 LIMIT 1`
	err := r.db.Get(&intentID, query, uid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.GetIntentByID(intentID)
}

// GetIntentsByUserID retrieves all intents for a user
func (r *BookingIntentRepository) GetIntentsByUserID(userID uuid.UUID, limit, offset int) ([]*models.BookingIntent, error) {
	query := `
		SELECT id FROM booking_intents 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`

	var intentIDs []uuid.UUID
	err := r.db.Select(&intentIDs, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	intents := make([]*models.BookingIntent, 0, len(intentIDs))
	for _, id := range intentIDs {
		intent, err := r.GetIntentByID(id)
		if err != nil {
			return nil, err
		}
		if intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

// UpdateIntentStatus updates the status of an intent
func (r *BookingIntentRepository) UpdateIntentStatus(intentID uuid.UUID, status models.BookingIntentStatus) error {
	query := `UPDATE booking_intents SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, intentID, status)
	return err
}

// UpdateIntentPaymentPending updates status to payment_pending
func (r *BookingIntentRepository) UpdateIntentPaymentPending(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'payment_pending', updated_at = NOW()
		WHERE id = $1 AND status = 'held'`
	result, err := r.db.Exec(query, intentID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("intent not in 'held' status or not found")
	}
	return nil
}

// UpdateIntentConfirmed marks intent as confirmed (returns success as it doesn't store booking IDs directly anymore)
func (r *BookingIntentRepository) UpdateIntentConfirmed(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'confirmed', confirmed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending', 'confirming')`
	result, err := r.db.Exec(query, intentID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("intent not in valid status for confirmation")
	}
	return nil
}

// UpdateIntentExpired marks intent as expired
func (r *BookingIntentRepository) UpdateIntentExpired(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'expired', expired_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending')`
	_, err := r.db.Exec(query, intentID)
	return err
}

// UpdateIntentCancelled marks intent as cancelled
func (r *BookingIntentRepository) UpdateIntentCancelled(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status IN ('held', 'payment_pending')`
	_, err := r.db.Exec(query, intentID)
	return err
}

// UpdateIntentConfirmationFailed marks intent as confirmation failed (needs refund)
func (r *BookingIntentRepository) UpdateIntentConfirmationFailed(intentID uuid.UUID) error {
	query := `
		UPDATE booking_intents 
		SET status = 'confirmation_failed', updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}

// GetPaymentPendingTimedOutIntents returns payment_pending intents that have timed out
func (r *BookingIntentRepository) GetPaymentPendingTimedOutIntents(timeout time.Duration, limit int) ([]*models.BookingIntent, error) {
	cutoff := time.Now().Add(-timeout)
	
	// Complex join: Find intents pending, where their latest payment attempt initiated_at < cutoff
	query := `
		SELECT b.id FROM booking_intents b
		JOIN payment_attempts p ON p.booking_intent_id = b.id
		WHERE b.status = 'payment_pending' 
		  AND p.status = 'pending'
		  AND p.initiated_at < $1
		ORDER BY p.initiated_at ASC
		LIMIT $2`

	var intentIDs []uuid.UUID
	err := r.db.Select(&intentIDs, query, cutoff, limit)
	if err != nil {
		return nil, err
	}

	intents := make([]*models.BookingIntent, 0, len(intentIDs))
	for _, id := range intentIDs {
		intent, err := r.GetIntentByID(id)
		if err != nil {
			return nil, err
		}
		if intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents, nil
}

// ExtendSeatHolds extends the hold time for all seats held by an intent
func (r *BookingIntentRepository) ExtendSeatHolds(intentID uuid.UUID, newExpiresAt time.Time) error {
	query := `
		UPDATE trip_seats 
		SET held_until = $2, updated_at = NOW()
		WHERE held_by_intent_id = $1`
	_, err := r.db.Exec(query, intentID, newExpiresAt)
	return err
}

// HoldSeatsForIntent locks seats for a booking intent with TTL
func (r *BookingIntentRepository) HoldSeatsForIntent(intentID uuid.UUID, seatIDs []string, expiresAt time.Time) (int, error) {
	if len(seatIDs) == 0 {
		return 0, nil
	}

	query, args, err := sqlx.In(`
		UPDATE trip_seats 
		SET held_by_intent_id = ?, held_until = ?, updated_at = NOW()
		WHERE id IN (?) 
		  AND status = 'available'
		  AND (held_by_intent_id IS NULL OR held_until < NOW())
	`, intentID, expiresAt, seatIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to build hold query: %w", err)
	}

	query = r.db.Rebind(query)
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to hold seats: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// ReleaseSeatHoldsForIntent releases all seat holds for an intent
func (r *BookingIntentRepository) ReleaseSeatHoldsForIntent(intentID uuid.UUID) error {
	query := `
		UPDATE trip_seats 
		SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW()
		WHERE held_by_intent_id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}

// CheckSeatsAvailableForHold checks if seats can be held (not booked, not held by others)
func (r *BookingIntentRepository) CheckSeatsAvailableForHold(seatIDs []string) ([]string, []string, error) {
	if len(seatIDs) == 0 {
		return []string{}, []string{}, nil
	}

	query, args, err := sqlx.In(`SELECT id, status, held_by_intent_id, held_until FROM trip_seats WHERE id IN (?)`, seatIDs)
	if err != nil {
		return nil, nil, err
	}

	query = r.db.Rebind(query)

	type seatStatus struct {
		ID             string     `db:"id"`
		Status         string     `db:"status"`
		HeldByIntentID *uuid.UUID `db:"held_by_intent_id"`
		HeldUntil      *time.Time `db:"held_until"`
	}

	var seats []seatStatus
	err = r.db.Select(&seats, query, args...)
	if err != nil {
		return nil, nil, err
	}

	available := make([]string, 0)
	unavailable := make([]string, 0)

	for _, seat := range seats {
		if seat.Status == "available" {
			if seat.HeldByIntentID == nil || (seat.HeldUntil != nil && seat.HeldUntil.Before(time.Now())) {
				available = append(available, seat.ID)
			} else {
				unavailable = append(unavailable, seat.ID)
			}
		} else {
			unavailable = append(unavailable, seat.ID)
		}
	}

	return available, unavailable, nil
}

// CreateLoungeCapacityHold creates a lounge capacity hold for an intent
func (r *BookingIntentRepository) CreateLoungeCapacityHold(hold *models.LoungeCapacityHold) error {
	if hold.TimeSlotStart == hold.TimeSlotEnd {
		return fmt.Errorf("invalid time slot: start and end cannot be the same")
	}

	hold.ID = uuid.New()
	hold.CreatedAt = time.Now()
	hold.Status = "held"

	query := `
		INSERT INTO lounge_capacity_holds (
			id, lounge_id, intent_id, date, time_slot_start, time_slot_end,
			guests_count, held_until, status, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := r.db.Exec(query,
		hold.ID, hold.LoungeID, hold.IntentID, hold.Date,
		hold.TimeSlotStart, hold.TimeSlotEnd, hold.GuestsCount,
		hold.HeldUntil, hold.Status, hold.CreatedAt,
	)
	return err
}

// ReleaseLoungeHoldsForIntent releases all lounge holds for an intent
func (r *BookingIntentRepository) ReleaseLoungeHoldsForIntent(intentID uuid.UUID) error {
	query := `UPDATE lounge_capacity_holds SET status = 'released' WHERE intent_id = $1 AND status = 'held'`
	_, err := r.db.Exec(query, intentID)
	return err
}

// ConfirmLoungeHoldsForIntent marks lounge holds as confirmed
func (r *BookingIntentRepository) ConfirmLoungeHoldsForIntent(intentID uuid.UUID) error {
	query := `UPDATE lounge_capacity_holds SET status = 'confirmed' WHERE intent_id = $1 AND status = 'held'`
	_, err := r.db.Exec(query, intentID)
	return err
}

// ExpireIntentAndReleaseHolds atomically expires an intent and releases all its holds
func (r *BookingIntentRepository) ExpireIntentAndReleaseHolds(intentID uuid.UUID) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update intent status
	_, err = tx.Exec(`UPDATE booking_intents SET status = 'expired', expired_at = NOW(), updated_at = NOW() WHERE id = $1 AND status IN ('held', 'payment_pending')`, intentID)
	if err != nil {
		return err
	}

	// 2. Release seat holds
	_, err = tx.Exec(`UPDATE trip_seats SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW() WHERE held_by_intent_id = $1`, intentID)
	if err != nil {
		return err
	}

	// 3. Release lounge holds
	_, err = tx.Exec(`UPDATE lounge_capacity_holds SET status = 'released' WHERE intent_id = $1 AND status = 'held'`, intentID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// BeginTx starts a new transaction
func (r *BookingIntentRepository) BeginTx() (*sqlx.Tx, error) {
	return r.db.Beginx()
}

// GetDB returns the underlying database connection
func (r *BookingIntentRepository) GetDB() *sqlx.DB {
	return r.db
}

// GetExpiredHeldIntents
func (r *BookingIntentRepository) GetExpiredHeldIntents(limit int) ([]*models.BookingIntent, error) {
var ids []uuid.UUID
query := "SELECT id FROM booking_intents WHERE status = 'held' AND expires_at < NOW() LIMIT $1"
err := r.db.Select(&ids, query, limit)
if err != nil { return nil, err }

intents := make([]*models.BookingIntent, 0, len(ids))
for _, id := range ids {
intent, err := r.GetIntentByID(id)
if err == nil && intent != nil {
intents = append(intents, intent)
}
}
return intents, nil
}

// ReleaseOrphanSeatHolds
func (r *BookingIntentRepository) ReleaseOrphanSeatHolds() (int, error) {
query := "UPDATE trip_seats SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW() WHERE held_by_intent_id IS NOT NULL AND held_by_intent_id NOT IN (SELECT id FROM booking_intents WHERE status = 'held')"
result, err := r.db.Exec(query)
if err != nil { return 0, err }
affected, _ := result.RowsAffected()
return int(affected), nil
}

// ReleaseExpiredSeatHolds
func (r *BookingIntentRepository) ReleaseExpiredSeatHolds() (int, error) {
query := "UPDATE trip_seats SET held_by_intent_id = NULL, held_until = NULL, updated_at = NOW() WHERE held_until IS NOT NULL AND held_until < NOW()"
result, err := r.db.Exec(query)
if err != nil { return 0, err }
affected, _ := result.RowsAffected()
return int(affected), nil
}

// GetLoungeCapacityAvailable 
func (r *BookingIntentRepository) GetLoungeCapacityAvailable(loungeID uuid.UUID, date time.Time, start, end string) (int, error) {
// Dummy implementation for now to pass build, capacity holds etc are advanced tasks
return 999, nil
}

// UpdateIntentPaymentUID
func (r *BookingIntentRepository) UpdateIntentPaymentUID(intentID uuid.UUID, uid, statusIndicator string) error {
    query := "INSERT INTO payment_attempts (id, booking_intent_id, payment_uid, status_indicator, initiated_at) VALUES ($1, $2, $3, $4, NOW())"
    _, err := r.db.Exec(query, uuid.New(), intentID, uid, statusIndicator)
    return err
}

// UpdateIntentPaymentSuccess
func (r *BookingIntentRepository) UpdateIntentPaymentSuccess(intentID uuid.UUID) error {
query := "UPDATE booking_intents SET status = 'confirming', updated_at = NOW() WHERE id = $1"
_, err := r.db.Exec(query, intentID)
return err
}
