//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	connUrl := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	conn, err := pgx.Connect(context.Background(), connUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	fromName := "Mirigama"
	toName := "Kandy"

	// 1. Coordinates check for Mirigama
	var lat, lng float64
	queryCoords := `
		SELECT latitude, longitude
		FROM master_route_stops
		WHERE LOWER(stop_name) LIKE LOWER('%' || $1 || '%')
		   OR LOWER($1) LIKE LOWER('%' || stop_name || '%')
		ORDER BY is_major_stop DESC LIMIT 1
	`
	err = conn.QueryRow(context.Background(), queryCoords, strings.TrimSpace(fromName)).Scan(&lat, &lng)
	if err != nil {
		fmt.Printf("Mirigama Coords Check: FAILED (%v)\n", err)
	} else {
		fmt.Printf("Mirigama Coords: %f, %f\n", lat, lng)
	}

	// 2. Destination stops for Kandy
	fmt.Println("\n--- Destination stops for 'Kandy' ---")
	rows, _ := conn.Query(context.Background(), `
		SELECT id, stop_name, stop_order, master_route_id
		FROM master_route_stops
		WHERE LOWER(stop_name) LIKE LOWER('%' || $1 || '%')
	`, toName)
	for rows.Next() {
		var id, name, rid string
		var order int
		rows.Scan(&id, &name, &order, &rid)
		fmt.Printf("ID: %s | Stop: %s | Order: %d | RouteID: %s\n", id, name, order, rid)
	}

	// 3. FULL QUERY TEST (Manual run of the logic in FindInterceptStopPair)
	fmt.Println("\n--- Full Intercept Query Test ---")
	fullQuery := `
		WITH
		-- Step 1: find destination stop on any active route
		destination_stops AS (
			SELECT DISTINCT
			       mrs.id       AS dest_id,
			       mrs.stop_name AS dest_name,
			       mrs.stop_order AS dest_order,
			       mrs.master_route_id
			FROM   master_route_stops mrs
			WHERE  LOWER(mrs.stop_name) LIKE LOWER('%' || $1 || '%')
		),

		-- Step 2: find routes that contain this destination stop
		matching_routes AS (
			SELECT mr.id          AS master_route_id,
			       mr.route_name
			FROM   master_routes mr
			JOIN   destination_stops ds ON ds.master_route_id = mr.id
			WHERE  mr.is_active = true
		),

		-- Step 3: Candidate boarding stops on these matching routes
		candidate_stops AS (
			SELECT DISTINCT
			       mrs.id,
			       mrs.stop_name,
			       mrs.stop_order,
			       mrs.latitude,
			       mrs.longitude,
			       mrs.is_major_stop,
			       mr.master_route_id
			FROM   master_route_stops mrs
			JOIN   matching_routes mr ON mr.master_route_id = mrs.master_route_id
			WHERE  mrs.latitude  IS NOT NULL
			  AND  mrs.longitude IS NOT NULL
		)

		-- Step 4: score candidate stops by distance to user location
		SELECT
			cs.id                                                                AS from_id,
			cs.stop_name                                                         AS from_name,
			ds.dest_id                                                           AS to_id,
			ds.dest_name                                                         AS to_name,
			cs.master_route_id                                                   AS route_id,
			mr.route_name,
			-- Approximate Euclidean distance (fine for nearby-stop detection)
			SQRT(
			  POWER(cs.latitude  - $2,  2) +
			  POWER(cs.longitude - $3, 2)
			)                                                                    AS distance,
			-- Convert degree-distance to km (rough: 1° ≈ 111 km)
			SQRT(
			  POWER((cs.latitude  - $2)  * 111.0, 2) +
			  POWER((cs.longitude - $3) * 111.0 * 0.9997 /* COS(RADIANS(7.1)) - roughly Sri Lanka */, 2)
			)                                                                    AS distance_km
		FROM   candidate_stops cs
		JOIN   destination_stops ds  ON ds.master_route_id = cs.master_route_id
		JOIN   matching_routes   mr  ON mr.master_route_id = cs.master_route_id
		-- Ensure boarding stop comes before destination stop on the route
		WHERE  cs.stop_order < ds.dest_order
		ORDER  BY distance ASC
		LIMIT  5
	`
	// Fallback coordinates if DB lookup failed
	if lat == 0 { lat = 7.2435; lng = 80.1383 } // Approx Mirigama
	
	rows, err = conn.Query(context.Background(), fullQuery, toName, lat, lng)
	if err != nil {
		fmt.Printf("Query Error: %v\n", err)
	} else {
		for rows.Next() {
			var fromId, fromName, toId, toName, rid, rname string
			var dist, distKm float64
			rows.Scan(&fromId, &fromName, &toId, &toName, &rid, &rname, &dist, &distKm)
			fmt.Printf("JOIN AT: %s -> %s | Route: %s | Distance: %.2f km\n", fromName, toName, rname, distKm)
		}
	}

	fmt.Println("\nDone")
}
