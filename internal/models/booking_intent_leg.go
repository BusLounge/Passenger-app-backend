package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BookingIntentLegType represents the type of a booking leg
type BookingIntentLegType string

const (
	LegTypeBusOutbound        BookingIntentLegType = "bus_outbound"
	LegTypeBusTransit         BookingIntentLegType = "bus_transit"
	LegTypeBusReturn          BookingIntentLegType = "bus_return"
	LegTypeLoungePreOutbound  BookingIntentLegType = "lounge_pre_outbound"
	LegTypeLoungePostOutbound BookingIntentLegType = "lounge_post_outbound"
	LegTypeLoungePreReturn    BookingIntentLegType = "lounge_pre_return"
	LegTypeLoungePostReturn   BookingIntentLegType = "lounge_post_return"
	LegTypeTransitLounge      BookingIntentLegType = "transit_lounge"
	LegTypeTransport          BookingIntentLegType = "transport"
)

// BookingIntentLeg represents a single leg of a passenger's journey within a booking intent
type BookingIntentLeg struct {
	ID              uuid.UUID            `json:"id" db:"id"`
	BookingIntentID uuid.UUID            `json:"booking_intent_id" db:"booking_intent_id"`
	LegType         BookingIntentLegType `json:"leg_type" db:"leg_type"`
	
	// LegIntent stores detailed JSON metadata specific to this leg (e.g. seats, pickup time)
	LegIntent       json.RawMessage      `json:"leg_intent" db:"leg_intent"` 
	
	Fare            float64              `json:"fare" db:"fare"`
	BusBookingID    *uuid.UUID           `json:"bus_booking_id,omitempty" db:"bus_booking_id"`
	LoungeBookingID *uuid.UUID           `json:"lounge_booking_id,omitempty" db:"lounge_booking_id"`
	SequenceOrder   int                  `json:"sequence_order" db:"sequence_order"`
	CreatedAt       time.Time            `json:"created_at" db:"created_at"`
}
