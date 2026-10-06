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

query := "SELECT table_name, column_name, data_type FROM information_schema.columns WHERE table_name = 'lounge_booking_pre_orders';"
rows, err := db.Query(query)
if err != nil {
log.Fatal(err)
}
defer rows.Close()
fmt.Println("COLUMNS:")
for rows.Next() {
var t, c, d string
if err := rows.Scan(&t, &c, &d); err != nil {
log.Fatal(err)
}
fmt.Printf("%s.%s %s\n", t, c, d)
}

query2 := "SELECT conname, pg_get_constraintdef(c.oid) FROM pg_constraint c WHERE conname LIKE '%lounge_booking_pre_orders%';"
rows2, err := db.Query(query2)
if err != nil {
log.Fatal(err)
}
defer rows2.Close()
fmt.Println("CONSTRAINTS:")
for rows2.Next() {
var name, def string
if err := rows2.Scan(&name, &def); err != nil {
log.Fatal(err)
}
fmt.Printf("%s: %s\n", name, def)
}
}
