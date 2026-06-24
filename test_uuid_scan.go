package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type TestStruct struct {
	MasterBookingID *uuid.UUID `db:"master_booking_id"`
}

func main() {
	dbURL := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	db, err := sqlx.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var tests []TestStruct
	err = db.Select(&tests, "SELECT master_booking_id FROM lounge_bookings LIMIT 1")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Success:", len(tests), tests[0].MasterBookingID)
	}
}
