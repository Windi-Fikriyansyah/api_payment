package main

import (
	"fmt"
	"payment_service/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	db := config.ConnectDB()
	defer db.Close()

	rows, err := db.Query("SELECT id, token, project_id, amount, order_id, expired_at FROM payment_sessions ORDER BY id DESC LIMIT 10")
	if err != nil {
		fmt.Printf("Error querying: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("ID | Token | ProjectID | Amount | OrderID | ExpiredAt")
	fmt.Println("---------------------------------------------------------------")
	for rows.Next() {
		var id, projectID uint
		var token, orderID, expiredAt string
		var amount float64
		rows.Scan(&id, &token, &projectID, &amount, &orderID, &expiredAt)
		fmt.Printf("%d | %s | %d | %.2f | %s | %s\n", id, token, projectID, amount, orderID, expiredAt)
	}
}
