package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/smarttransit/sms-auth-backend/internal/config"
	"github.com/smarttransit/sms-auth-backend/internal/database"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	query := `
	CREATE TABLE IF NOT EXISTS public.call_logs (
		id uuid not null default extensions.uuid_generate_v4 (),
		trip_id uuid not null,
		caller_id uuid not null,
		receiver_id uuid not null,
		caller_role character varying(20) not null,
		receiver_role character varying(20) not null,
		channel_name character varying(255) not null,
		call_status character varying(50) not null default 'initiated'::character varying,
		started_at timestamp with time zone null default now(),
		ended_at timestamp with time zone null,
		duration_seconds integer null default 0,
		created_at timestamp with time zone null default now(),
		constraint call_logs_pkey primary key (id)
	) TABLESPACE pg_default;
	`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}

	fmt.Println("Successfully created call_logs table")
}
