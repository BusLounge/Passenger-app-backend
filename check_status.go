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

	refs := []string{"BL-20260330-731939", "BL-20260323-D2482F", "BL-20260319-F24734"}
	
	for _, ref := range refs {
		var id, bStatus string
		err := conn.QueryRow(context.Background(), 
			"SELECT id, booking_status FROM bookings WHERE booking_reference = $1", ref).Scan(&id, &bStatus)
		if err != nil {
			fmt.Printf("Ref: %s | Error: %v\n", ref, err)
			continue
		}
		fmt.Printf("Ref: %s | ID: %s | Status: %s\n", ref, id, bStatus)
	}
}
