package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type SavedCardRepository interface {
	GetSavedCardsByUserID(userID uuid.UUID) ([]models.SavedCard, error)
	GetSavedCardByID(cardID uuid.UUID, userID uuid.UUID) (*models.SavedCard, error)
	SaveCard(card *models.SavedCard) error
	DeleteSavedCard(cardID uuid.UUID, userID uuid.UUID) error
	SetDefaultCard(cardID uuid.UUID, userID uuid.UUID) error
}

type savedCardRepository struct {
	db *sqlx.DB
}

func NewSavedCardRepository(db *sqlx.DB) SavedCardRepository {
	return &savedCardRepository{db: db}
}

func (r *savedCardRepository) GetSavedCardsByUserID(userID uuid.UUID) ([]models.SavedCard, error) {
	var cards []models.SavedCard
	query := `
		SELECT id, user_id, card_brand, card_holder_name, card_last_four,
		       expiry_month, expiry_year, gateway_customer_token, gateway_name,
		       is_default, created_at, updated_at
		FROM user_saved_cards
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`
	err := r.db.Select(&cards, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch saved cards: %w", err)
	}
	return cards, nil
}

func (r *savedCardRepository) GetSavedCardByID(cardID uuid.UUID, userID uuid.UUID) (*models.SavedCard, error) {
	var card models.SavedCard
	query := `
		SELECT id, user_id, card_brand, card_holder_name, card_last_four,
		       expiry_month, expiry_year, gateway_customer_token, gateway_name,
		       is_default, created_at, updated_at
		FROM user_saved_cards
		WHERE id = $1 AND user_id = $2
	`
	err := r.db.Get(&card, query, cardID, userID)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (r *savedCardRepository) SaveCard(card *models.SavedCard) error {
	if card.ID == uuid.Nil {
		card.ID = uuid.New()
	}
	now := time.Now()
	card.CreatedAt = now
	card.UpdatedAt = now

	// If marked as default, unmark other cards first
	if card.IsDefault {
		_, _ = r.db.Exec("UPDATE user_saved_cards SET is_default = false WHERE user_id = $1", card.UserID)
	} else {
		// If user has no existing cards, make this one default automatically
		var count int
		_ = r.db.Get(&count, "SELECT COUNT(*) FROM user_saved_cards WHERE user_id = $1", card.UserID)
		if count == 0 {
			card.IsDefault = true
		}
	}

	query := `
		INSERT INTO user_saved_cards (
			id, user_id, card_brand, card_holder_name, card_last_four,
			expiry_month, expiry_year, gateway_customer_token, gateway_name,
			is_default, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(query,
		card.ID, card.UserID, card.CardBrand, card.CardHolderName, card.CardLastFour,
		card.ExpiryMonth, card.ExpiryYear, card.GatewayCustomerToken, card.GatewayName,
		card.IsDefault, card.CreatedAt, card.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert saved card: %w", err)
	}
	return nil
}

func (r *savedCardRepository) DeleteSavedCard(cardID uuid.UUID, userID uuid.UUID) error {
	query := "DELETE FROM user_saved_cards WHERE id = $1 AND user_id = $2"
	res, err := r.db.Exec(query, cardID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete saved card: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("saved card not found")
	}
	return nil
}

func (r *savedCardRepository) SetDefaultCard(cardID uuid.UUID, userID uuid.UUID) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Unset all
	if _, err := tx.Exec("UPDATE user_saved_cards SET is_default = false, updated_at = NOW() WHERE user_id = $1", userID); err != nil {
		return err
	}

	// 2. Set default
	res, err := tx.Exec("UPDATE user_saved_cards SET is_default = true, updated_at = NOW() WHERE id = $1 AND user_id = $2", cardID, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("saved card not found")
	}

	return tx.Commit()
}
