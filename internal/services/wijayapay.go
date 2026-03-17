package services

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"payment_service/internal/models"
	"strconv"
	"strings"
	"time"
)

type WijayaPayConfig struct {
	MerchantCode string
	APIKey       string
	BaseURL      string
}

type WijayaPayService struct {
	Config WijayaPayConfig
}

func NewWijayaPayService(config WijayaPayConfig) *WijayaPayService {
	return &WijayaPayService{
		Config: config,
	}
}

func (s *WijayaPayService) GenerateSignature(merchantCode, apiKey, refID string) string {
	data := merchantCode + apiKey + refID
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *WijayaPayService) CreateTransaction(mode string, gatewayMethod string, fee float64, req models.TransactionRequest, feeByMerchant bool) (*models.PaymentDetail, error) {
	totalPayment := req.Amount
	if !feeByMerchant {
		totalPayment = req.Amount + fee
	}

	if mode == "sandbox" {
		return s.createSandboxTransaction(gatewayMethod, fee, req, totalPayment)
	}

	signature := s.GenerateSignature(s.Config.MerchantCode, s.Config.APIKey, req.OrderID)

	data := url.Values{}
	data.Set("code_merchant", s.Config.MerchantCode)
	data.Set("api_key", s.Config.APIKey)
	data.Set("ref_id", req.OrderID)
	data.Set("code_payment", gatewayMethod)
	data.Set("nominal", strconv.FormatFloat(totalPayment, 'f', 0, 64))

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", s.Config.BaseURL+"/api/transaction/create", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("X-Signature", signature)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("[WijayaPay] Response: %s\n", string(body))

	var wijayaResp struct {
		Success bool `json:"success"`
		Message string `json:"message"`
		Data    struct {
			TrxReference    string `json:"trx_reference"`
			TotalBayar       int    `json:"total_bayar"`
			NomorVa          string `json:"nomor_va"`          // For VA
			QrString         string `json:"qr_string"`         // For QRIS
			NomorPembayaran  string `json:"nomor_pembayaran"`  // For Retail
			ExpiredAt        string `json:"expired_at"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &wijayaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if !wijayaResp.Success {
		return nil, fmt.Errorf("WijayaPay Error: %s", wijayaResp.Message)
	}

	paymentNumber := wijayaResp.Data.NomorVa
	if wijayaResp.Data.QrString != "" {
		paymentNumber = wijayaResp.Data.QrString
	} else if wijayaResp.Data.NomorPembayaran != "" {
		paymentNumber = wijayaResp.Data.NomorPembayaran
	}

	expiredAt := time.Now().Add(24 * time.Hour)
	if wijayaResp.Data.ExpiredAt != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", wijayaResp.Data.ExpiredAt); err == nil {
			expiredAt = t
		}
	}

	return &models.PaymentDetail{
		Project:       req.Project,
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		Fee:           fee,
		TotalPayment:  totalPayment,
		PaymentMethod: gatewayMethod,
		PaymentNumber: paymentNumber,
		Reference:     wijayaResp.Data.TrxReference,
		ExpiredAt:     expiredAt,
	}, nil
}

func (s *WijayaPayService) createSandboxTransaction(gatewayMethod string, fee float64, req models.TransactionRequest, totalPayment float64) (*models.PaymentDetail, error) {
	fmt.Printf("[WijayaPay Sandbox] Creating transaction for %s\n", req.OrderID)

	// Mock data based on WijayaPay structure
	reference := "WP-SANDBOX-" + req.OrderID
	paymentNumber := ""

	// Determine payment number format based on method
	upperMethod := strings.ToUpper(gatewayMethod)
	if strings.Contains(upperMethod, "VA") {
		paymentNumber = "99" + strconv.Itoa(int(time.Now().Unix()%100000000))
	} else if upperMethod == "QRIS" {
		paymentNumber = "00020101021126570022ID.CO.WIJAYAPAY.WWW0118936005230000052203030215123456789012345678905204599953033605802ID5911WIJAYAPAY6013JAKARTA62070703A016304ABCD"
	} else if upperMethod == "ALFAMART" || upperMethod == "INDOMARET" {
		paymentNumber = "88" + strconv.Itoa(int(time.Now().Unix()%100000000))
	} else {
		paymentNumber = "SANDBOX-" + strconv.FormatInt(time.Now().Unix(), 10)
	}

	return &models.PaymentDetail{
		Project:       req.Project,
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		Fee:           fee,
		TotalPayment:  totalPayment,
		PaymentMethod: gatewayMethod,
		PaymentNumber: paymentNumber,
		Reference:     reference,
		ExpiredAt:     time.Now().Add(1 * time.Hour),
	}, nil
}

func (s *WijayaPayService) VerifyCallback(orderID string, totalBayar string, signature string) error {
	expected := s.GenerateSignature(s.Config.MerchantCode, s.Config.APIKey, orderID)
	if signature == expected {
		return nil
	}
	// Also check signature for Sandbox just in case
	sandboxExpected := s.GenerateSignature(s.Config.MerchantCode, s.Config.APIKey, orderID)
	if signature == sandboxExpected {
		return nil
	}
	
	return errors.New("invalid signature")
}

func (s *WijayaPayService) CancelTransaction(mode string, req models.TransactionRequest) error {
	// WijayaPay might not have a cancel API accessible the same way as Duitku.
	// We'll leave it as a no-op or implement if found.
	// Based on subagent search, no cancel API was mentioned.
	fmt.Printf("[%s] WijayaPay Cancel Request for %s (No-op as per docs)\n", mode, req.OrderID)
	return nil
}
