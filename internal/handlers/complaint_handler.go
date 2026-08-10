package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// ComplaintHandler processes HTTP requests for complaints
type ComplaintHandler struct {
	service services.ComplaintService
	logger  *logrus.Logger
}

// NewComplaintHandler creates a new handler
func NewComplaintHandler(service services.ComplaintService, logger *logrus.Logger) *ComplaintHandler {
	return &ComplaintHandler{
		service: service,
		logger:  logger,
	}
}

// SubmitComplaint handles submission of a complaint
func (h *ComplaintHandler) SubmitComplaint(c *gin.Context) {
	var req services.SubmitComplaintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	complaint, err := h.service.SubmitComplaint(req)
	if err != nil {
		h.logger.Errorf("Failed to submit complaint: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit complaint"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Complaint submitted successfully",
		"data":    complaint,
	})
}

// GetAllComplaints gets all complaints for admin view
func (h *ComplaintHandler) GetAllComplaints(c *gin.Context) {
	complaints, err := h.service.GetAllComplaints()
	if err != nil {
		h.logger.Errorf("Failed to fetch all complaints: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve complaints"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": complaints})
}

// GetUserComplaints fetches complaints matching the authenticated user
func (h *ComplaintHandler) GetUserComplaints(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		// Fallback to query if bypassing JWT context 
		userIDStr = c.Query("user_id")
		if userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID required in context or query"})
			return
		}
	}

	userID, err := uuid.Parse(fmt.Sprintf("%v", userIDStr))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	complaints, err := h.service.GetUserComplaints(userID)
	if err != nil {
		h.logger.Errorf("Failed to fetch user complaints for %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve your complaints"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": complaints})
}

// UpdateStatus allows admin to update the status and activity logs
func (h *ComplaintHandler) UpdateStatus(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid complaint ID format"})
		return
	}

	var req struct {
		Status   string  `json:"status" binding:"required"`
		Activity *string `json:"activity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	err = h.service.UpdateStatus(id, req.Status, req.Activity)
	if err != nil {
		h.logger.Errorf("Failed to update complaint status %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update complaint", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Complaint updated successfully"})
}
