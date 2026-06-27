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

	// unused
	toName := "Kandy"

	// test destination stops
	rows, err := conn.Query(context.Background(), `
		SELECT DISTINCT mrs.id, mrs.stop_name, mrs.stop_order, mrs.master_route_id
		FROM master_route_stops mrs
		WHERE LOWER(mrs.stop_name) LIKE LOWER('%' || $1 || '%')
	`, toName)
	var count int
	for rows.Next() {
		count++
	}
	fmt.Printf("Destination stops count: %d\n", count)

	err = conn.QueryRow(context.Background(), `
		WITH 
		destination_stops AS (
			SELECT DISTINCT mrs.id AS dest_id, mrs.stop_name AS dest_name, mrs.stop_order AS dest_order, mrs.master_route_id
			FROM master_route_stops mrs WHERE LOWER(mrs.stop_name) LIKE LOWER('%' || $1 || '%')
		),
		matching_routes AS (
			SELECT mr.id AS master_route_id, mr.route_name
			FROM master_routes mr
			JOIN destination_stops ds ON ds.master_route_id = mr.id
			WHERE mr.is_active = true
		),
		candidate_stops AS (
			SELECT DISTINCT mrs.id, mrs.stop_name, mrs.stop_order, mrs.latitude, mrs.longitude, mrs.is_major_stop, mr.master_route_id
			FROM master_route_stops mrs
			JOIN matching_routes mr ON mr.master_route_id = mrs.master_route_id
			WHERE mrs.latitude IS NOT NULL AND mrs.longitude IS NOT NULL
		)
		SELECT count(*)
		FROM candidate_stops cs
		JOIN destination_stops ds ON ds.master_route_id = cs.master_route_id
		JOIN matching_routes mr ON mr.master_route_id = cs.master_route_id
		WHERE cs.stop_order < ds.dest_order
	`, toName).Scan(&count)
	fmt.Printf("Candidate stops before distance: %d\n", count)

	fmt.Println("Done")
}
