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

	fmt.Println("Applying database migration for booking_intents...")

	// Read SQL file
	sqlBytes, err := os.ReadFile("scripts/04_add_transport_intents_to_booking_intents.sql")
	if err != nil {
		log.Fatalf("Failed to read SQL script: %v", err)
	}

	sqlString := string(sqlBytes)

	// Execute SQL Script
	_, err = conn.Exec(context.Background(), sqlString)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Added transport_intents column to booking_intents successfully.")
}
