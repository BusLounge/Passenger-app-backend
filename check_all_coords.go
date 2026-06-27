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

	fmt.Println("Checking all stops with coordinates...")
	rows, _ := conn.Query(context.Background(), `
		SELECT stop_name, latitude, longitude
		FROM master_route_stops
		WHERE latitude IS NOT NULL AND longitude IS NOT NULL
		LIMIT 20
	`)
	for rows.Next() {
		var name string
		var lat, lng float64
		rows.Scan(&name, &lat, &lng)
		fmt.Printf("Stop: %s | LAT/LNG: %f, %f\n", name, lat, lng)
	}
}
