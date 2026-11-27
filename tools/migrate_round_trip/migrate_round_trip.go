//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

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

	fmt.Println("Applying database migration for round trip...")

	// Add new columns to booking_intents
	alterQueries := []string{
		"ALTER TABLE booking_intents ADD COLUMN IF NOT EXISTS return_bus_intent JSONB;",
		"ALTER TABLE booking_intents ADD COLUMN IF NOT EXISTS return_bus_booking_id UUID REFERENCES bus_bookings(id);",
	}

	for _, q := range alterQueries {
		_, err := conn.Exec(context.Background(), q)
		if err != nil {
			log.Printf("Error running query: %v\nQuery: %s", err, q)
		} else {
			fmt.Printf("Executed: %s\n", q)
		}
	}

	fmt.Println("Database schema updated successfully!")
}
