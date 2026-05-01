package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	dsn := "host=localhost port=5432 user=postgres password=1234 dbname=qrapp sslmode=disable"
	fmt.Println("DSN:", dsn)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Open error:", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Ping error:", err)
	}

	fmt.Println("Successfully connected to database!")
}
