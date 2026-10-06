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

query := 
SELECT
    tc.table_name, 
    kcu.column_name, 
    ccu.table_name AS foreign_table_name,
    ccu.column_name AS foreign_column_name 
FROM 
    information_schema.table_constraints AS tc 
    JOIN information_schema.key_column_usage AS kcu
      ON tc.constraint_name = kcu.constraint_name
      AND tc.table_schema = kcu.table_schema
    JOIN information_schema.constraint_column_usage AS ccu
      ON ccu.constraint_name = tc.constraint_name
      AND ccu.table_schema = tc.table_schema
WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name='lounge_booking_pre_orders';

rows, err := db.Query(query)
if err != nil {
log.Fatal(err)
}
defer rows.Close()
for rows.Next() {
var t, c, ft, fc string
if err := rows.Scan(&t, &c, &ft, &fc); err != nil {
log.Fatal(err)
}
fmt.Printf("%s.%s -> %s.%s\n", t, c, ft, fc)
}
}
