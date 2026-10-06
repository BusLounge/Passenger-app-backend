package main

import (
	"database/sql"
	"fmt"
	"log"
	
	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	query := "SELECT id, created_at, leg_intent::text FROM booking_intent_legs WHERE leg_type='lounge_pre_outbound' OR leg_type='lounge_post_outbound' OR leg_type='transit_lounge' ORDER BY created_at DESC LIMIT 5;"
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var ca string
		var intent string
		if err := rows.Scan(&id, &ca, &intent); err == nil {
			fmt.Printf("ID: %s\n%s\n\n", id, intent)
		}
	}
}
