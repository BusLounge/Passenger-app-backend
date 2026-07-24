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
	config, err := pgx.ParseConfig("postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres")
	if err != nil {
		log.Fatalf("ParseConfig failed: %v", err)
	}
	// Disable prepared statement cache to avoid 42P05 errors
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	futureDate := time.Now().Add(48 * time.Hour)
	futureDateStr := futureDate.Format("2006-01-02 15:04:05")

	fmt.Println("--- Force Sync: All Trips to Future ---")
	
	// Update all scheduled_trips that have bookings to be in the future
	res, err := conn.Exec(context.Background(), `
		UPDATE scheduled_trips
		SET departure_datetime = $1, status = 'scheduled'
		WHERE id IN (
			SELECT bb.scheduled_trip_id 
			FROM bus_bookings bb
		)
	`, futureDate)
	
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	fmt.Printf("Successfully updated %d scheduled trips to %s\n", res.RowsAffected(), futureDateStr)
	fmt.Println("\nPlease refresh your app. The countdown should now appear on all booking cards.")
}
