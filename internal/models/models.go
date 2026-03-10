package models

import "time"

type TransactionRequest struct {
	Project string  `json:"project"`
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
	APIKey  string  `json:"api_key"`
}

type PaymentResponse struct {
	Payment PaymentDetail `json:"payment"`
}

type PaymentDetail struct {
	Project       string    `json:"project"`
	OrderID       string    `json:"order_id"`
	Amount        float64   `json:"amount"`
	Fee           float64   `json:"fee"`
	TotalPayment  float64   `json:"total_payment"`
	PaymentMethod string    `json:"payment_method"`
	PaymentNumber string    `json:"payment_number"`
	Reference     string    `json:"reference"`
	ExpiredAt     time.Time `json:"expired_at"`
}

type TransactionDetailResponse struct {
	Transaction TransactionDetail `json:"transaction"`
}

type TransactionDetail struct {
	Amount        float64   `json:"amount"`
	Fee           float64   `json:"fee"`
	TotalPayment  float64   `json:"total_payment"`
	OrderID       string    `json:"order_id"`
	Project       string    `json:"project"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

type PaymentMethod struct {
	ID         uint    `json:"id"`
	Code       string  `json:"code"`
	DuitkuCode string  `json:"duitku_code"`
	Name       string  `json:"name"`
	ImageURL   string  `json:"image_url"`
	FeeFlat    float64 `json:"fee_flat"`
	FeePercent float64 `json:"fee_percent"`
	IsActive   bool    `json:"is_active"`
}

type PaymentMethodItem struct {
	PaymentMethod string  `json:"payment_method"`
	PaymentName   string  `json:"payment_name"`
	PaymentImage  string  `json:"payment_image"`
	TotalFee      float64 `json:"total_fee"`
}

type PaymentMethodResponse struct {
	Methods []PaymentMethodItem `json:"methods"`
}

type WebhookPayload struct {
	Amount        float64   `json:"amount"`
	Fee           float64   `json:"fee"`
	NetAmount     float64   `json:"net_amount"`
	OrderID       string    `json:"order_id"`
	Project       string    `json:"project"`
	Status        string    `json:"status"`
	PaymentMethod string    `json:"payment_method"`
	CompletedAt   time.Time `json:"completed_at"`
}
