package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'transactions'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	f, _ := os.Create("db_report.txt")
	defer f.Close()
	fmt.Fprintln(f, "Columns in transactions table:")
	for rows.Next() {
		var name, dtype string
		if err := rows.Scan(&name, &dtype); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(f, "COL: %s | TYPE: %s\n", name, dtype)
	}
}
