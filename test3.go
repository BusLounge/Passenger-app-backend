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
	if err != nil { panic(err) }
	
	repo := database.NewBookingIntentRepository(db)
	
	type IDStruct struct {
		ID string `db:"id"`
	}
	var ids []IDStruct
	err = db.Select(&ids, "SELECT DISTINCT booking_intent_id AS id FROM booking_intent_legs WHERE leg_type LIKE '%lounge%' ORDER BY booking_intent_id DESC LIMIT 10")
	if err != nil { panic(err) }

	for _, idObj := range ids {
		intent, err := repo.GetIntentByID(uuid.MustParse(idObj.ID))
		if err != nil {
			fmt.Println("Error:", idObj.ID, err)
			continue
		}
		
		fmt.Printf("ID: %s, Type: %s, Legs: %d\n", intent.ID, intent.IntentType, len(intent.Legs))
		for _, leg := range intent.Legs {
			fmt.Printf("  Leg: %s\n", leg.LegType)
		}
	}
}
