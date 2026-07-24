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

	fmt.Println("Checking all stops for all master routes...")
	rows, err := conn.Query(context.Background(), `
		SELECT mr.route_name, s.stop_name, s.stop_order, s.latitude, s.longitude, s.id
		FROM master_route_stops s
		JOIN master_routes mr ON s.master_route_id = mr.id
		ORDER BY mr.route_name, s.stop_order
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var routeName, stopName string
		var order int
		var lat, lng *float64
		var id string
		rows.Scan(&routeName, &stopName, &order, &lat, &lng, &id)
		latStr, lngStr := "NULL", "NULL"
		if lat != nil {
			latStr = fmt.Sprintf("%f", *lat)
		}
		if lng != nil {
			lngStr = fmt.Sprintf("%f", *lng)
		}
		fmt.Printf("Route: %s | Stop %d: %s | ID: %s | LAT/LNG: %s, %s\n", routeName, order, stopName, id, latStr, lngStr)
	}
}
