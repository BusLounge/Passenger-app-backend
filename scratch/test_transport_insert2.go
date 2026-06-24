package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := "postgres://postgres:2230101sS@localhost:5432/postgres?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	userID := "d33de146-281b-410a-8bf8-024c08cd4851" // A UUID

	query := `
		INSERT INTO transport_bookings (
			user_id, vehicle_type, vehicle_quantity, transport_price, transport_date, transport_time,
			status, payment_status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id`

	var newID string
	err = pool.QueryRow(context.Background(), query,
		userID, "car", 1, 1500.00, time.Now(), time.Now(),
		"pending", "paid",
	).Scan(&newID)

	if err != nil {
		fmt.Println("Error inserting:", err)
	} else {
		fmt.Println("Success! ID:", newID)
	}
}

