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
	conn, err := pgx.Connect(context.Background(), connUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	fmt.Println("--- Diagnostic: Available Lounges ---")
	rows, _ := conn.Query(context.Background(), "SELECT id, lounge_name, latitude, longitude FROM lounges")
	for rows.Next() {
		var id, name string
		var lat, lng float64
		rows.Scan(&id, &name, &lat, &lng)
		fmt.Printf("Lounge: %s | ID: %s | Lat: %f | Lng: %f\n", name, id, lat, lng)
	}

	fmt.Println("\nDone")
}
