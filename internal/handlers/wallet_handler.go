package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

type WalletHandler struct {
	walletService *services.WalletService
	logger        *logrus.Logger
}

func NewWalletHandler(walletService *services.WalletService, logger *logrus.Logger) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
		logger:        logger,
	}
}

// GetWallet retrieves the user's wallet info and transactions
func (h *WalletHandler) GetWallet(c *gin.Context) {
	// Get user context from middleware
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format: " + err.Error()})
		return
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
