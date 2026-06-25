//go:build ignore

package main

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	dbUrl := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	db, err := sqlx.Connect("postgres", dbUrl)
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}
	defer db.Close()

	userID := "a0fe65c2-34a5-4754-a81c-e991884cfcb7"
	fmt.Printf("Searching for bookings/trips associated with user: %s\n", userID)

	// Query 1: Find trip seats booked by this user via bookings
	type TripSeatResult struct {
		ScheduledTripID  string `db:"scheduled_trip_id"`
		BookingReference string `db:"booking_reference"`
		BookingStatus    string `db:"booking_status"`
		PaymentStatus    string `db:"payment_status"`
	}
	var results []TripSeatResult
	err = db.Select(&results, `
		SELECT DISTINCT
			ts.scheduled_trip_id,
			b.booking_reference,
			b.booking_status,
			b.payment_status
		FROM bookings b
		JOIN bus_bookings bb ON bb.booking_id = b.id
		JOIN bus_booking_seats bbs ON bbs.bus_booking_id = bb.id
		JOIN trip_seats ts ON ts.bus_booking_seat_id = bbs.id
		WHERE b.user_id = $1
	`, userID)
	if err != nil {
		fmt.Printf("Error running Query 1: %v\n", err)
	} else {
		fmt.Println("\n--- Query 1: Bookings by User via bus_bookings/trip_seats ---")
		if len(results) == 0 {
			fmt.Println("No bookings found through bookings->bus_bookings->bus_booking_seats->trip_seats path.")
		}
		for _, r := range results {
			fmt.Printf("Scheduled Trip ID: %s | Booking Ref: %s | Status: %s | Payment: %s\n", r.ScheduledTripID, r.BookingReference, r.BookingStatus, r.PaymentStatus)
		}
	}

	// Query 2: Let's also check if user has general bookings, and what trips are linked
	type GeneralBookingResult struct {
		ID               string  `db:"id"`
		BookingReference string  `db:"booking_reference"`
		BookingStatus    string  `db:"booking_status"`
		PaymentStatus    string  `db:"payment_status"`
		ScheduledTripID  *string `db:"scheduled_trip_id"`
	}
	var results2 []GeneralBookingResult
	err = db.Select(&results2, `
		SELECT 
			b.id,
			b.booking_reference,
			b.booking_status,
			b.payment_status,
			bb.scheduled_trip_id
		FROM bookings b
		LEFT JOIN bus_bookings bb ON bb.booking_id = b.id
		WHERE b.user_id = $1
	`, userID)
	if err != nil {
		fmt.Printf("Error running Query 2: %v\n", err)
	} else {
		fmt.Println("\n--- Query 2: All Bookings for User ---")
		if len(results2) == 0 {
			fmt.Println("No records found in bookings table for this user.")
		}
		for _, r := range results2 {
			tVal := "NULL"
			if r.ScheduledTripID != nil {
				tVal = *r.ScheduledTripID
			}
			fmt.Printf("Booking ID: %s | Ref: %s | Status: %s | Payment: %s | Trip ID: %s\n", r.ID, r.BookingReference, r.BookingStatus, r.PaymentStatus, tVal)
		}
	}

	// Query 3: Let's list some recent scheduled trips in the system
	type RecentTripResult struct {
		ID          string `db:"id"`
		TripDate    string `db:"trip_date"`
		Status      string `db:"status"`
		TotalSeats  int    `db:"total_seats"`
		BookedSeats int    `db:"booked_seats"`
	}
	var results3 []RecentTripResult
	err = db.Select(&results3, `
		SELECT 
			st.id,
			st.trip_date::text as trip_date,
			st.status,
			(SELECT COUNT(*) FROM trip_seats WHERE scheduled_trip_id = st.id) as total_seats,
			(SELECT COUNT(*) FROM trip_seats WHERE scheduled_trip_id = st.id AND status = 'booked') as booked_seats
		FROM scheduled_trips st
		ORDER BY st.created_at DESC
		LIMIT 5
	`)
	if err != nil {
		fmt.Printf("Error running Query 3: %v\n", err)
	} else {
		fmt.Println("\n--- Query 3: Recent Scheduled Trips ---")
		for _, r := range results3 {
			fmt.Printf("Trip ID: %s | Date: %s | Status: %s | Seats: %d/%d (booked/total)\n", r.ID, r.TripDate, r.Status, r.BookedSeats, r.TotalSeats)
		}
	}
}
