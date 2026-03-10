package services

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"payment_service/internal/models"
	"strconv"
)

type DuitkuConfig struct {
	MerchantCode string
	APIKey       string
	BaseURL      string
}

type DuitkuService struct {
	Sandbox    DuitkuConfig
	Production DuitkuConfig
}

func NewDuitkuService(sandbox, production DuitkuConfig) *DuitkuService {
	return &DuitkuService{
		Sandbox:    sandbox,
		Production: production,
	}
}

func (s *DuitkuService) getByMode(mode string) DuitkuConfig {
	if mode == "production" {
		return s.Production
	}
	return s.Sandbox
}

func (s *DuitkuService) GenerateSignature(mode string, orderID string, amount int) string {
	cfg := s.getByMode(mode)
	data := cfg.MerchantCode + orderID + strconv.Itoa(amount) + cfg.APIKey
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *DuitkuService) CreateTransaction(mode string, method string, req models.TransactionRequest) (*models.PaymentDetail, error) {
	cfg := s.getByMode(mode)
	duitkuMethod := mapMethod(method)
	amountInt := int(req.Amount)
	signature := s.GenerateSignature(mode, req.OrderID, amountInt)

	payload := map[string]interface{}{
		"merchantCode":    cfg.MerchantCode,
		"paymentAmount":   amountInt,
		"paymentMethod":   duitkuMethod,
		"merchantOrderId": req.OrderID,
		"productDetails":  "Payment for " + req.OrderID,
		"email":           "customer@example.com",
		"phoneNumber":     "08123456789",
		"signature":       signature,
		"callbackUrl":     os.Getenv("APP_URL") + "/webhook/duitku",
		"returnUrl":       os.Getenv("APP_URL") + "/return",
		"expiryPeriod":    60,
	}

	jsonPayload, _ := json.Marshal(payload)
	fmt.Printf("[%s] Duitku Request Payload: %s\n", mode, string(jsonPayload))

	resp, err := http.Post(cfg.BaseURL+"/webapi/api/merchant/v2/inquiry", "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[%s] Duitku Response: %s\n", mode, string(body))

	var duitkuResp struct {
		Reference     string `json:"reference"`
		VaNumber      string `json:"vaNumber"`
		QrString      string `json:"qrString"`
		StatusCode    string `json:"statusCode"`
		StatusMessage string `json:"statusMessage"`
	}

	if err := json.Unmarshal(body, &duitkuResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if duitkuResp.StatusCode != "00" {
		return nil, fmt.Errorf("Duitku Error: %s - %s", duitkuResp.StatusCode, duitkuResp.StatusMessage)
	}

	paymentNumber := duitkuResp.VaNumber
	if method == "qris" {
		paymentNumber = duitkuResp.QrString
	}

	return &models.PaymentDetail{
		Project:       req.Project,
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		Fee:           0,
		TotalPayment:  req.Amount,
		PaymentMethod: method,
		PaymentNumber: paymentNumber,
		Reference:     duitkuResp.Reference,
	}, nil
}

func mapMethod(method string) string {
	switch method {
	case "qris":
		return "SP"
	case "bni_va":
		return "I1"
	case "bri_va":
		return "BR"
	case "permata_va":
		return "BT"
	case "maybank_va":
		return "VA"
	case "cimb_niaga_va":
		return "B1"
	case "atm_bersama_va":
		return "A1"
	default:
		return "SP"
	}
}

func (s *DuitkuService) CheckStatus(mode string, project string, orderID string, amount int) (*models.TransactionDetail, error) {
	// Status API call would go here using cfg.BaseURL/cfg.MerchantCode etc.
	return &models.TransactionDetail{
		Amount:        float64(amount),
		OrderID:       orderID,
		Project:       project,
		Status:        "pending",
		PaymentMethod: "unknown",
	}, nil
}

func (s *DuitkuService) VerifyCallback(orderID string, amount string, signature string) error {
	// Try Sandbox first, then Prod?
	// Or based on actual transaction data if we passed it in.
	// For now, check both as it's easier or assume a default.

	if s.verifyOne(s.Sandbox, orderID, amount, signature) == nil ||
		s.verifyOne(s.Production, orderID, amount, signature) == nil {
		return nil
	}
	return errors.New("invalid signature")
}

func (s *DuitkuService) verifyOne(cfg DuitkuConfig, orderID string, amount string, signature string) error {
	data := cfg.MerchantCode + amount + orderID + cfg.APIKey
	hash := md5.Sum([]byte(data))
	expected := hex.EncodeToString(hash[:])
	if signature == expected {
		return nil
	}
	return errors.New("invalid")
}

func (s *DuitkuService) CancelTransaction(mode string, req models.TransactionRequest) error {
	cfg := s.getByMode(mode)
	amountInt := int(req.Amount)
	signature := s.GenerateSignature(mode, req.OrderID, amountInt)

	payload := map[string]interface{}{
		"merchantCode":    cfg.MerchantCode,
		"merchantOrderId": req.OrderID,
		"paymentAmount":   amountInt,
		"signature":       signature,
	}

	jsonPayload, _ := json.Marshal(payload)
	fmt.Printf("[%s] Duitku Cancel Request: %s\n", mode, string(jsonPayload))

	resp, err := http.Post(cfg.BaseURL+"/webapi/api/merchant/v2/cancel", "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("[%s] Duitku Cancel API unreachable: %v (proceeding with local cancel)\n", mode, err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[%s] Duitku Cancel Response (HTTP %d): %s\n", mode, resp.StatusCode, string(body))

	// Handle non-JSON response (e.g. Duitku sandbox doesn't support cancel)
	var cancelResp struct {
		StatusCode    string `json:"statusCode"`
		StatusMessage string `json:"statusMessage"`
	}

	if err := json.Unmarshal(body, &cancelResp); err != nil {
		fmt.Printf("[%s] Duitku cancel returned non-JSON response, proceeding with local cancel\n", mode)
		return nil
	}

	if cancelResp.StatusCode != "00" {
		fmt.Printf("[%s] Duitku Cancel Warning: %s - %s (proceeding with local cancel)\n", mode, cancelResp.StatusCode, cancelResp.StatusMessage)
	}

	return nil
}
