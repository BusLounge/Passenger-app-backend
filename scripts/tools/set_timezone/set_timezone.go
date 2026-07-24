//go:build ignore

package main

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	dbURL := "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres"
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("ALTER DATABASE postgres SET timezone TO 'Asia/Colombo';")
	if err != nil {
		fmt.Println("Error setting timezone: ", err)
	} else {
		fmt.Println("Timezone set to Asia/Colombo successfully!")
	}
}
