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
	"time"
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

func (s *DuitkuService) CalculateFee(method string, amount float64) float64 {
	switch method {
	case "bri_va", "bni_va", "atm_bersama_va", "bnc_va", "cimb_niaga_va", "maybank_va", "permata_va":
		return 3500
	case "mandiri_va":
		return 4500
	case "bca_va":
		return 5500
	case "artha_graha_va", "sampoerna_va":
		return 2000
	case "qris":
		return (amount * 0.007) + 310
	default:
		return 0
	}
}

func (s *DuitkuService) CreateTransaction(mode string, method string, req models.TransactionRequest, feeByMerchant bool) (*models.PaymentDetail, error) {
	cfg := s.getByMode(mode)
	duitkuMethod := mapMethod(method)

	fee := s.CalculateFee(method, req.Amount)
	totalPayment := req.Amount
	if !feeByMerchant {
		totalPayment = req.Amount + fee
	}

	amountInt := int(totalPayment)
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
	fmt.Printf("[%s] Duitku Request Payload (Total: %d, Fee: %f): %s\n", mode, amountInt, fee, string(jsonPayload))

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
		Fee:           fee,
		TotalPayment:  totalPayment,
		PaymentMethod: method,
		PaymentNumber: paymentNumber,
		Reference:     duitkuResp.Reference,
		ExpiredAt:     time.Now().Add(1 * time.Hour), // Add dummy expiry if Duitku doesn't return it
	}, nil
}

func mapMethod(method string) string {
	switch method {
	case "qris":
		return "SP"
	case "bri_va":
		return "BR"
	case "bni_va":
		return "I1"
	case "atm_bersama_va":
		return "A1"
	case "bnc_va":
		return "BN"
	case "cimb_niaga_va":
		return "B1"
	case "maybank_va":
		return "VA"
	case "permata_va":
		return "BT"
	case "artha_graha_va":
		return "AG"
	case "sampoerna_va":
		return "SA"
	case "mandiri_va":
		return "M2"
	case "bca_va":
		return "BC"
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
