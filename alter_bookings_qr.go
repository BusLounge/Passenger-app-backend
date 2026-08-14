package main

import (
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fallback for local development
		dbURL = "postgres://postgres:postgres@localhost:5432/passenger_db?sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	// 1. Add columns to bookings
	_, err = db.Exec(`
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS qr_code_data VARCHAR UNIQUE;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS qr_generated_at TIMESTAMP WITH TIME ZONE;
	`)
	if err != nil {
		log.Fatalf("Failed to add columns: %v", err)
	}
	log.Println("Added qr_code_data and qr_generated_at to bookings table")

	// 2. Backfill from bus_bookings
	result, err := db.Exec(`
		UPDATE bookings b
		SET qr_code_data = bb.qr_code_data,
			qr_generated_at = bb.qr_generated_at
		FROM bus_bookings bb
		WHERE b.id = bb.booking_id
		AND b.qr_code_data IS NULL
		AND bb.qr_code_data IS NOT NULL
		AND bb.id = (
			SELECT id FROM bus_bookings WHERE booking_id = b.id LIMIT 1
		)
	`)
	if err != nil {
		log.Fatalf("Failed to backfill qr: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Successfully backfilled QR codes for %d existing bookings.\n", rowsAffected)
	log.Println("Migration completed successfully.")
}
