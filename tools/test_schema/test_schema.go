package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sqlx.Connect("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}

	type Col struct {
		Conname string `db:"conname"`
		Constraint string `db:"pg_get_constraintdef"`
		IsNullable string `db:"is_nullable"`
	}
	var cols []Col
	err = db.Select(&cols, "SELECT conname, pg_get_constraintdef(c.oid) FROM pg_constraint c JOIN pg_class t ON c.conrelid = t.oid WHERE t.relname = 'lounge_bookings';")
	if err != nil {
		panic(err)
	}

	b, _ := json.MarshalIndent(cols, "", "  ")
	err = os.WriteFile("schema.json", b, 0644)
	if err != nil {
		fmt.Println(err)
	}
}
