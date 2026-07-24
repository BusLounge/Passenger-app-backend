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

	// Data from Google Maps / Public data
	locations := map[string][]float64{
		"Colombo Fort": {6.9344, 79.8510},
		"Kadawatha":    {7.0016, 79.9547},
		"Nittambuwa":   {7.1435, 80.0950},
		"Kegalle":      {7.2514, 80.3465},
		"Mawanella":    {7.2544, 80.4444},
		"Peradeniya":   {7.2706, 80.5976},
		"Kandy":        {7.2906, 80.6337},
	}

	for name, coords := range locations {
		fmt.Printf("Updating %s...\n", name)
		_, err := conn.Exec(context.Background(), `
			UPDATE master_route_stops 
			SET latitude = $1, longitude = $2 
			WHERE stop_name = $3
		`, coords[0], coords[1], name)
		if err != nil {
			fmt.Printf("Failed to update %s: %v\n", name, err)
		}
	}

	fmt.Println("Coordinates updated successfully for searching!")
}
