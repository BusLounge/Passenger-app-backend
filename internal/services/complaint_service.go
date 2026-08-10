package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ComplaintService defines the interface for complaint operations
type ComplaintService interface {
	SubmitComplaint(req SubmitComplaintRequest) (*models.Complaint, error)
	GetAllComplaints() ([]models.Complaint, error)
	GetUserComplaints(userID uuid.UUID) ([]models.Complaint, error)
	UpdateStatus(id uuid.UUID, status string, activity *string) error
}

type complaintService struct {
	repo   *database.ComplaintRepository
	logger *logrus.Logger
}

// SubmitComplaintRequest details needed to create a complaint
type SubmitComplaintRequest struct {
	UserID           uuid.UUID `json:"user_id" binding:"required"`
	UserRole         string    `json:"user_role" binding:"required"`
	Name             string    `json:"name" binding:"required"`
	Contact          string    `json:"contact" binding:"required"`
	Category         string    `json:"category" binding:"required"`
	ComplaintMessage string    `json:"complaint_message" binding:"required"`
	PhotoVideo       string    `json:"photo_video"`
}

// NewComplaintService initializes the complaint service
func NewComplaintService(repo *database.ComplaintRepository, logger *logrus.Logger) ComplaintService {
	return &complaintService{
		repo:   repo,
		logger: logger,
	}
}

// SubmitComplaint submits a new complaint
func (s *complaintService) SubmitComplaint(req SubmitComplaintRequest) (*models.Complaint, error) {
	complaint := &models.Complaint{
		UserID:           req.UserID,
		UserRole:         req.UserRole,
		Name:             req.Name,
		Contact:          req.Contact,
		Category:         req.Category,
		ComplaintMessage: req.ComplaintMessage,
		PhotoVideo:       req.PhotoVideo,
	}

	if complaint.PhotoVideo == "" {
		complaint.PhotoVideo = "Empty"
	}

	s.logger.Infof("Submitting new complaint for user %s (%s)", req.UserID, req.UserRole)
	return s.repo.CreateComplaint(complaint)
}

// GetAllComplaints retrieves all complaints (admin perspective)
func (s *complaintService) GetAllComplaints() ([]models.Complaint, error) {
	return s.repo.GetAllComplaints()
}

// GetUserComplaints retrieves complaints for a given user
func (s *complaintService) GetUserComplaints(userID uuid.UUID) ([]models.Complaint, error) {
	return s.repo.GetComplaintsByUser(userID)
}

// UpdateStatus modifies an existing complaint's status and activity
func (s *complaintService) UpdateStatus(id uuid.UUID, status string, activity *string) error {
	validStatuses := map[string]bool{
		"Pending":    true,
		"In Progress": true,
		"Resolved":   true,
		"Rejected":   true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid complaint status: %s", status)
	}

	s.logger.Infof("Updating complaint %s to status: %s", id, status)
	return s.repo.UpdateComplaintStatus(id, status, activity)
}
