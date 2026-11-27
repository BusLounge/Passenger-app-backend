package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Connecting to DB...")
	db, err := sql.Open("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	queries := []string{
		"ALTER TABLE public.lounge_booking_pre_orders DROP CONSTRAINT IF EXISTS lounge_booking_pre_orders_booking_fkey;",
		"ALTER TABLE public.lounge_booking_pre_orders DROP CONSTRAINT IF EXISTS lounge_booking_pre_orders_lounge_booking_id_fkey;",
		"DELETE FROM public.lounge_booking_pre_orders WHERE lounge_booking_id NOT IN (SELECT id FROM public.lounge_bookings);",
		"ALTER TABLE public.lounge_booking_pre_orders ADD CONSTRAINT lounge_booking_pre_orders_lounge_booking_id_fkey FOREIGN KEY (lounge_booking_id) REFERENCES public.lounge_bookings(id) ON DELETE CASCADE;",
	}

	for _, q := range queries {
		fmt.Println("Executing:", q)
		_, err := db.Exec(q)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			fmt.Println("Success")
		}
	}
}
