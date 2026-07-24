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

	fmt.Println("Adding transport/pickup columns to lounge_bookings table...")

	queries := []string{
		"ALTER TABLE lounge_bookings ADD COLUMN IF NOT EXISTS transport_type VARCHAR(50) NULL",
		"ALTER TABLE lounge_bookings ADD COLUMN IF NOT EXISTS pickup_location VARCHAR(255) NULL",
		"ALTER TABLE lounge_bookings ADD COLUMN IF NOT EXISTS pickup_location_id UUID NULL",
		"ALTER TABLE lounge_bookings ADD COLUMN IF NOT EXISTS transport_cost NUMERIC(10, 2) DEFAULT 0.00 NOT NULL",
	}

	for _, q := range queries {
		_, err := conn.Exec(context.Background(), q)
		if err != nil {
			log.Fatalf("Query failed: %v\nQuery: %s", err, q)
		}
		fmt.Printf("Executed: %s\n", q)
	}

	fmt.Println("Database schema updated successfully!")
}
