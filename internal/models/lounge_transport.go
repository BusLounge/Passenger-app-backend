package models

import "github.com/google/uuid"

// LoungeTransportOption is one pickup location with per-vehicle prices for a lounge.
type LoungeTransportOption struct {
	LocationID        uuid.UUID `json:"location_id" db:"location_id"`
	Location          string    `json:"location" db:"location"`
	Latitude          float64   `json:"latitude" db:"latitude"`
	Longitude         float64   `json:"longitude" db:"longitude"`
	EstDurationMins   *int      `json:"est_duration_minutes,omitempty"`
	DistanceKm        *float64  `json:"distance_km,omitempty"`
	ThreeWheelerPrice float64   `json:"three_wheeler_price" db:"three_wheeler_price"`
	CarPrice          float64   `json:"car_price" db:"car_price"`
	VanPrice          float64   `json:"van_price" db:"van_price"`
}
