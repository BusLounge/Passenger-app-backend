package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// ComplaintRepository handles database operations for complaints
type ComplaintRepository struct {
	db DB
}

// NewComplaintRepository creates a new complaint repository
func NewComplaintRepository(db DB) *ComplaintRepository {
	return &ComplaintRepository{
		db: db,
	}
}

// CreateComplaint adds a new complaint to the database
func (r *ComplaintRepository) CreateComplaint(complaint *models.Complaint) (*models.Complaint, error) {
	complaint.ID = uuid.New()
	now := time.Now()
	complaint.CreatedAt = &now
	if complaint.Status == "" {
		complaint.Status = "Pending"
	}
	if complaint.PhotoVideo == "" {
		complaint.PhotoVideo = "Empty"
	}

	query := `
		INSERT INTO complaints (
			id, user_id, user_role, name, contact, category,
			complaint_message, photo_video, assigned_team, activity, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Exec(
		query,
		complaint.ID,
		complaint.UserID,
		complaint.UserRole,
		complaint.Name,
		complaint.Contact,
		complaint.Category,
		complaint.ComplaintMessage,
		complaint.PhotoVideo,
		complaint.AssignedTeam,
		complaint.Activity,
		complaint.Status,
		complaint.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create complaint: %w", err)
	}

	return complaint, nil
}

// GetAllComplaints retrieves all complaints from the database
func (r *ComplaintRepository) GetAllComplaints() ([]models.Complaint, error) {
	var complaints []models.Complaint
	query := `
		SELECT id, user_id, user_role, name, contact, category, complaint_message,
		       photo_video, assigned_team, activity, status, created_at
		FROM complaints
		ORDER BY created_at DESC
	`
	err := r.db.Select(&complaints, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all complaints: %w", err)
	}
	if complaints == nil {
		complaints = []models.Complaint{}
	}
	return complaints, nil
}

// GetComplaintsByUser retrieves complaints submitted by a specific user
func (r *ComplaintRepository) GetComplaintsByUser(userID uuid.UUID) ([]models.Complaint, error) {
	var complaints []models.Complaint
	query := `
		SELECT id, user_id, user_role, name, contact, category, complaint_message,
		       photo_video, assigned_team, activity, status, created_at
		FROM complaints
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	err := r.db.Select(&complaints, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user complaints: %w", err)
	}
	if complaints == nil {
		complaints = []models.Complaint{}
	}
	return complaints, nil
}

// UpdateComplaintStatus updates the status and activity of a complaint
func (r *ComplaintRepository) UpdateComplaintStatus(id uuid.UUID, status string, activity *string) error {
	query := `
		UPDATE complaints
		SET status = $1, activity = COALESCE($2, activity)
		WHERE id = $3
	`
	result, err := r.db.Exec(query, status, activity, id)
	if err != nil {
		return fmt.Errorf("failed to update complaint status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("complaint not found with id: %s", id)
	}

	return nil
}
