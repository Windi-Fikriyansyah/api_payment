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
	query := `INSERT INTO transactions (project_id, order_id, gateway_order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, jenis, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW()) RETURNING id`

	return r.DB.QueryRow(query, t.ProjectID, t.OrderID, t.GatewayOrderID, t.Reference, t.Amount, t.Fee, t.TotalPayment, t.Status, t.Mode, t.PaymentMethod, t.PaymentNumber, t.Jenis).Scan(&t.ID)
}

func (r *TransactionRepository) UpdateStatusWithTx(tx *sql.Tx, orderID string, reference string, status string) error {
	query := `UPDATE transactions SET status = $1, updated_at = NOW() WHERE (order_id = $2 OR gateway_order_id = $2) AND reference = $3`
	_, err := tx.Exec(query, status, orderID, reference)
	return err
}

func (r *TransactionRepository) FindByOrderID(orderID string) (*models.Transaction, error) {
	query := `SELECT id, project_id, order_id, gateway_order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, jenis, created_at, updated_at 
	          FROM transactions WHERE order_id = $1 OR gateway_order_id = $1 LIMIT 1`

	row := r.DB.QueryRow(query, orderID)
	var t models.Transaction
	err := row.Scan(&t.ID, &t.ProjectID, &t.OrderID, &t.GatewayOrderID, &t.Reference, &t.Amount, &t.Fee, &t.TotalPayment, &t.Status, &t.Mode, &t.PaymentMethod, &t.PaymentNumber, &t.Jenis, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepository) FindByOrderAndReference(orderID string, reference string) (*models.Transaction, error) {
	query := `SELECT id, project_id, order_id, gateway_order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, jenis, created_at, updated_at 
	          FROM transactions WHERE (order_id = $1 OR gateway_order_id = $1) AND reference = $2 LIMIT 1`

	row := r.DB.QueryRow(query, orderID, reference)
	var t models.Transaction
	err := row.Scan(&t.ID, &t.ProjectID, &t.OrderID, &t.GatewayOrderID, &t.Reference, &t.Amount, &t.Fee, &t.TotalPayment, &t.Status, &t.Mode, &t.PaymentMethod, &t.PaymentNumber, &t.Jenis, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepository) FindProjectByTransactionOrderAndReference(orderID string, reference string) (*models.Project, error) {
	query := `SELECT p.id, p.nama, p.slug, p.webhook_url, p.api_key, p.notifikasi_ke 
	          FROM projects p 
	          JOIN transactions t ON p.id = t.project_id 
	          WHERE (t.order_id = $1 OR t.gateway_order_id = $1) AND t.reference = $2 LIMIT 1`

	row := r.DB.QueryRow(query, orderID, reference)
	var p models.Project
	err := row.Scan(&p.ID, &p.Nama, &p.Slug, &p.WebhookURL, &p.APIKey, &p.NotifikasiKe)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *TransactionRepository) FindByProjectAndOrderID(projectID uint, orderID string) (*models.Transaction, error) {
	query := `SELECT id, project_id, order_id, gateway_order_id, reference, amount, fee, total_payment, status, mode, payment_method, payment_number, jenis, created_at, updated_at 
	          FROM transactions WHERE project_id = $1 AND (order_id = $2 OR gateway_order_id = $2) LIMIT 1`

	row := r.DB.QueryRow(query, projectID, orderID)
	var t models.Transaction
	err := row.Scan(&t.ID, &t.ProjectID, &t.OrderID, &t.GatewayOrderID, &t.Reference, &t.Amount, &t.Fee, &t.TotalPayment, &t.Status, &t.Mode, &t.PaymentMethod, &t.PaymentNumber, &t.Jenis, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
func (r *TransactionRepository) UpdatePaymentMethod(id uint, reference string, fee float64, totalPayment float64, method string, paymentNumber string) error {
	query := `UPDATE transactions SET reference = $1, fee = $2, total_payment = $3, payment_method = $4, payment_number = $5, updated_at = NOW() WHERE id = $6`
	_, err := r.DB.Exec(query, reference, fee, totalPayment, method, paymentNumber, id)
	return err
}
