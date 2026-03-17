package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func main() {
	// Ganti dengan data project Anda
	slug := "windi"
	amount := "50000"
	orderID := "ORD-001"
	redirect := "https://tokoanda.com/success"
	apiKey := "SB_H7sY4Kp8ZxQ2bLm9JcT5WnV6dE3rFa1UoG0tXi"

	stringToSign := fmt.Sprintf("%s:%s:%s:%s", slug, amount, orderID, redirect)

	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	fmt.Println("--- Signature Generator ---")
	fmt.Printf("Slug: %s\n", slug)
	fmt.Printf("Amount: %s\n", amount)
	fmt.Printf("OrderID: %s\n", orderID)
	fmt.Printf("Redirect: %s\n", redirect)
	fmt.Printf("API Key: %s\n", apiKey)
	fmt.Println("---------------------------")
	fmt.Printf("Generated Signature: %s\n", signature)
	fmt.Printf("\nTest URL:\nhttp://localhost:3005/pay/%s/%s?order_id=%s&redirect=%s&signature=%s\n",
		slug, amount, orderID, redirect, signature)
}
