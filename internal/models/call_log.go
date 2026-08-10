package models

import (
	"time"

	"github.com/google/uuid"
)

// CallLog represents a record of a WebRTC call
type CallLog struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	TripID          uuid.UUID  `json:"trip_id" db:"trip_id"`
	CallerID        uuid.UUID  `json:"caller_id" db:"caller_id"`
	ReceiverID      uuid.UUID  `json:"receiver_id" db:"receiver_id"`
	CallerRole      string     `json:"caller_role" db:"caller_role"`
	ReceiverRole    string     `json:"receiver_role" db:"receiver_role"`
	ChannelName     string     `json:"channel_name" db:"channel_name"`
	CallStatus      string     `json:"call_status" db:"call_status"` // 'initiated', 'ringing', 'answered', 'ended', 'missed', etc.
	StartedAt       *time.Time `json:"started_at" db:"started_at"`
	EndedAt         *time.Time `json:"ended_at" db:"ended_at"`
	DurationSeconds int        `json:"duration_seconds" db:"duration_seconds"`
	CreatedAt       *time.Time `json:"created_at" db:"created_at"`
}
