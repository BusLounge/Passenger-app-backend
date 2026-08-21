package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres?sslmode=require"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userID := "1f79307d-da70-4964-b3e8-c426d512f332"
	query := `
		SELECT 
			b.id, b.booking_reference, b.booking_type,
			b.booking_status
		FROM bookings b
		LEFT JOIN bus_bookings bb ON bb.booking_id = b.id
		LEFT JOIN scheduled_trips st ON st.id = bb.scheduled_trip_id
		LEFT JOIN bus_owner_routes bor ON bor.id = st.bus_owner_route_id
		LEFT JOIN lounges lf ON b.search_from_lounge = lf.id::text
		LEFT JOIN lounges lt ON b.search_to_lounge = lt.id::text
		WHERE b.user_id = $1
		  AND b.booking_status NOT IN ('cancelled', 'completed', 'partial_cancel')
		  AND (bb.status IS NULL OR bb.status NOT IN ('cancelled', 'completed', 'no_show'))
		  AND (
			  COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			  ) IS NULL
			  OR
			  COALESCE(
				st.departure_datetime,
				(SELECT MIN(scheduled_arrival) FROM lounge_bookings lb WHERE lb.master_booking_id = b.id)
			  ) > NOW() AT TIME ZONE 'Asia/Colombo' - INTERVAL '24 hours'
		  )
		ORDER BY b.created_at ASC
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, ref, bType, status sql.NullString
		if err := rows.Scan(&id, &ref, &bType, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Found: %s | %s | %s | %s\n", id.String, ref.String, bType.String, status.String)
		count++
	}
	fmt.Printf("Total returned: %d\n", count)
}
