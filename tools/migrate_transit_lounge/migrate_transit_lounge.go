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

	fmt.Println("Adding missing columns to booking_intents...")
	
	queries := []string{
		"ALTER TABLE booking_intents ADD COLUMN IF NOT EXISTS transit_lounge_intent jsonb",
		"ALTER TABLE booking_intents ADD COLUMN IF NOT EXISTS transit_lounge_fare numeric DEFAULT 0",
		"ALTER TABLE booking_intents ADD COLUMN IF NOT EXISTS transit_lounge_booking_id uuid",
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
