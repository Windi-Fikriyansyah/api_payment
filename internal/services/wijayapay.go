package services

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"payment_service/internal/models"
	"strings"
	"time"
)

type WijayaPayConfig struct {
	MerchantCode string
	APIKey       string
	BaseURL      string
	AppURL       string
}

type WijayaPayService struct {
	Config WijayaPayConfig
}

func NewWijayaPayService(config WijayaPayConfig) *WijayaPayService {
	return &WijayaPayService{
		Config: config,
	}
}

func (s *WijayaPayService) GenerateSignature(refID string) string {
	data := s.Config.MerchantCode + s.Config.APIKey + refID
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *WijayaPayService) CreateTransaction(mode string, gatewayMethod string, fee float64, req models.TransactionRequest, feeByMerchant bool) (*models.PaymentDetail, error) {
	totalPayment := req.Amount
	if !feeByMerchant {
		totalPayment = req.Amount + fee
	}

	// WijayaPay focuses on Production, but we can implement a mock for sandbox if needed.
	// For now, let's assume we use the real API or a mock if mode is sandbox.
	if mode == "sandbox" {
		return s.createSandboxTransaction(gatewayMethod, fee, req, totalPayment)
	}

	signature := s.GenerateSignature(req.OrderID)

	data := url.Values{}
	data.Set("code_merchant", s.Config.MerchantCode)
	data.Set("api_key", s.Config.APIKey)
	data.Set("ref_id", req.OrderID)
	data.Set("code_payment", gatewayMethod)
	data.Set("nominal", fmt.Sprintf("%d", int(totalPayment)))
	data.Set("customer_name", "Customer")
	data.Set("customer_email", "customer@email.com")
	data.Set("customer_phone", "08123456789")
	data.Set("callback_url", s.Config.AppURL+"/webhook/wijayapay")
	data.Set("signature", signature)

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", s.Config.BaseURL+"/transaction/create", strings.NewReader(data.Encode()))
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

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("[WijayaPay] Response: %s\n", string(respBody))

	var wpayResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			TrxReference string `json:"trx_reference"`
			RefId        string `json:"ref_id"`
			PaymentName  string `json:"payment_name"`
			TotalBayar   int    `json:"total_bayar"`
			VaNumber     string `json:"va_number"`
			QrString     string `json:"qr_string"`
			QrImage      string `json:"qr_image"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &wpayResp); err != nil {
		return nil, fmt.Errorf("failed to parse WijayaPay response: %v", err)
	}

	if !wpayResp.Success {
		return nil, fmt.Errorf("WijayaPay error: %s", wpayResp.Message)
	}

	paymentNumber := wpayResp.Data.VaNumber
	if paymentNumber == "" {
		paymentNumber = wpayResp.Data.QrString
	}

	return &models.PaymentDetail{
		Project:       req.Project,
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		Fee:           fee,
		TotalPayment:  totalPayment,
		PaymentMethod: gatewayMethod,
		PaymentNumber: paymentNumber,
		Reference:     wpayResp.Data.TrxReference,
		ExpiredAt:     time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *WijayaPayService) createSandboxTransaction(gatewayMethod string, fee float64, req models.TransactionRequest, totalPayment float64) (*models.PaymentDetail, error) {
	reference := "WP-SANDBOX-" + req.OrderID
	paymentNumber := "99" + fmt.Sprintf("%d", time.Now().Unix()%1000000)

	if gatewayMethod == "QRIS" {
		paymentNumber = "00020101021126570022ID.CO.WIJAYAPAY.WWW0118936005230000052203030215123456789012345678905204599953033605802ID5911WIJAYAPAY6013JAKARTA62070703A016304ABCD"
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
		ExpiredAt:     time.Now().Add(24 * time.Hour),
	}, nil
}

func (s *WijayaPayService) VerifyCallbackSignature(refID string, signature string) bool {
	expected := s.GenerateSignature(refID)
	return expected == signature
}
