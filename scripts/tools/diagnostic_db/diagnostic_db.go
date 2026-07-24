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

	// toName := "Kandy"

	fmt.Println("--- Diagnostic: Available Master Routes ---")
	rows, _ := conn.Query(context.Background(), "SELECT id, route_name FROM master_routes WHERE is_active = true")
	for rows.Next() {
		var id, name string
		rows.Scan(&id, &name)
		fmt.Printf("Route: %s | ID: %s\n", name, id)
	}

	fmt.Println("\n--- Diagnostic: Kandy Stops ---")
	rows, _ = conn.Query(context.Background(), "SELECT id, stop_name, master_route_id, stop_order FROM master_route_stops WHERE LOWER(stop_name) LIKE '%kandy%'")
	for rows.Next() {
		var id, name, rid string
		var order int
		rows.Scan(&id, &name, &rid, &order)
		fmt.Printf("Stop: %s | Order: %d | RouteID: %s\n", name, order, rid)
	}

	fmt.Println("\n--- Diagnostic: All Stops for Colombo-Kandy Route ---")
	// Using a LIKE to find it if UUID varies
	rows, _ = conn.Query(context.Background(), `
		SELECT stop_name, stop_order 
		FROM master_route_stops 
		WHERE master_route_id IN (SELECT id FROM master_routes WHERE route_name LIKE '%Colombo%Kandy%')
		ORDER BY stop_order
	`)
	for rows.Next() {
		var name string
		var order int
		rows.Scan(&name, &order)
		fmt.Printf("%d. %s\n", order, name)
	}

	fmt.Println("\nDone")
}
