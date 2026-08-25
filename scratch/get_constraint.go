package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:postgres@localhost:5432/passenger_db_local?sslmode=disable"
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var definition string
	err = db.QueryRow(`
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		WHERE t.relname = 'booking_intents' AND c.conname = 'chk_intent_type_matches_payload'
	`).Scan(&definition)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("CONSTRAINT:", definition)
}
