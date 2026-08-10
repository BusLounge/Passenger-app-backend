package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// CallLogRepository handles call logs database operations
type CallLogRepository struct {
	db DB
}

// NewCallLogRepository creates a new call log repository
func NewCallLogRepository(db DB) *CallLogRepository {
	return &CallLogRepository{
		db: db,
	}
}

// CreateCallLog creates a new call log record
func (r *CallLogRepository) CreateCallLog(log *models.CallLog) (*models.CallLog, error) {
	log.ID = uuid.New()
	now := time.Now()
	log.CreatedAt = &now
	if log.StartedAt == nil {
		log.StartedAt = &now
	}
	if log.CallStatus == "" {
		log.CallStatus = "initiated" // default based on schema
	}

	query := `
		INSERT INTO call_logs (
			id, trip_id, caller_id, receiver_id, 
			caller_role, receiver_role, channel_name, call_status, 
			started_at, created_at, ended_at, duration_seconds
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Exec(
		query,
		log.ID,
		log.TripID,
		log.CallerID,
		log.ReceiverID,
		log.CallerRole,
		log.ReceiverRole,
		log.ChannelName,
		log.CallStatus,
		log.StartedAt,
		log.CreatedAt,
		log.EndedAt,
		log.DurationSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create call log: %w", err)
	}

	return log, nil
}

// UpdateCallStatus updates the status of an existing call log
func (r *CallLogRepository) UpdateCallStatus(id uuid.UUID, status string) error {
	query := `
		UPDATE call_logs
		SET call_status = $1
		WHERE id = $2
	`
	result, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update call status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("call log not found for ID: %s", id)
	}

	return nil
}

// EndCall marks the call as ended and records duration
func (r *CallLogRepository) EndCall(id uuid.UUID, endedAt time.Time, duration int) error {
	query := `
		UPDATE call_logs
		SET call_status = 'ended',
		    ended_at = $1,
		    duration_seconds = $2
		WHERE id = $3
	`

	result, err := r.db.Exec(query, endedAt, duration, id)
	if err != nil {
		return fmt.Errorf("failed to end call: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("call log not found for ID: %s", id)
	}

	return nil
}

// GetCallLogByID retrieves a single call log
func (r *CallLogRepository) GetCallLogByID(id uuid.UUID) (*models.CallLog, error) {
	var callLog models.CallLog
	query := `
		SELECT id, trip_id, caller_id, receiver_id, caller_role, receiver_role, 
		       channel_name, call_status, started_at, ended_at, duration_seconds, created_at
		FROM call_logs
		WHERE id = $1
	`
	err := r.db.Get(&callLog, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get call log by ID: %w", err)
	}
	return &callLog, nil
}

// GetCallLogsByUser retrieves all calls involving a specific user (either as caller or receiver)
func (r *CallLogRepository) GetCallLogsByUser(userID uuid.UUID, limit, offset int) ([]models.CallLog, error) {
	var logs []models.CallLog
	query := `
		SELECT id, trip_id, caller_id, receiver_id, caller_role, receiver_role, 
		       channel_name, call_status, started_at, ended_at, duration_seconds, created_at
		FROM call_logs
		WHERE caller_id = $1 OR receiver_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	err := r.db.Select(&logs, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list call logs: %w", err)
	}
	// Return empty slice instead of nil for JSON serialization
	if logs == nil {
		logs = []models.CallLog{}
	}
	return logs, nil
}
