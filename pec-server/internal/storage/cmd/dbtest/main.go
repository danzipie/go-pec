package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		os.Getenv("PGHOST"),
		os.Getenv("PGPORT"),
		os.Getenv("PGUSER"),
		os.Getenv("PGPASSWORD"),
		os.Getenv("PGDATABASE"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected to PostgreSQL!")
}
