package models

import "time"

type PaymentSession struct {
	ID          uint      `json:"id"`
	Token       string    `json:"token"`
	ProjectID   uint      `json:"project_id"`
	Amount      float64   `json:"amount"`
	OrderID     string    `json:"order_id"`
	RedirectURL string    `json:"redirect_url"`
	ExpiredAt   time.Time `json:"expired_at"`
	CreatedAt      time.Time `json:"created_at"`
	BuyerName      string    `json:"buyer_name"`
	WhatsappNumber string    `json:"whatsapp_number"`
}

type CheckoutSessionRequest struct {
	Amount      float64 `json:"amount"`
	OrderID     string  `json:"order_id"`
	RedirectURL string  `json:"redirect_url"`
}

type CheckoutSessionResponse struct {
	PaymentURL string `json:"payment_url"`
	OrderID    string `json:"order_id"`
	Amount     float64 `json:"amount"`
}
