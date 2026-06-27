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

	futureDate := time.Now().Add(48 * time.Hour).Format("2006-01-02 15:04:05")

	fmt.Println("Updating all 'confirmed' bookings to be in the future...")
	res, err := conn.Exec(context.Background(), `
		UPDATE bus_bookings bb
		SET departure_datetime = $1
		FROM master_bookings mb
		WHERE mb.id = bb.booking_id
		  AND mb.booking_status = 'confirmed'
	`, futureDate)

	if err != nil {
		log.Fatalf("Failed to update bookings: %v", err)
	}

	fmt.Printf("Updated %d bookings to future date: %s\n", res.RowsAffected(), futureDate)
}
