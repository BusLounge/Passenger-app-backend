package main
import (
	"fmt"
	"encoding/json"
	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
)
type TestLeg struct {
	LegIntent json.RawMessage `db:"leg_intent"`
}
func main() {
	db, err := sqlx.Connect("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres?sslmode=disable")
	if err != nil { panic(err) }
	
	// Just select one row from booking_intent_legs
	var t TestLeg
	err = db.Get(&t, "SELECT leg_intent FROM booking_intent_legs LIMIT 1")
	fmt.Printf("Error: %v\n", err)
	fmt.Printf("Data: %s\n", string(t.LegIntent))
}
