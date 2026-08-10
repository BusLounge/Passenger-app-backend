package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// CallLogHandler handles call log related requests
type CallLogHandler struct {
	service services.CallLogService
	logger  *logrus.Logger
}

// NewCallLogHandler creates a new CallLogHandler
func NewCallLogHandler(service services.CallLogService, logger *logrus.Logger) *CallLogHandler {
	return &CallLogHandler{
		service: service,
		logger:  logger,
	}
}

// InitiateCall creates a new call log entry
func (h *CallLogHandler) InitiateCall(c *gin.Context) {
	var req services.InitiateCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	callLog, err := h.service.InitiateCall(req)
	if err != nil {
		h.logger.Errorf("Failed to initiate call: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create call log"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Call log created",
		"data":    callLog,
	})
}

// UpdateCallStatus updates the status of a call log
func (h *CallLogHandler) UpdateCallStatus(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid call log ID format"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	err = h.service.UpdateCallStatus(id, req.Status)
	if err != nil {
		h.logger.Errorf("Failed to update call status for ID %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update call status", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Call status updated successfully"})
}

// EndCall marks a call as ended with its final duration
func (h *CallLogHandler) EndCall(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid call log ID format"})
		return
	}

	var req struct {
		Duration int `json:"duration_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	err = h.service.EndCall(id, req.Duration)
	if err != nil {
		h.logger.Errorf("Failed to end call for ID %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to end call", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Call ended successfully"})
}

// GetUserCallLogs gets paginated call history for a user
func (h *CallLogHandler) GetUserCallLogs(c *gin.Context) {
	// The user ID should preferably come from the JWT context middleware
	userIDStr, exists := c.Get("user_id")
	var userID uuid.UUID
	var err error

	if !exists {
		// Fallback to path/query param if not in context
		idParam := c.Query("user_id")
		if idParam == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context or query string"})
			return
		}
		userIDStr = idParam
	}

	userID, err = uuid.Parse(userIDStr.(string))
	if err != nil {
		// If the assertion or parsing fails, try passing directly if it was from query
		userID, err = uuid.Parse(fmt.Sprintf("%v", userIDStr))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
			return
		}
	}

	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, err := h.service.GetUserCallLogs(userID, limit, offset)
	if err != nil {
		h.logger.Errorf("Failed to get call logs for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve call logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   logs,
		"limit":  limit,
		"offset": offset,
	})
}
