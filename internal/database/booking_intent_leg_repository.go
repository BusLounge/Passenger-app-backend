package database

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

type BookingIntentLegRepository interface {
	CreateBookingIntentLeg(leg *models.BookingIntentLeg) error
	GetLegsByIntentID(intentID uuid.UUID) ([]models.BookingIntentLeg, error)
	UpdateLegBookingIDs(id uuid.UUID, busBookingID *uuid.UUID, loungeBookingID *uuid.UUID) error
	DeleteLegsByIntentID(intentID uuid.UUID) error
}

type bookingIntentLegRepository struct {
	db *sqlx.DB
}

func NewBookingIntentLegRepository(db *sqlx.DB) BookingIntentLegRepository {
	return &bookingIntentLegRepository{db: db}
}

func (r *bookingIntentLegRepository) CreateBookingIntentLeg(leg *models.BookingIntentLeg) error {
	query := `
		INSERT INTO booking_intent_legs (
			booking_intent_id, leg_type, leg_intent, fare, bus_booking_id, lounge_booking_id, sequence_order
		) VALUES (
			:booking_intent_id, :leg_type, :leg_intent, :fare, :bus_booking_id, :lounge_booking_id, :sequence_order
		)
		RETURNING id, created_at
	`
	
	stmt, err := r.db.PrepareNamed(query)
	if err != nil {
		return fmt.Errorf("failed to prepare create booking intent leg statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.Get(leg, leg)
	if err != nil {
		return fmt.Errorf("failed to create booking intent leg: %w", err)
	}

	return nil
}

func (r *bookingIntentLegRepository) GetLegsByIntentID(intentID uuid.UUID) ([]models.BookingIntentLeg, error) {
	var legs []models.BookingIntentLeg
	query := `
		SELECT * FROM booking_intent_legs 
		WHERE booking_intent_id = $1 
		ORDER BY sequence_order ASC
	`
	err := r.db.Select(&legs, query, intentID)
	return legs, err
}

func (r *bookingIntentLegRepository) UpdateLegBookingIDs(id uuid.UUID, busBookingID *uuid.UUID, loungeBookingID *uuid.UUID) error {
	query := `
		UPDATE booking_intent_legs 
		SET bus_booking_id = COALESCE($1, bus_booking_id),
		    lounge_booking_id = COALESCE($2, lounge_booking_id)
		WHERE id = $3
	`
	_, err := r.db.Exec(query, busBookingID, loungeBookingID, id)
	return err
}

func (r *bookingIntentLegRepository) DeleteLegsByIntentID(intentID uuid.UUID) error {
	query := `DELETE FROM booking_intent_legs WHERE booking_intent_id = $1`
	_, err := r.db.Exec(query, intentID)
	return err
}
