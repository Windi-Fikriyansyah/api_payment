package repository

import (
	"database/sql"
	"payment_service/internal/models"
)

type LedgerRepository struct {
	DB *sql.DB
}

func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{DB: db}
}

func (r *LedgerRepository) CreateWithTx(tx *sql.Tx, l *models.Ledger) error {
	query := `INSERT INTO ledgers (project_id, transaction_id, amount, type, description, created_at)
	          VALUES ($1, $2, $3, $4, $5, NOW()) RETURNING id`
	return tx.QueryRow(query, l.ProjectID, l.TransactionID, l.Amount, l.Type, l.Description).Scan(&l.ID)
}
