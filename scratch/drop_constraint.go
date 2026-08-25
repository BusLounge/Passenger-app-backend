package main

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec("ALTER TABLE booking_intents DROP CONSTRAINT IF EXISTS chk_intent_type_matches_payload")
	if err != nil {
		log.Printf("Could not drop constraint: %v", err)
	} else {
		log.Println("Constraint chk_intent_type_matches_payload dropped successfully!")
	}
}
