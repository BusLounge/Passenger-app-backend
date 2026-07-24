//go:build ignore

package main

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("ALTER TABLE bookings ADD COLUMN search_from_lounge character varying;")
	if err != nil {
		fmt.Println("Error adding search_from_lounge: ", err)
	} else {
		fmt.Println("Added search_from_lounge to bookings")
	}

	_, err = db.Exec("ALTER TABLE bookings ADD COLUMN search_to_lounge character varying;")
	if err != nil {
		fmt.Println("Error adding search_to_lounge: ", err)
	} else {
		fmt.Println("Added search_to_lounge to bookings")
	}
}
