package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

type WalletHandler struct {
	walletService  *services.WalletService
	payhereService *services.PayHereService
	logger         *logrus.Logger
}

func NewWalletHandler(walletService *services.WalletService, payhereService *services.PayHereService, logger *logrus.Logger) *WalletHandler {
	return &WalletHandler{
		walletService:  walletService,
		payhereService: payhereService,
		logger:         logger,
	}
}

// GetWallet retrieves the user's wallet info and transactions
func (h *WalletHandler) GetWallet(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	data, err := h.walletService.GetWalletData(userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// TopUpConfirmRequest represents the request body for top-up confirmation
type TopUpConfirmRequest struct {
	Amount           float64 `json:"amount" binding:"required"`
	GatewayReference string  `json:"gateway_reference" binding:"required"`
}

// ConfirmTopUp processes the top-up transaction from the frontend SDK or Webhook
func (h *WalletHandler) ConfirmTopUp(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req TopUpConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Fallback to query params for GET request bypass via Choreo gateway
		amountStr := c.Query("amount")
		gatewayRef := c.Query("gateway_reference")
		if amountStr != "" && gatewayRef != "" {
			var amount float64
			if _, scanErr := fmt.Sscanf(amountStr, "%f", &amount); scanErr == nil && amount > 0 {
				req.Amount = amount
				req.GatewayReference = gatewayRef
				err = nil
			}
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format: " + err.Error()})
			return
		}
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than zero"})
		return
	}

	err := h.walletService.ConfirmTopUp(userCtx.UserID, req.Amount, req.GatewayReference)
	if err != nil {
		if err.Error() == "failed to confirm top-up: transaction already confirmed" {
			c.JSON(http.StatusConflict, gin.H{"error": "Transaction already confirmed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Top-up confirmed successfully",
		"status":  "success",
	})
}

// GetTopUpHash generates a PayHere payment hash server-side so the merchant secret never reaches the client.
// GET /api/v1/wallet/topup/hash?order_id=WAL-xxx&amount=1000.00&currency=LKR
func (h *WalletHandler) GetTopUpHash(c *gin.Context) {
	// Must be authenticated — user can only generate a hash for themselves
	_, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	orderID := c.Query("order_id")
	amountStr := c.Query("amount")
	currency := c.Query("currency")

	if orderID == "" || amountStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id and amount are required query parameters"})
		return
	}

	if currency == "" {
		currency = "LKR"
	}

	var amount float64
	if _, err := fmt.Sscanf(amountStr, "%f", &amount); err != nil || amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a valid positive number"})
		return
	}

	if h.payhereService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PayHere service not configured"})
		return
	}

	hash := h.payhereService.GenerateClientHash(orderID, amount, currency)

	c.JSON(http.StatusOK, gin.H{
		"hash":        hash,
		"merchant_id": h.payhereService.GetMerchantID(),
		"order_id":    orderID,
		"amount":      fmt.Sprintf("%.2f", amount),
		"currency":    currency,
	})
}
