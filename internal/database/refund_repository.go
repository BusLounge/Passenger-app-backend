package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type RefundRepository interface {
	CreateRefund(refund *models.Refund) error
	GetRefundsByBookingID(bookingID uuid.UUID) ([]models.Refund, error)
	UpdateRefundStatus(id uuid.UUID, status string, refundRef *string) error
}

type refundRepository struct {
	db *sqlx.DB
}

func NewRefundRepository(db *sqlx.DB) RefundRepository {
	return &refundRepository{db: db}
}

func (r *refundRepository) CreateRefund(refund *models.Refund) error {
	query := `
		INSERT INTO refunds 
		(booking_id, leg_id, transaction_id, amount, reason, status, refund_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	err := r.db.QueryRowx(query, 
		refund.BookingID, refund.LegID, refund.TransactionID, 
		refund.Amount, refund.Reason, refund.Status, refund.RefundReference,
	).StructScan(refund)
	
	if err != nil {
		return fmt.Errorf("failed to create refund record: %w", err)
	}
	return nil
}

func (r *refundRepository) GetRefundsByBookingID(bookingID uuid.UUID) ([]models.Refund, error) {
	var refunds []models.Refund
	err := r.db.Select(&refunds, `
		SELECT id, booking_id, leg_id, transaction_id, CAST(amount AS FLOAT) as amount, 
		       reason, status, refund_reference, created_at, refunded_at
		FROM refunds 
		WHERE booking_id = $1 
		ORDER BY created_at DESC
	`, bookingID)
	return refunds, err
}

func (r *refundRepository) UpdateRefundStatus(id uuid.UUID, status string, refundRef *string) error {
	var err error
	if status == "completed" {
		_, err = r.db.Exec(`
			UPDATE refunds 
			SET status = $1, refund_reference = $2, refunded_at = $3
			WHERE id = $4
		`, status, refundRef, time.Now(), id)
	} else {
		_, err = r.db.Exec(`
			UPDATE refunds 
			SET status = $1, refund_reference = COALESCE($2, refund_reference)
			WHERE id = $3
		`, status, refundRef, id)
	}
	return err
}
