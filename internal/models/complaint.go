package models

import (
	"time"

	"github.com/google/uuid"
)

// Complaint represents a user complaint or feedback
type Complaint struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	UserRole         string     `json:"user_role" db:"user_role"`
	Name             string     `json:"name" db:"name"`
	Contact          string     `json:"contact" db:"contact"`
	Category         string     `json:"category" db:"category"`
	ComplaintMessage string     `json:"complaint_message" db:"complaint_message"`
	PhotoVideo       string     `json:"photo_video" db:"photo_video"`
	AssignedTeam     *string    `json:"assigned_team" db:"assigned_team"`
	Activity         *string    `json:"activity" db:"activity"`
	Status           string     `json:"status" db:"status"`
	CreatedAt        *time.Time `json:"created_at" db:"created_at"`
}
