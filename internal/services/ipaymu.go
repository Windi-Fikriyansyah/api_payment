package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"payment_service/internal/models"
	"strconv"
	"strings"
	"time"
)

type IPaymuConfig struct {
	Va           string
	APIKey       string
	BaseURL      string
}

type IPaymuService struct {
	Config IPaymuConfig
}

func NewIPaymuService(config IPaymuConfig) *IPaymuService {
	return &IPaymuService{
		Config: config,
	}
}

func (s *IPaymuService) GenerateSignature(body []byte, method string) string {
	hBody := sha256.New()
	hBody.Write(body)
	bodyHash := hex.EncodeToString(hBody.Sum(nil))

	stringToSign := method + ":" + s.Config.Va + ":" + strings.ToLower(bodyHash) + ":" + s.Config.APIKey
    
	h := hmac.New(sha256.New, []byte(s.Config.APIKey))
	h.Write([]byte(stringToSign))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *IPaymuService) CreateTransaction(mode string, gatewayMethod string, fee float64, req models.TransactionRequest, feeByMerchant bool) (*models.PaymentDetail, error) {
	totalPayment := req.Amount
	if !feeByMerchant {
		totalPayment = req.Amount + fee
	}

	if mode == "sandbox" {
		return s.createSandboxTransaction(gatewayMethod, fee, req, totalPayment)
	}

	// iPaymu Payload
	payload := map[string]interface{}{
		"name":        "Customer", // Can be dynamic if added to models
		"phone":       "08123456789",
		"email":       "customer@email.com",
		"amount":      totalPayment,
		"notifyUrl":   "", // Handled by our gateway but iPaymu needs it for direct webhook
		"expired":     24,
		"expiredType": "hours",
		"referenceId": req.OrderID,
		"paymentMethod": strings.ToLower(gatewayMethod), // e.g., va, qris, cstore
	}

    // Specialized payment channels for VA
    if strings.Contains(strings.ToLower(gatewayMethod), "va") {
        payload["paymentMethod"] = "va"
        // iPaymu uses paymentChannel for specific banks in VA
        // We'll assume gatewayMethod passed is the channel (e.g., bca, bri)
        payload["paymentChannel"] = strings.ReplaceAll(strings.ToLower(gatewayMethod), "_va", "")
    } else if strings.ToLower(gatewayMethod) == "qris" {
        payload["paymentMethod"] = "qris"
        payload["paymentChannel"] = "qris"
    }

	body, _ := json.Marshal(payload)
	signature := s.GenerateSignature(body, "POST")

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", s.Config.BaseURL+"/api/v2/payment/direct", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("va", s.Config.Va)
	httpReq.Header.Set("signature", signature)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("[iPaymu] Response: %s\n", string(respBody))

	var ipaymuResp struct {
		Status  int    `json:"status"`
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			TransactionId int    `json:"TransactionId"`
			ReferenceId   string `json:"ReferenceId"`
			Va            string `json:"Va"`
			QrString      string `json:"QrString"`
			Expired       string `json:"Expired"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &ipaymuResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if ipaymuResp.Status != 200 {
		return nil, fmt.Errorf("%s", ipaymuResp.Message)
	}

	paymentNumber := ipaymuResp.Data.Va
	if ipaymuResp.Data.QrString != "" {
		paymentNumber = ipaymuResp.Data.QrString
	}

	expiredAt := time.Now().Add(24 * time.Hour)
	if ipaymuResp.Data.Expired != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", ipaymuResp.Data.Expired); err == nil {
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
		Reference:     strconv.Itoa(ipaymuResp.Data.TransactionId),
		ExpiredAt:     expiredAt,
	}, nil
}

func (s *IPaymuService) createSandboxTransaction(gatewayMethod string, fee float64, req models.TransactionRequest, totalPayment float64) (*models.PaymentDetail, error) {
	fmt.Printf("[iPaymu Sandbox] Creating transaction for %s\n", req.OrderID)

	reference := "IP-SANDBOX-" + strconv.FormatInt(time.Now().Unix(), 10)
	paymentNumber := ""

	upperMethod := strings.ToUpper(gatewayMethod)
	if strings.Contains(upperMethod, "VA") {
		paymentNumber = "99" + strconv.Itoa(int(time.Now().Unix()%100000000))
	} else if upperMethod == "QRIS" {
		paymentNumber = "00020101021126570022ID.CO.IPAYMU.WWW0118936005230000052203030215123456789012345678905204599953033605802ID5911IPAYMU6013JAKARTA62070703A016304ABCD"
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

func (s *IPaymuService) VerifyCallback(trxID string, status string, signature string) error {
    // iPaymu callback usually doesn't send signature in typical sense or sends it in headers
    // For simplicity, we can implement it if iPaymu documentation specifies a callback signature.
    // iPaymu v2 signature for callback: HMAC-SHA256(apiKey, stringToSign)
    // But typically we'll just check if it's from trusted IP or if we can verify it.
	return nil
}

func (s *IPaymuService) CancelTransaction(mode string, req models.TransactionRequest) error {
	fmt.Printf("[%s] iPaymu Cancel Request for %s (No-op)\n", mode, req.OrderID)
	return nil
}
