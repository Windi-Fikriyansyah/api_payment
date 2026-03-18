package repository

import (
	"database/sql"
	"fmt"
	"payment_service/internal/models"
)

type ProjectRepository struct {
	DB *sql.DB
}

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{DB: db}
}

func (r *ProjectRepository) FindByAPIKey(apiKey string) (*models.Project, error) {
	query := `SELECT id, nama, slug, total_transaksi, saldo_tertunda, status, mode, fee_by_merchant, COALESCE(webhook_url, ''), COALESCE(notifikasi_ke, ''), api_key, created_at, updated_at, user_id, COALESCE(no_whatsapp, '') 
	          FROM projects WHERE api_key = $1 LIMIT 1`

	row := r.DB.QueryRow(query, apiKey)

	var p models.Project
	err := row.Scan(
		&p.ID, &p.Nama, &p.Slug, &p.TotalTransaksi, &p.SaldoTertunda, &p.Status, &p.Mode,
		&p.FeeByMerchant, &p.WebhookURL, &p.NotifikasiKe, &p.APIKey, &p.CreatedAt, &p.UpdatedAt, &p.UserID, &p.NoWhatsApp,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Auth Warning: API Key NOT FOUND in database: %s\n", apiKey)
		} else {
			fmt.Printf("Database Error during API Key lookup (%s): %v\n", apiKey, err)
		}
		return nil, err
	}

	return &p, nil
}

func (r *ProjectRepository) FindByID(id uint) (*models.Project, error) {
	query := `SELECT id, nama, slug, total_transaksi, saldo_tertunda, status, mode, fee_by_merchant, COALESCE(webhook_url, ''), COALESCE(notifikasi_ke, ''), api_key, created_at, updated_at, user_id, COALESCE(no_whatsapp, '') 
	          FROM projects WHERE id = $1 LIMIT 1`

	row := r.DB.QueryRow(query, id)

	var p models.Project
	err := row.Scan(
		&p.ID, &p.Nama, &p.Slug, &p.TotalTransaksi, &p.SaldoTertunda, &p.Status, &p.Mode,
		&p.FeeByMerchant, &p.WebhookURL, &p.NotifikasiKe, &p.APIKey, &p.CreatedAt, &p.UpdatedAt, &p.UserID, &p.NoWhatsApp,
	)

	return &p, err
}

func (r *ProjectRepository) FindByIDWithTx(tx *sql.Tx, id uint) (*models.Project, error) {
	query := `SELECT id, nama, slug, total_transaksi, saldo_tertunda, status, mode, fee_by_merchant, COALESCE(webhook_url, ''), COALESCE(notifikasi_ke, ''), api_key, created_at, updated_at, user_id, COALESCE(no_whatsapp, '') 
	          FROM projects WHERE id = $1 FOR UPDATE`

	row := tx.QueryRow(query, id)

	var p models.Project
	err := row.Scan(
		&p.ID, &p.Nama, &p.Slug, &p.TotalTransaksi, &p.SaldoTertunda, &p.Status, &p.Mode,
		&p.FeeByMerchant, &p.WebhookURL, &p.NotifikasiKe, &p.APIKey, &p.CreatedAt, &p.UpdatedAt, &p.UserID, &p.NoWhatsApp,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepository) UpdateBalanceWithTx(tx *sql.Tx, projectID uint, totalTransaksi float64, saldoTertunda float64) error {
	query := `UPDATE projects SET total_transaksi = $1, saldo_tertunda = $2, updated_at = NOW() WHERE id = $3`
	_, err := tx.Exec(query, totalTransaksi, saldoTertunda, projectID)
	return err
}
func (r *ProjectRepository) FindBySlug(slug string) (*models.Project, error) {
	query := `SELECT id, nama, slug, total_transaksi, saldo_tertunda, status, mode, fee_by_merchant, COALESCE(webhook_url, ''), COALESCE(notifikasi_ke, ''), api_key, created_at, updated_at, user_id, COALESCE(no_whatsapp, '') 
	          FROM projects WHERE slug = $1 LIMIT 1`
 

	row := r.DB.QueryRow(query, slug)

	var p models.Project
	err := row.Scan(
		&p.ID, &p.Nama, &p.Slug, &p.TotalTransaksi, &p.SaldoTertunda, &p.Status, &p.Mode,
		&p.FeeByMerchant, &p.WebhookURL, &p.NotifikasiKe, &p.APIKey, &p.CreatedAt, &p.UpdatedAt, &p.UserID, &p.NoWhatsApp,
	)

	if err != nil {
		return nil, err
	}

	return &p, nil
}
func (r *ProjectRepository) FindByNoWhatsApp(noWhatsApp string) (*models.Project, error) {
	query := `SELECT id, nama, slug, total_transaksi, saldo_tertunda, status, mode, fee_by_merchant, COALESCE(webhook_url, ''), COALESCE(notifikasi_ke, ''), api_key, created_at, updated_at, user_id, COALESCE(no_whatsapp, '') 
	          FROM projects WHERE no_whatsapp = $1 LIMIT 1`
 

	row := r.DB.QueryRow(query, noWhatsApp)

	var p models.Project
	err := row.Scan(
		&p.ID, &p.Nama, &p.Slug, &p.TotalTransaksi, &p.SaldoTertunda, &p.Status, &p.Mode,
		&p.FeeByMerchant, &p.WebhookURL, &p.NotifikasiKe, &p.APIKey, &p.CreatedAt, &p.UpdatedAt, &p.UserID, &p.NoWhatsApp,
	)

	return &p, err
}
func (r *ProjectRepository) CalculateBalance(projectID uint, mode string) (float64, error) {
	query := `
		SELECT (
			(SELECT COALESCE(SUM(l.amount), 0) FROM ledgers l JOIN transactions t ON l.transaction_id = t.id WHERE l.project_id = $1 AND t.status = 'success' AND t.mode = $2 AND l.type = 'credit') - 
			(SELECT COALESCE(SUM(l.amount), 0) FROM ledgers l JOIN penarikan p ON l.penarikan_id = p.id WHERE l.project_id = $1 AND p.status != 'Ditolak' AND p.mode = $2 AND l.type = 'debit')
		) as total_transaksi`

	var balance float64
	err := r.DB.QueryRow(query, projectID, mode).Scan(&balance)
	return balance, err
}
