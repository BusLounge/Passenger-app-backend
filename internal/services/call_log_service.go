package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// CallLogService defines the interface for the call log service
type CallLogService interface {
	InitiateCall(req InitiateCallRequest) (*models.CallLog, error)
	UpdateCallStatus(id uuid.UUID, status string) error
	EndCall(id uuid.UUID, duration int) error
	GetCallLog(id uuid.UUID) (*models.CallLog, error)
	GetUserCallLogs(userID uuid.UUID, limit, offset int) ([]models.CallLog, error)
}

type callLogService struct {
	repo   *database.CallLogRepository
	logger *logrus.Logger
}

// InitiateCallRequest represents the data needed to start a call
type InitiateCallRequest struct {
	TripID       uuid.UUID `json:"trip_id" binding:"required"`
	CallerID     uuid.UUID `json:"caller_id" binding:"required"`
	ReceiverID   uuid.UUID `json:"receiver_id" binding:"required"`
	CallerRole   string    `json:"caller_role" binding:"required"`
	ReceiverRole string    `json:"receiver_role" binding:"required"`
	ChannelName  string    `json:"channel_name" binding:"required"`
}

// NewCallLogService creates a new call log service
func NewCallLogService(repo *database.CallLogRepository, logger *logrus.Logger) CallLogService {
	return &callLogService{
		repo:   repo,
		logger: logger,
	}
}

// InitiateCall starts a new call log record
func (s *callLogService) InitiateCall(req InitiateCallRequest) (*models.CallLog, error) {
	log := &models.CallLog{
		TripID:       req.TripID,
		CallerID:     req.CallerID,
		ReceiverID:   req.ReceiverID,
		CallerRole:   req.CallerRole,
		ReceiverRole: req.ReceiverRole,
		ChannelName:  req.ChannelName,
		CallStatus:   "initiated", // Default start status
	}
	
	s.logger.Infof("Initiating call log for channel: %s", req.ChannelName)

	return s.repo.CreateCallLog(log)
}

// UpdateCallStatus updates the status of an ongoing call
func (s *callLogService) UpdateCallStatus(id uuid.UUID, status string) error {
	// Validate valid status
	validStatuses := map[string]bool{
		"initiated": true,
		"ringing":   true,
		"answered":  true,
		"ended":     true,
		"missed":    true,
		"declined":  true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid call status: %s", status)
	}

	s.logger.Infof("Updating call log %s to status: %s", id, status)
	return s.repo.UpdateCallStatus(id, status)
}

// EndCall marks a call as ended and updates the duration
func (s *callLogService) EndCall(id uuid.UUID, duration int) error {
	now := time.Now()
	s.logger.Infof("Ending call log %s with duration: %d seconds", id, duration)
	return s.repo.EndCall(id, now, duration)
}

// GetCallLog retrieves a single call log
func (s *callLogService) GetCallLog(id uuid.UUID) (*models.CallLog, error) {
	return s.repo.GetCallLogByID(id)
}

// GetUserCallLogs retrieves call logs for a specific user
func (s *callLogService) GetUserCallLogs(userID uuid.UUID, limit, offset int) ([]models.CallLog, error) {
	return s.repo.GetCallLogsByUser(userID, limit, offset)
}
