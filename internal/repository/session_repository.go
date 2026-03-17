package repository

import (
	"database/sql"
	"payment_service/internal/models"
)

type SessionRepository struct {
	DB *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{DB: db}
}

func (r *SessionRepository) Create(s *models.PaymentSession) error {
	query := `INSERT INTO payment_sessions (token, project_id, amount, order_id, redirect_url, expired_at) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	
	err := r.DB.QueryRow(query, s.Token, s.ProjectID, s.Amount, s.OrderID, s.RedirectURL, s.ExpiredAt).Scan(&s.ID)
	return err
}

func (r *SessionRepository) FindByToken(token string) (*models.PaymentSession, error) {
	query := `SELECT id, token, project_id, amount, order_id, redirect_url, expired_at, created_at 
	          FROM payment_sessions WHERE token = $1 AND expired_at > NOW() LIMIT 1`
	
	row := r.DB.QueryRow(query, token)
	
	var s models.PaymentSession
	err := row.Scan(&s.ID, &s.Token, &s.ProjectID, &s.Amount, &s.OrderID, &s.RedirectURL, &s.ExpiredAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
