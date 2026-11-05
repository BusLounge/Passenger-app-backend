package models

import (
	"time"

	"github.com/google/uuid"
)

// SavedCard represents a saved payment card token for a user
type SavedCard struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	UserID               uuid.UUID `json:"user_id" db:"user_id"`
	CardBrand            string    `json:"card_brand" db:"card_brand"`
	CardHolderName       *string   `json:"card_holder_name,omitempty" db:"card_holder_name"`
	CardLastFour         string    `json:"card_last_four" db:"card_last_four"`
	ExpiryMonth          string    `json:"expiry_month" db:"expiry_month"`
	ExpiryYear           string    `json:"expiry_year" db:"expiry_year"`
	GatewayCustomerToken string    `json:"gateway_customer_token" db:"gateway_customer_token"`
	GatewayName          string    `json:"gateway_name" db:"gateway_name"`
	IsDefault            bool      `json:"is_default" db:"is_default"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}

// SaveCardRequest payload when saving a new card token
type SaveCardRequest struct {
	CardBrand            string  `json:"card_brand" binding:"required"`
	CardHolderName       *string `json:"card_holder_name"`
	CardLastFour         string  `json:"card_last_four" binding:"required"`
	ExpiryMonth          string  `json:"expiry_month" binding:"required"`
	ExpiryYear           string  `json:"expiry_year" binding:"required"`
	GatewayCustomerToken string  `json:"gateway_customer_token" binding:"required"`
	GatewayName          string  `json:"gateway_name"`
	IsDefault            bool    `json:"is_default"`
}
