package main

import (
"database/sql"
"fmt"
"log"

_ "github.com/lib/pq"
)

func main() {
db, err := sql.Open("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:6543/postgres")
if err != nil {
log.Fatal(err)
}

query := "SELECT conname, pg_get_constraintdef(c.oid) FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace WHERE conname = 'lounge_booking_pre_orders_booking_fkey';"
rows, err := db.Query(query)
if err != nil {
log.Fatal(err)
}
defer rows.Close()
for rows.Next() {
var name, def string
if err := rows.Scan(&name, &def); err != nil {
log.Fatal(err)
}
fmt.Printf("%s: %s\n", name, def)
}
}
