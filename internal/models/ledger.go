package models

import "time"

type Ledger struct {
	ID            uint      `json:"id"`
	ProjectID     uint      `json:"project_id"`
	TransactionID uint      `json:"transaction_id"`
	Amount        float64   `json:"amount"`
	Type          string    `json:"type"` // credit (masuk), debit (keluar)
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
}
