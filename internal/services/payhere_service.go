package services

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// PayHereWebhookPayload represents the form data payload received from PayHere IPN
type PayHereWebhookPayload struct {
	MerchantID     string  `form:"merchant_id"`
	OrderID        string  `form:"order_id"`
	PaymentID      string  `form:"payment_id"`
	PayHereAmount  float64 `form:"payhere_amount"`
	PayHereCurrency string `form:"payhere_currency"`
	StatusCode     int     `form:"status_code"`
	Md5Sig         string  `form:"md5sig"`
	Custom1        string  `form:"custom_1"`
	Custom2        string  `form:"custom_2"`
	StatusMessage  string  `form:"status_message"`
	Method         string  `form:"method"`
}

type PayHereService struct {
	merchantID     string
	merchantSecret string
	logger         *logrus.Logger
}

func NewPayHereService(merchantID, merchantSecret string, logger *logrus.Logger) *PayHereService {
	// Fallback to sandbox values if not provided
	if merchantID == "" {
		merchantID = "1237200"
	}
	if merchantSecret == "" {
		merchantSecret = "MjM2MzMzNjQzODMwMjI2MDgwMDIyMTE3OTEwNDA1NTU1ODkzMTg5"
	}
	
	return &PayHereService{
		merchantID:     merchantID,
		merchantSecret: merchantSecret,
		logger:         logger,
	}
}

// GenerateHash generates the PayHere md5 verification hash
func (s *PayHereService) GenerateHash(orderID string, amount float64, currency string, statusCode int) string {
	// Formula: md5sig = UPPERCASE(MD5(merchant_id + order_id + payhere_amount + payhere_currency + status_code + UPPERCASE(MD5(merchant_secret))))
	
	amountFormatted := fmt.Sprintf("%.2f", amount) // Must be formatted to 2 decimal places as received
	
	secretHash1 := md5.Sum([]byte(s.merchantSecret))
	upperSecretHash := strings.ToUpper(hex.EncodeToString(secretHash1[:]))
	
	dataString := fmt.Sprintf("%s%s%s%s%d%s",
		s.merchantID,
		orderID,
		amountFormatted,
		currency,
		statusCode,
		upperSecretHash,
	)
	
	finalHash := md5.Sum([]byte(dataString))
	return strings.ToUpper(hex.EncodeToString(finalHash[:]))
}

// VerifyWebhook validates the MD5 signature of the incoming webhook
func (s *PayHereService) VerifyWebhook(payload PayHereWebhookPayload) bool {
	expectedHash := s.GenerateHash(payload.OrderID, payload.PayHereAmount, payload.PayHereCurrency, payload.StatusCode)

	if expectedHash != payload.Md5Sig {
		s.logger.WithFields(logrus.Fields{
			"order_id":      payload.OrderID,
			"expected_hash": expectedHash,
			"received_hash": payload.Md5Sig,
		}).Warn("PayHere webhook signature mismatch!")
		return false
	}
	
	return true
}
