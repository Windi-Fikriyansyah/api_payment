package repository

import (
	"database/sql"
	"payment_service/internal/models"
)

type PaymentMethodRepository struct {
	DB *sql.DB
}

func NewPaymentMethodRepository(db *sql.DB) *PaymentMethodRepository {
	return &PaymentMethodRepository{DB: db}
}

func (r *PaymentMethodRepository) GetAllActive() ([]models.PaymentMethod, error) {
	query := `SELECT id, code, gateway_code, name, image_url, fee_flat, fee_percent, is_active 
	          FROM payment_methods WHERE is_active = TRUE ORDER BY id ASC`

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []models.PaymentMethod
	for rows.Next() {
		var m models.PaymentMethod
		err := rows.Scan(&m.ID, &m.Code, &m.GatewayCode, &m.Name, &m.ImageURL, &m.FeeFlat, &m.FeePercent, &m.IsActive)
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	return methods, nil
}

func (r *PaymentMethodRepository) FindByCode(code string) (*models.PaymentMethod, error) {
	query := `SELECT id, code, gateway_code, name, image_url, fee_flat, fee_percent, is_active 
	          FROM payment_methods WHERE code = $1 LIMIT 1`

	row := r.DB.QueryRow(query, code)
	var m models.PaymentMethod
	err := row.Scan(&m.ID, &m.Code, &m.GatewayCode, &m.Name, &m.ImageURL, &m.FeeFlat, &m.FeePercent, &m.IsActive)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func (r *PaymentMethodRepository) GetByProjectID(projectID uint) ([]models.PaymentMethod, error) {
	query := `SELECT pm.id, pm.code, pm.gateway_code, pm.name, pm.image_url, pm.fee_flat, pm.fee_percent, pm.is_active 
	          FROM payment_methods pm
	          JOIN project_payment_methods ppm ON pm.id = ppm.payment_method_id
	          WHERE ppm.project_id = $1 AND pm.is_active = TRUE
	          ORDER BY pm.id ASC`

	rows, err := r.DB.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []models.PaymentMethod
	for rows.Next() {
		var m models.PaymentMethod
		err := rows.Scan(&m.ID, &m.Code, &m.GatewayCode, &m.Name, &m.ImageURL, &m.FeeFlat, &m.FeePercent, &m.IsActive)
		if err != nil {
			return nil, err
		}
		methods = append(methods, m)
	}
	return methods, nil
}
