package main

import (
	"fmt"
	"payment_service/internal/config"
	"payment_service/internal/repository"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	db := config.ConnectDB()
	defer db.Close()

	_ = repository.NewProjectRepository(db)
	
	// Since we don't have a GetAll, we'll use a raw query
	rows, err := db.Query("SELECT id, nama, slug, api_key, status, mode FROM projects")
	if err != nil {
		fmt.Printf("Error querying: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("ID | Nama | Slug | API Key | Status | Mode")
	fmt.Println("-------------------------------------------------------------------")
	for rows.Next() {
		var id uint
		var nama, slug, apiKey, status, mode string
		rows.Scan(&id, &nama, &slug, &apiKey, &status, &mode)
		fmt.Printf("%d | %s | %s | %s | %s | %s\n", id, nama, slug, apiKey, status, mode)
	}
}
