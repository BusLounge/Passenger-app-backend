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
	
	// Enable simple protocol for Supabase pooler
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	
	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	fmt.Println("Listing columns for booking_intents...")
	rows, err := conn.Query(context.Background(), `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'lounge_bookings'
		ORDER BY ordinal_position
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, dtype string
		if err := rows.Scan(&name, &dtype); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s (%s)\n", name, dtype)
	}
}
