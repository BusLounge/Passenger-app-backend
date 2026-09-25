package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                uuid.UUID      `json:"id" db:"id"`
	UserID            uuid.UUID      `json:"user_id" db:"user_id"`
	ReferenceType     string         `json:"reference_type" db:"reference_type"` // booking, lounge_order, transport_service, wallet_topup
	ReferenceID       *uuid.UUID     `json:"reference_id" db:"reference_id"`
	PaymentSource     string         `json:"payment_source" db:"payment_source"` // wallet, payhere, payable
	ProviderReference *string        `json:"provider_reference" db:"provider_reference"`
	Subtotal          float64        `json:"subtotal" db:"subtotal"`
	TaxAmount         float64        `json:"tax_amount" db:"tax_amount"`
	TotalAmount       float64        `json:"total_amount" db:"total_amount"`
	PriceBreakdown    TransactionPriceBreakdown `json:"price_breakdown" db:"price_breakdown"`
	Status            string         `json:"status" db:"status"` // pending, success, failed, reversed
	CreatedAt         time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at" db:"updated_at"`
}

type TransactionPriceBreakdown map[string]interface{}

// Value makes TransactionPriceBreakdown implement the driver.Valuer interface.
func (pb TransactionPriceBreakdown) Value() (driver.Value, error) {
	if len(pb) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(pb)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan makes TransactionPriceBreakdown implement the sql.Scanner interface.
func (pb *TransactionPriceBreakdown) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	b, ok := value.([]byte)
	if !ok {
		s, ok := value.(string)
		if !ok {
			return nil
		}
		b = []byte(s)
	}
	
	if len(b) == 0 {
		return nil
	}
	
	return json.Unmarshal(b, pb)
}
