package models

import "time"

type AuditLog struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"project_id"`
	TransactionID uint      `json:"transaction_id"`
	BeforeBalance float64   `json:"before_balance"`
	AfterBalance  float64   `json:"after_balance"`
	Amount        float64   `json:"amount"`
	Type          string    `json:"type"`
	CreatedAt     time.Time `json:"created_at"`
}
