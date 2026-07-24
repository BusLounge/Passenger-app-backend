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

	_, err = conn.Exec(context.Background(), "ALTER TYPE lounge_booking_type ADD VALUE IF NOT EXISTS 'transit';")
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		fmt.Println("Added transit to lounge_booking_type")
	}
}