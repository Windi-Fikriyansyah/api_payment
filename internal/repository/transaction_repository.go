package repository

import (
	"database/sql"
	"payment_service/internal/models"
)

type TransactionRepository struct {
	DB *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{DB: db}
}

func (r *TransactionRepository) Create(t *models.Transaction) error {
	query := `INSERT INTO transactions (project_id, order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW()) RETURNING id`

	return r.DB.QueryRow(query, t.ProjectID, t.OrderID, t.Reference, t.Amount, t.Fee, t.TotalPayment, t.Status, t.Mode, t.PaymentMethod, t.PaymentNumber).Scan(&t.ID)
}

func (r *TransactionRepository) UpdateStatusWithTx(tx *sql.Tx, orderID string, status string) error {
	query := `UPDATE transactions SET status = $1, updated_at = NOW() WHERE order_id = $2`
	_, err := tx.Exec(query, status, orderID)
	return err
}

func (r *TransactionRepository) FindByOrderID(orderID string) (*models.Transaction, error) {
	query := `SELECT id, project_id, order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, created_at, updated_at 
	          FROM transactions WHERE order_id = $1 LIMIT 1`

	row := r.DB.QueryRow(query, orderID)
	var t models.Transaction
	err := row.Scan(&t.ID, &t.ProjectID, &t.OrderID, &t.Reference, &t.Amount, &t.Fee, &t.TotalPayment, &t.Status, &t.Mode, &t.PaymentMethod, &t.PaymentNumber, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepository) FindProjectByTransactionOrderID(orderID string) (*models.Project, error) {
	query := `SELECT p.id, p.nama, p.slug, p.webhook_url, p.api_key, p.notifikasi_ke 
	          FROM projects p 
	          JOIN transactions t ON p.id = t.project_id 
	          WHERE t.order_id = $1 LIMIT 1`

	row := r.DB.QueryRow(query, orderID)
	var p models.Project
	err := row.Scan(&p.ID, &p.Nama, &p.Slug, &p.WebhookURL, &p.APIKey, &p.NotifikasiKe)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
