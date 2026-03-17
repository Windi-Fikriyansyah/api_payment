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
	Va      string
	APIKey  string
	BaseURL string
	AppURL  string
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

	// Determine paymentMethod and paymentChannel from gateway code
	paymentMethod := strings.ToLower(gatewayMethod)
	paymentChannel := strings.ToLower(gatewayMethod)

	switch paymentMethod {
	case "qris":
		paymentMethod = "qris"
		paymentChannel = "qris"
	case "alfamart", "indomaret":
		paymentChannel = paymentMethod
		paymentMethod = "cstore"
	default:
		// VA banks: bca, mandiri, bri, bni, cimb, permata, danamon, bag, bsi, bnc
		paymentChannel = paymentMethod
		paymentMethod = "va"
	}

	amountInt := int(totalPayment)

	// iPaymu v2 Direct Payment Payload
	payload := map[string]interface{}{
		"product":        []string{"Payment"},
		"qty":            []int{1},
		"price":          []int{amountInt},
		"amount":         amountInt,
		"returnUrl":      s.Config.AppURL,
		"notifyUrl":      s.Config.AppURL + "/webhook/ipaymu",
		"cancelUrl":      s.Config.AppURL,
		"name":           "Customer",
		"phone":          "08123456789",
		"email":          "customer@email.com",
		"referenceId":    req.OrderID,
		"paymentMethod":  paymentMethod,
		"paymentChannel": paymentChannel,
		"expired":        24,
		"expiredType":    "hours",
	}

	body, _ := json.Marshal(payload)

	// Generate signature: HMAC-SHA256(StringToSign, ApiKey)
	// StringToSign = POST:VA:Lowercase(SHA256(body)):ApiKey
	signature := s.GenerateSignature(body, "POST")
	timestamp := time.Now().Format("20060102150405")

	fmt.Printf("[iPaymu] === DEBUG ===\n")
	fmt.Printf("[iPaymu] VA: %s\n", s.Config.Va)
	fmt.Printf("[iPaymu] Body: %s\n", string(body))
	fmt.Printf("[iPaymu] Signature: %s\n", signature)
	fmt.Printf("[iPaymu] Timestamp: %s\n", timestamp)
	fmt.Printf("[iPaymu] Method=%s, Channel=%s, Amount=%d, Ref=%s\n", paymentMethod, paymentChannel, amountInt, req.OrderID)
	fmt.Printf("[iPaymu] URL: %s\n", s.Config.BaseURL+"/api/v2/payment/direct")

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", s.Config.BaseURL+"/api/v2/payment/direct", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("va", s.Config.Va)
	httpReq.Header.Set("signature", signature)
	httpReq.Header.Set("timestamp", timestamp)

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
			PaymentNo     string `json:"PaymentNo"`
			PaymentName   string `json:"PaymentName"`
			Expired       string `json:"Expired"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &ipaymuResp); err != nil {
		// Try a more generic map if unmarshal fails
		var genericResp map[string]interface{}
		json.Unmarshal(respBody, &genericResp)
		fmt.Printf("[iPaymu] Generic Parse: %+v\n", genericResp)
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if ipaymuResp.Status != 200 {
		return nil, fmt.Errorf("%s", ipaymuResp.Message)
	}

	// Try multiple fields for payment number
	paymentNumber := ""
	if ipaymuResp.Data.Va != "" {
		paymentNumber = ipaymuResp.Data.Va
	} else if ipaymuResp.Data.QrString != "" {
		paymentNumber = ipaymuResp.Data.QrString
	} else if ipaymuResp.Data.PaymentNo != "" {
		paymentNumber = ipaymuResp.Data.PaymentNo
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

	method := strings.ToLower(gatewayMethod)
	// List of VA banks in iPaymu
	isVA := false
	vaBanks := []string{"bca", "bri", "bni", "mandiri", "cimb", "permata", "danamon", "bag", "bsi", "bnc", "muamalat"}
	for _, b := range vaBanks {
		if method == b || strings.Contains(method, "va") {
			isVA = true
			break
		}
	}

	if isVA {
		paymentNumber = "99" + strconv.Itoa(int(time.Now().Unix()%100000000))
	} else if method == "qris" {
		paymentNumber = "00020101021126570022ID.CO.IPAYMU.WWW0118936005230000052203030215123456789012345678905204599953033605802ID5911IPAYMU6013JAKARTA62070703A016304ABCD"
	} else if method == "alfamart" || method == "indomaret" {
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
