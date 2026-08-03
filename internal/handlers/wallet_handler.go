package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
	"github.com/smarttransit/sms-auth-backend/pkg/sms"
)

type WalletHandler struct {
	walletRepo     database.WalletRepository
	userRepository *database.UserRepository
	smsGateway     sms.SMSGateway
}

func NewWalletHandler(walletRepo database.WalletRepository, userRepository *database.UserRepository, smsGateway sms.SMSGateway) *WalletHandler {
	return &WalletHandler{
		walletRepo:     walletRepo,
		userRepository: userRepository,
		smsGateway:     smsGateway,
	}
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userCtx := middleware.MustGetUserContext(c)

	wallet, err := h.walletRepo.GetWalletByUserID(userCtx.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed retrieving wallet"})
		return
	}

	transactions, err := h.walletRepo.GetWalletTransactions(wallet.ID)
	if err != nil {
		transactions = []models.WalletTransaction{} // empty slice if error
	}

	c.JSON(http.StatusOK, gin.H{
		"wallet":       wallet,
		"transactions": transactions,
	})
}

type TopUpConfirmRequest struct {
	Amount           float64 `json:"amount" binding:"required"`
	GatewayReference string  `json:"gateway_reference" binding:"required"`
}

func (h *WalletHandler) ConfirmTopUp(c *gin.Context) {
	userCtx := middleware.MustGetUserContext(c)

	var req TopUpConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	err := h.walletRepo.ConfirmTopUp(userCtx.UserID, req.Amount, req.GatewayReference)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch user details for SMS notification
	user, err := h.userRepository.GetUserByID(userCtx.UserID)
	if err == nil && user != nil {
		// Send async SMS notification
		go func() {
			message := fmt.Sprintf("Your e-wallet was successfully topped up by LKR %.2f. Reft: %s", req.Amount, req.GatewayReference)
			// TODO: Implement generic SendSMS in smsGateway to actually dispatch this. Currently it only supports SendOTP.
			fmt.Println("[WALLET_NOTIFICATION] SMS to", user.Phone, ":", message)
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "top-up confirmed successfully",
	})
}
