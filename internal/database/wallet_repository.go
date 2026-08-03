package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type WalletRepository interface {
	GetWalletByUserID(userID uuid.UUID) (*models.Wallet, error)
	GetWalletTransactions(walletID uuid.UUID) ([]models.WalletTransaction, error)
	ConfirmTopUp(userID uuid.UUID, amount float64, gatewayRef string) error
}

type walletRepository struct {
	db *sqlx.DB
}

func NewWalletRepository(db *sqlx.DB) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) GetWalletByUserID(userID uuid.UUID) (*models.Wallet, error) {
	var wallet models.Wallet
	err := r.db.Get(&wallet, "SELECT * FROM wallets WHERE user_id = $1", userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Auto-create wallet for user on first access
			err = r.db.QueryRowx(`
				INSERT INTO wallets (user_id, balance, status)
				VALUES ($1, 0, 'ACTIVE')
				RETURNING *
			`, userID).StructScan(&wallet)
			if err != nil {
				return nil, fmt.Errorf("failed to create wallet: %v", err)
			}
			return &wallet, nil
		}
		return nil, err
	}
	return &wallet, nil
}

func (r *walletRepository) GetWalletTransactions(walletID uuid.UUID) ([]models.WalletTransaction, error) {
	var transactions []models.WalletTransaction
	err := r.db.Select(&transactions, `
		SELECT * FROM wallet_transactions 
		WHERE wallet_id = $1 
		ORDER BY created_at DESC 
		LIMIT 50
	`, walletID)
	return transactions, err
}

func (r *walletRepository) ConfirmTopUp(userID uuid.UUID, amount float64, gatewayRef string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	wallet, err := r.GetWalletByUserID(userID)
	if err != nil {
		return err
	}

	// Double entry check: avoid duplicate webhooks/callbacks
	var exists int
	err = tx.Get(&exists, "SELECT COUNT(*) FROM wallet_transactions WHERE gateway_reference = $1", gatewayRef)
	if err == nil && exists > 0 {
		return fmt.Errorf("transaction already confirmed")
	}

	// Insert transaction
	_, err = tx.Exec(`
		INSERT INTO wallet_transactions 
		(wallet_id, amount, transaction_type, reference_type, gateway_reference, description) 
		VALUES ($1, $2, 'CREDIT', 'TOPUP', $3, 'Top up via PayHere')
	`, wallet.ID, amount, gatewayRef)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	// Update wallet balance
	_, err = tx.Exec("UPDATE wallets SET balance = balance + $1 WHERE id = $2", amount, wallet.ID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return tx.Commit()
}
