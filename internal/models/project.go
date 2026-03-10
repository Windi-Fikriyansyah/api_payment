package models

import "time"

type Project struct {
	ID             uint      `json:"id"`
	Nama           string    `json:"nama"`
	Slug           string    `json:"slug"`
	TotalTransaksi float64   `json:"total_transaksi"`
	SaldoTertunda  float64   `json:"saldo_tertunda"`
	Status         string    `json:"status"`
	Mode           string    `json:"mode"`
	FeeByMerchant  bool      `json:"fee_by_merchant"`
	WebhookURL     string    `json:"webhook_url"`
	NotifikasiKe   string    `json:"notifikasi_ke"`
	APIKey         string    `json:"api_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	UserID         uint      `json:"user_id"`
}
