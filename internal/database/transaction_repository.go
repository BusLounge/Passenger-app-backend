package database

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type TransactionRepository interface {
	CreateTransaction(tx *sqlx.Tx, txn *models.Transaction) error
	GetTransactionsByUserID(userID uuid.UUID) ([]models.Transaction, error)
}

type transactionRepository struct {
	db *sqlx.DB
}

func NewTransactionRepository(db *sqlx.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) CreateTransaction(tx *sqlx.Tx, txn *models.Transaction) error {
	query := `
		INSERT INTO transactions 
		(user_id, reference_type, reference_id, payment_source, provider_reference, subtotal, tax_amount, total_amount, price_breakdown, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	var err error
	if tx != nil {
		err = tx.QueryRowx(query, 
			txn.UserID, txn.ReferenceType, txn.ReferenceID, txn.PaymentSource, 
			txn.ProviderReference, txn.Subtotal, txn.TaxAmount, txn.TotalAmount, 
			txn.PriceBreakdown, txn.Status,
		).StructScan(txn)
	} else {
		err = r.db.QueryRowx(query, 
			txn.UserID, txn.ReferenceType, txn.ReferenceID, txn.PaymentSource, 
			txn.ProviderReference, txn.Subtotal, txn.TaxAmount, txn.TotalAmount, 
			txn.PriceBreakdown, txn.Status,
		).StructScan(txn)
	}
	
	if err != nil {
		return fmt.Errorf("failed to create global transaction: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetTransactionsByUserID(userID uuid.UUID) ([]models.Transaction, error) {
	var txns []models.Transaction
	err := r.db.Select(&txns, `
		SELECT id, user_id, reference_type, reference_id, payment_source, provider_reference, 
		       CAST(subtotal AS FLOAT) as subtotal, CAST(tax_amount AS FLOAT) as tax_amount, CAST(total_amount AS FLOAT) as total_amount, 
		       price_breakdown, status, created_at, updated_at
		FROM transactions 
		WHERE user_id = $1 
		ORDER BY created_at DESC
	`, userID)
	return txns, err
}
