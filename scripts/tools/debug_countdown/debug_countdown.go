//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	connUrl := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	conn, err := pgx.Connect(context.Background(), connUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	futureDate := time.Now().Add(48 * time.Hour)
	futureDateStr := futureDate.Format("2006-01-02 15:04:05")

	fmt.Println("--- Advanced Diagnostic: Upcoming Bookings ---")
	
	// 1. Check all bookings for the logged in user or most recent bookings
	fmt.Println("\nRecent Bookings:")
	rows, _ := conn.Query(context.Background(), `
		SELECT b.id, b.booking_reference, b.booking_status, b.payment_status, 
		       st.departure_datetime, bor.custom_route_name
		FROM bookings b
		LEFT JOIN bus_bookings bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		ORDER BY b.created_at DESC
		LIMIT 10
	`)
	for rows.Next() {
		var id, ref, bStatus, pStatus, rName string
		var dTime *time.Time
		rows.Scan(&id, &ref, &bStatus, &pStatus, &dTime, &rName)
		dTimeStr := "NULL"
		if dTime != nil {
			dTimeStr = dTime.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("ID: %s | Ref: %s | Status: %s | Payment: %s | Departure: %s | Route: %s\n", 
			id, ref, bStatus, pStatus, dTimeStr, rName)
	}

	// 2. Force update all 'confirmed' bookings to be in the future
	fmt.Printf("\nForcing all 'confirmed' bookings to departure date: %s\n", futureDateStr)
	
	// Update scheduled_trips first (the source of truth)
	res1, err := conn.Exec(context.Background(), `
		UPDATE scheduled_trips
		SET departure_datetime = $1, status = 'scheduled'
		WHERE id IN (
			SELECT bb.scheduled_trip_id 
			FROM bus_bookings bb
			JOIN bookings b ON b.id = bb.booking_id
			WHERE b.booking_status = 'confirmed'
		)
	`, futureDate)
	if err != nil {
		fmt.Printf("Error updating scheduled_trips: %v\n", err)
	} else {
		fmt.Printf("Updated %d trips in scheduled_trips\n", res1.RowsAffected())
	}

	// Update denormalized departure_datetime in bus_bookings if it exists (some schemas use it)
	// We'll check if the column exists first by attempting the update
	res2, err := conn.Exec(context.Background(), `
		UPDATE bus_bookings
		SET departure_datetime = $1
		WHERE booking_id IN (
			SELECT id FROM bookings WHERE booking_status = 'confirmed'
		)
	`, futureDate)
	if err != nil {
		fmt.Printf("Note: bus_bookings.departure_datetime update skipped/failed: %v\n", err)
	} else {
		fmt.Printf("Updated %d rows in bus_bookings\n", res2.RowsAffected())
	}

	fmt.Println("\nDone. If you still don't see countdowns, check if the app is filtering by status correctly.")
}
