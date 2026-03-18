package models

import "time"

type Transaction struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"project_id"`
	OrderID       string    `json:"order_id"`
	GatewayOrderID string    `json:"gateway_order_id"`
	Reference     string    `json:"reference"`
	Amount        float64   `json:"amount"`
	Fee           float64   `json:"fee"`
	TotalPayment  float64   `json:"total_payment"`
	Status        string    `json:"status"` // pending, success, failed, expired
	Mode          string    `json:"mode"`   // sandbox, production
	PaymentMethod string    `json:"payment_method"`
	PaymentNumber string    `json:"payment_number"`
	Jenis         string    `json:"jenis"`
	ExpiredAt     time.Time `json:"expired_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	BuyerName      string    `json:"buyer_name"`
	WhatsappNumber string    `json:"whatsapp_number"`
}
