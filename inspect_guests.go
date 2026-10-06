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

	query2 := "SELECT conname, pg_get_constraintdef(c.oid) FROM pg_constraint c WHERE conname LIKE '%lounge_booking_guests%';"
	rows2, err := db.Query(query2)
	if err != nil {
		log.Fatal(err)
	}
	defer rows2.Close()
	fmt.Println("CONSTRAINTS:")
	for rows2.Next() {
		var name, def string
		if err := rows2.Scan(&name, &def); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %s\n", name, def)
	}
}
