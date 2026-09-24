package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type PaymentAttemptRepository interface {
	CreatePaymentAttempt(attempt *models.PaymentAttempt) error
	GetPaymentAttemptsByIntentID(intentID uuid.UUID) ([]models.PaymentAttempt, error)
	GetPaymentAttemptByReference(reference string) (*models.PaymentAttempt, error)
	UpdatePaymentAttemptStatus(id uuid.UUID, status models.PaymentAttemptStatus, statusIndicator *string, resolvedAt *time.Time) error
}

type paymentAttemptRepository struct {
	db *sqlx.DB
}

func NewPaymentAttemptRepository(db *sqlx.DB) PaymentAttemptRepository {
	return &paymentAttemptRepository{db: db}
}

func (r *paymentAttemptRepository) CreatePaymentAttempt(attempt *models.PaymentAttempt) error {
	query := `
		INSERT INTO payment_attempts (
			booking_intent_id, payment_gateway, payment_reference, payment_uid, 
			status, status_indicator
		) VALUES (
			:booking_intent_id, :payment_gateway, :payment_reference, :payment_uid, 
			:status, :status_indicator
		)
		RETURNING id, initiated_at
	`
	
	stmt, err := r.db.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("failed to prepare create payment attempt statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.Get(attempt, attempt)
	if err != nil {
		return fmt.Errorf("failed to create payment attempt: %w", err)
	}

	return nil
}

func (r *paymentAttemptRepository) GetPaymentAttemptsByIntentID(intentID uuid.UUID) ([]models.PaymentAttempt, error) {
	var attempts []models.PaymentAttempt
	query := `
		SELECT * FROM payment_attempts 
		WHERE booking_intent_id = $1 
		ORDER BY initiated_at DESC
	`
	err := r.db.Select(&attempts, query, intentID)
	return attempts, err
}

func (r *paymentAttemptRepository) GetPaymentAttemptByReference(reference string) (*models.PaymentAttempt, error) {
	var attempt models.PaymentAttempt
	query := `
		SELECT * FROM payment_attempts 
		WHERE payment_reference = $1 
		LIMIT 1
	`
	err := r.db.Get(&attempt, query, reference)
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *paymentAttemptRepository) UpdatePaymentAttemptStatus(id uuid.UUID, status models.PaymentAttemptStatus, statusIndicator *string, resolvedAt *time.Time) error {
	query := `
		UPDATE payment_attempts 
		SET status = $1, status_indicator = COALESCE($2, status_indicator), resolved_at = COALESCE($3, resolved_at)
		WHERE id = $4
	`
	_, err := r.db.Exec(query, status, statusIndicator, resolvedAt, id)
	return err
}
