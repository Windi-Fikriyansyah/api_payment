package repository

import (
	"database/sql"
	"payment_service/internal/models"
)

type AuditLogRepository struct {
	DB *sql.DB
}

func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{DB: db}
}

func (r *AuditLogRepository) CreateWithTx(tx *sql.Tx, a *models.AuditLog) error {
	query := `INSERT INTO audit_logs (project_id, transaction_id, before_balance, after_balance, amount, type, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING id`
	return tx.QueryRow(query, a.ProjectID, a.TransactionID, a.BeforeBalance, a.AfterBalance, a.Amount, a.Type).Scan(&a.ID)
}
