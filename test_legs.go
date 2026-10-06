package main

import (
	
	"fmt"
	

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/smarttransit/sms-auth-backend/internal/database"
)

func main() {
	db, err := sqlx.Connect("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres?sslmode=disable")
	if err != nil {
		fmt.Println("DB err:", err)
		return
	}
	repo := database.NewBookingIntentRepository(db)

	type DBIntent struct {
		ID uuid.UUID `db:"id"`
	}
	var intents []DBIntent
	err = db.Select(&intents, "SELECT id FROM booking_intents ORDER BY created_at DESC LIMIT 5")
	if err != nil {
		fmt.Println("Query err:", err)
		return
	}

	for _, dbIntent := range intents {
		intent, err := repo.GetIntentByID(dbIntent.ID)
		if err != nil {
			fmt.Println("GetIntentByID err:", err)
			continue
		}
		
		fmt.Printf("Intent ID: %s, Type: %s\n", intent.ID, intent.IntentType)
		
		bus := intent.GetBusIntent()
		fmt.Printf("  Has Bus Intent: %v\n", bus != nil)
		
		pre := intent.GetPreTripLoungeIntent()
		fmt.Printf("  Has Pre Lounge: %v\n", pre != nil)

		if len(intent.Legs) > 0 {
			fmt.Printf("  Legs count: %d\n", len(intent.Legs))
			for _, leg := range intent.Legs {
				fmt.Printf("    Leg Type: %s, LegIntent length: %d\n", leg.LegType, len(leg.LegIntent))
				if leg.LegType == "lounge_pre_outbound" {
					fmt.Printf("      Leg Intent Content: %s\n", string(leg.LegIntent))
				}
			}
		}
	}
}
