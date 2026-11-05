package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type SavedCardHandler struct {
	cardRepo database.SavedCardRepository
	logger   *logrus.Logger
}

func NewSavedCardHandler(cardRepo database.SavedCardRepository, logger *logrus.Logger) *SavedCardHandler {
	return &SavedCardHandler{
		cardRepo: cardRepo,
		logger:   logger,
	}
}

// GetSavedCards handles GET /api/v1/payment-methods/cards
func (h *SavedCardHandler) GetSavedCards(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	cards, err := h.cardRepo.GetSavedCardsByUserID(userCtx.UserID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch saved cards")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch saved cards"})
		return
	}

	if cards == nil {
		cards = []models.SavedCard{}
	}

	c.JSON(http.StatusOK, gin.H{"data": cards})
}

// SaveCard handles POST /api/v1/payment-methods/cards
func (h *SavedCardHandler) SaveCard(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req models.SaveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	gatewayName := req.GatewayName
	if gatewayName == "" {
		gatewayName = "payhere"
	}

	card := models.SavedCard{
		UserID:               userCtx.UserID,
		CardBrand:            req.CardBrand,
		CardHolderName:       req.CardHolderName,
		CardLastFour:         req.CardLastFour,
		ExpiryMonth:          req.ExpiryMonth,
		ExpiryYear:           req.ExpiryYear,
		GatewayCustomerToken: req.GatewayCustomerToken,
		GatewayName:          gatewayName,
		IsDefault:            req.IsDefault,
	}

	if err := h.cardRepo.SaveCard(&card); err != nil {
		h.logger.WithError(err).Error("Failed to save card")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save card"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Card saved successfully", "data": card})
}

// DeleteSavedCard handles DELETE /api/v1/payment-methods/cards/:id
func (h *SavedCardHandler) DeleteSavedCard(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	cardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid card ID"})
		return
	}

	if err := h.cardRepo.DeleteSavedCard(cardID, userCtx.UserID); err != nil {
		h.logger.WithError(err).Error("Failed to delete saved card")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Card deleted successfully"})
}

// SetDefaultCard handles PUT /api/v1/payment-methods/cards/:id/default
func (h *SavedCardHandler) SetDefaultCard(c *gin.Context) {
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	cardID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid card ID"})
		return
	}

	if err := h.cardRepo.SetDefaultCard(cardID, userCtx.UserID); err != nil {
		h.logger.WithError(err).Error("Failed to set default card")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default card updated"})
}
