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

	_, err = db.Exec("ALTER TABLE bookings ADD COLUMN transport_type character varying;")
	if err != nil {
		fmt.Println("Error adding transport_type: ", err)
	} else {
		fmt.Println("Added transport_type to bookings")
	}

	_, err = db.Exec("ALTER TABLE bookings ADD COLUMN transport_time timestamp with time zone;")
	if err != nil {
		fmt.Println("Error adding transport_time: ", err)
	} else {
		fmt.Println("Added transport_time to bookings")
	}
}
