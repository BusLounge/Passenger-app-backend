//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	connUrl := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"

	config, err := pgx.ParseConfig(connUrl)
	if err != nil {
		log.Fatalf("Parse config failed: %v", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	fmt.Println("Applying database migration for transport_bookings...")

	// Read SQL file
	sqlBytes, err := os.ReadFile("scripts/03_create_transport_bookings.sql")
	if err != nil {
		log.Fatalf("Failed to read SQL script: %v", err)
	}

	sqlString := string(sqlBytes)

	// Execute SQL Script
	_, err = conn.Exec(context.Background(), sqlString)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Table transport_bookings created successfully.")

	// Drop old columns from bookings and lounge_bookings
	dropQueries := []string{
		"ALTER TABLE bookings DROP COLUMN IF EXISTS transport_type;",
		"ALTER TABLE bookings DROP COLUMN IF EXISTS transport_time;",
		"ALTER TABLE lounge_bookings DROP COLUMN IF EXISTS transport_type;",
		"ALTER TABLE lounge_bookings DROP COLUMN IF EXISTS pickup_location;",
		"ALTER TABLE lounge_bookings DROP COLUMN IF EXISTS pickup_location_id;",
		"ALTER TABLE lounge_bookings DROP COLUMN IF EXISTS transport_cost;",
		"ALTER TABLE lounge_bookings DROP COLUMN IF EXISTS transport_time;",
	}

	for _, q := range dropQueries {
		_, err := conn.Exec(context.Background(), q)
		if err != nil {
			log.Printf("Warning: Query failed (might not exist): %v\nQuery: %s", err, q)
		} else {
			fmt.Printf("Executed: %s\n", q)
		}
	}

	fmt.Println("Database schema updated successfully!")
}
