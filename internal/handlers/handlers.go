package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"payment_service/internal/models"
	"payment_service/internal/repository"
	"payment_service/internal/services"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type PaymentHandler struct {
	WijayaPay         *services.WijayaPayService
	TransactionRepo   *repository.TransactionRepository
	ProjectRepo       *repository.ProjectRepository
	LedgerRepo        *repository.LedgerRepository
	AuditLogRepo      *repository.AuditLogRepository
	PaymentMethodRepo *repository.PaymentMethodRepository
	WorkerPool        *services.WorkerPool
	EmailService      *services.EmailService
	DB                *sql.DB
}

func NewPaymentHandler(
	wijayaPay *services.WijayaPayService,
	transactionRepo *repository.TransactionRepository,
	projectRepo *repository.ProjectRepository,
	ledgerRepo *repository.LedgerRepository,
	auditLogRepo *repository.AuditLogRepository,
	paymentMethodRepo *repository.PaymentMethodRepository,
	workerPool *services.WorkerPool,
	emailService *services.EmailService,
	db *sql.DB,
) *PaymentHandler {
	return &PaymentHandler{
		WijayaPay:         wijayaPay,
		TransactionRepo:   transactionRepo,
		ProjectRepo:       projectRepo,
		LedgerRepo:        ledgerRepo,
		AuditLogRepo:      auditLogRepo,
		PaymentMethodRepo: paymentMethodRepo,
		WorkerPool:        workerPool,
		EmailService:      emailService,
		DB:                db,
	}
}

func (h *PaymentHandler) CreateTransaction(c *fiber.Ctx) error {
	method := c.Params("method")
	var req models.TransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	project := c.Locals("project").(*models.Project)

	// Security: Validate project name matches
	if req.Project != project.Nama {
		return c.Status(401).JSON(fiber.Map{"error": "Project name mismatch"})
	}

	// Fetch Payment Method from DB
	pm, err := h.PaymentMethodRepo.FindByCode(method)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Metode pembayaran tidak valid"})
	}

	// Calculate Fee based on DB
	fee := pm.FeeFlat + (req.Amount * pm.FeePercent)

	// Prefix order_id for Gateway to ensure uniqueness across projects
	originalOrderID := req.OrderID
	gatewayOrderID := fmt.Sprintf("P%d-%s", project.ID, originalOrderID)
	req.OrderID = gatewayOrderID

	payment, err := h.WijayaPay.CreateTransaction(project.Mode, pm.GatewayCode, fee, req, project.FeeByMerchant)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	payment.ExpiredAt = time.Now().Add(24 * time.Hour)
	payment.OrderID = originalOrderID // Return original ID to merchant in response

	transaction := &models.Transaction{
		ProjectID:      project.ID,
		OrderID:        originalOrderID,
		GatewayOrderID: gatewayOrderID,
		Reference:      payment.Reference,
		Amount:         req.Amount,
		Fee:            payment.Fee,
		TotalPayment:   payment.TotalPayment,
		Status:         "pending",
		Mode:           project.Mode,
		PaymentMethod:  method,
		PaymentNumber:  payment.PaymentNumber,
	}

	err = h.TransactionRepo.Create(transaction)
	if err != nil {
		fmt.Printf("Database Error saving transaction: %v\n", err)
	}

	return c.Status(200).JSON(models.PaymentResponse{
		Payment: *payment,
	})
}

func (h *PaymentHandler) WijayaPayWebhook(c *fiber.Ctx) error {
	// IP Whitelisting Security
	clientIP := c.IP()
	allowedIP := "45.158.126.118"

	// Allow WijayaPay IP or localhost (for sandbox simulation)
	if clientIP != allowedIP && clientIP != "127.0.0.1" && clientIP != "::1" {
		fmt.Printf("Blocked Unauthorized IP: %s\n", clientIP)
		return c.Status(403).SendString("Unauthorized IP Source")
	}

	// WijayaPay callback structure based on latest documentation
	var payload struct {
		Status string `json:"status"` // e.g. "paid"
		Data   struct {
			RefID          string  `json:"ref_id"`
			TrxReference   string  `json:"trx_reference"`
			TotalDibayar   float64 `json:"total_dibayar"`
			PaymentMethode string  `json:"payment_methode"`
			UpdatedAt      string  `json:"updated_at"`
		} `json:"data"`
	}

	if err := c.BodyParser(&payload); err != nil {
		fmt.Printf("Webhook Parsing Error: %v\n", err)
		return c.Status(400).JSON(fiber.Map{"status": false})
	}

	signature := c.Get("X-Signature")

	fmt.Printf("Incoming WijayaPay Webhook: OrderID=%s, Status=%s\n",
		payload.Data.RefID, payload.Status)

	// Verify Signature (md5 of code_merchant + api_key + ref_id)
	err := h.WijayaPay.VerifyCallback(payload.Data.RefID, signature)
	if err != nil {
		fmt.Printf("Webhook Signature Invalid: %v\n", err)
		// Return true anyway if signature check failed to stop retries if desired, 
		// but typically we return false or error if we want them to retry. 
		// However, the doc says "wajib mengembalikan response {"status": true}"
		return c.Status(200).JSON(fiber.Map{"status": true})
	}

	// Fetch transaction to see if it exists
	tx, err := h.TransactionRepo.FindByOrderID(payload.Data.RefID)
	if err != nil {
		fmt.Printf("Webhook Error: Transaction %s not found in database: %v\n", payload.Data.RefID, err)
		// Return true as per requirement to stop retries for non-existent orders
		return c.Status(200).JSON(fiber.Map{"status": true})
	}

	// 3. Process Settlement Atomically
	if payload.Status == "paid" {
		err = h.ProcessSettlement(payload.Data.RefID, payload.Data.TrxReference)
		if err != nil {
			fmt.Printf("Settlement Error for %s: %v\n", payload.Data.RefID, err)
			return c.Status(200).JSON(fiber.Map{"status": false})
		}

		fmt.Printf("Transaction %s (Ref: %s) processed successfully\n", payload.Data.RefID, payload.Data.TrxReference)

		// 4. Forward Callback asynchronously
		tx, _ = h.TransactionRepo.FindByOrderAndReference(payload.Data.RefID, payload.Data.TrxReference)
		project, _ := h.TransactionRepo.FindProjectByTransactionOrderAndReference(payload.Data.RefID, payload.Data.TrxReference)

		if project != nil && project.WebhookURL != "" && tx.Jenis != "url" {
			netAmount := tx.Amount
			if tx.TotalPayment == tx.Amount {
				netAmount = tx.Amount - tx.Fee
			}

			h.WorkerPool.Submit(func() {
				h.SendCallback(project.WebhookURL, models.WebhookPayload{
					Amount:        tx.Amount,
					Fee:           tx.Fee,
					NetAmount:     netAmount,
					OrderID:       tx.OrderID,
					Project:       project.Nama,
					Status:        "success",
					PaymentMethod: tx.PaymentMethod,
					CompletedAt:   time.Now(),
				})
			})
		}

		// 5. Send notification email asynchronously
		if project != nil && project.NotifikasiKe != "" {
			h.WorkerPool.Submit(func() {
				err := h.EmailService.SendPaymentSuccessEmail(project.NotifikasiKe, project.Nama, tx.OrderID, tx.Amount)
				if err != nil {
					fmt.Printf("Email Notification Error for %s: %v\n", tx.OrderID, err)
				}
			})
		}
	}

	return c.Status(200).JSON(fiber.Map{"status": true})
}

func (h *PaymentHandler) SendCallback(url string, payload models.WebhookPayload) {
	jsonPayload, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("Callback Post Error to %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Callback sent to %s. Status: %s\n", url, resp.Status)
}

func (h *PaymentHandler) PaymentSimulation(c *fiber.Ctx) error {
	var req models.TransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	projectFromCtx := c.Locals("project").(*models.Project)
	if projectFromCtx.Mode != "sandbox" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Simulation only available for projects in sandbox mode",
		})
	}

	// Security: Validate project name matches
	if req.Project != projectFromCtx.Nama {
		return c.Status(401).JSON(fiber.Map{"error": "Project name mismatch"})
	}

	tx, err := h.TransactionRepo.FindByProjectAndOrderID(projectFromCtx.ID, req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	// Security: Validate amount matches
	if tx.Amount != req.Amount {
		return c.Status(400).JSON(fiber.Map{"error": "Transaction amount mismatch"})
	}

	// Security: Validate transaction mode
	if tx.Mode != "sandbox" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Simulation only available for transactions in sandbox mode",
		})
	}

	if tx.Status == "success" {
		return c.Status(200).JSON(fiber.Map{"message": "Transaction already paid"})
	}

	err = h.ProcessSettlement(req.OrderID, tx.Reference)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to settle: " + err.Error()})
	}

	project, _ := h.TransactionRepo.FindProjectByTransactionOrderAndReference(req.OrderID, tx.Reference)
	if project != nil && project.WebhookURL != "" {
		netAmount := tx.Amount
		if tx.TotalPayment == tx.Amount {
			netAmount = tx.Amount - tx.Fee
		}

		h.WorkerPool.Submit(func() {
			h.SendCallback(project.WebhookURL, models.WebhookPayload{
				Amount:        tx.Amount,
				Fee:           tx.Fee,
				NetAmount:     netAmount,
				OrderID:       tx.OrderID,
				Project:       project.Nama,
				Status:        "success",
				PaymentMethod: tx.PaymentMethod,
				CompletedAt:   time.Now(),
			})
		})
	}

	// Send notification email asynchronously
	if project != nil && project.NotifikasiKe != "" {
		h.WorkerPool.Submit(func() {
			// Use original order ID for email notification
			err := h.EmailService.SendPaymentSuccessEmail(project.NotifikasiKe, project.Nama, tx.OrderID, tx.Amount)
			if err != nil {
				fmt.Printf("Email Notification Error for %s: %v\n", tx.OrderID, err)
			}
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"status":  "success",
		"message": "Simulation successful for " + req.OrderID,
	})
}

func (h *PaymentHandler) TransactionCancel(c *fiber.Ctx) error {
	var req models.TransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	project := c.Locals("project").(*models.Project)

	// Security: Validate project name matches
	if req.Project != project.Nama {
		return c.Status(401).JSON(fiber.Map{"error": "Project name mismatch"})
	}

	// Find transaction in database (filtered by project to prevent affecting other projects)
	tx, err := h.TransactionRepo.FindByProjectAndOrderID(project.ID, req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	// Security: Validate amount matches
	if tx.Amount != req.Amount {
		return c.Status(400).JSON(fiber.Map{"error": "Transaction amount mismatch"})
	}

	// Validate transaction is in a cancellable state
	if tx.Status != "pending" {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("Transaction cannot be cancelled, current status: %s", tx.Status),
		})
	}

	// Call WijayaPay cancel API
	err = h.WijayaPay.CancelTransaction(project.Mode, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update transaction status in database atomically (partial transaction for status change)
	dtx, dtxe := h.DB.Begin()
	if dtxe == nil {
		h.TransactionRepo.UpdateStatusWithTx(dtx, req.OrderID, tx.Reference, "cancelled")
		dtx.Commit()
	}

	// Send webhook callback to project
	if project.WebhookURL != "" {
		h.WorkerPool.Submit(func() {
			h.SendCallback(project.WebhookURL, models.WebhookPayload{
				Amount:        tx.Amount,
				Fee:           0,         // No fee for cancelled
				NetAmount:     tx.Amount, // Or 0? Let's use Amount as base
				OrderID:       tx.OrderID,
				Project:       project.Nama,
				Status:        "cancelled",
				PaymentMethod: tx.PaymentMethod,
				CompletedAt:   time.Now(),
			})
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Transaction cancelled",
	})
}

func (h *PaymentHandler) TransactionDetail(c *fiber.Ctx) error {
	project := c.Locals("project").(*models.Project)
	orderID := c.Query("order_id")

	if orderID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "order_id is required"})
	}

	tx, err := h.TransactionRepo.FindByProjectAndOrderID(project.ID, orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	detail := models.TransactionDetail{
		Amount:        tx.Amount,
		Fee:           tx.Fee,
		TotalPayment:  tx.TotalPayment,
		OrderID:       tx.OrderID,
		Project:       project.Nama,
		Status:        tx.Status,
		PaymentMethod: tx.PaymentMethod,
	}

	if tx.Status == "success" {
		detail.CompletedAt = tx.UpdatedAt
	}

	return c.Status(200).JSON(models.TransactionDetailResponse{
		Transaction: detail,
	})
}

func (h *PaymentHandler) ProcessSettlement(orderID string, reference string) error {
	// Start Database Transaction
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Fetch Transaction with Idempotency Check
	transaction, err := h.TransactionRepo.FindByOrderAndReference(orderID, reference)
	if err != nil {
		return err
	}
	if transaction.Status == "success" {
		// Already processed
		return nil
	}

	// 2. Row Lock the Project (Race Condition Protection)
	project, err := h.ProjectRepo.FindByIDWithTx(tx, transaction.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to lock project: %v", err)
	}

	// 2a. Check Project Status
	if project.Status != "Aktif" {
		return fmt.Errorf("project is not active: %s", project.Status)
	}

	// 3. Calculate Amounts
	netAmount := transaction.Amount
	if transaction.TotalPayment == transaction.Amount {
		netAmount = transaction.Amount - transaction.Fee
	}

	beforeBalance := project.TotalTransaksi
	afterBalance := beforeBalance + netAmount

	// 4. Update Transaction Status
	if err := h.TransactionRepo.UpdateStatusWithTx(tx, orderID, reference, "success"); err != nil {
		return fmt.Errorf("failed to update tx status: %v", err)
	}

	// 5. Create Ledger Entry
	ledger := &models.Ledger{
		ProjectID:     project.ID,
		TransactionID: transaction.ID,
		Amount:        netAmount,
		Type:          "credit",
		Description:   fmt.Sprintf("Payment settlement for Order %s", orderID),
	}
	if err := h.LedgerRepo.CreateWithTx(tx, ledger); err != nil {
		return fmt.Errorf("failed to create ledger: %v", err)
	}

	// 6. Create Audit Log
	audit := &models.AuditLog{
		ProjectID:     project.ID,
		TransactionID: transaction.ID,
		BeforeBalance: beforeBalance,
		AfterBalance:  afterBalance,
		Amount:        netAmount,
		Type:          "credit",
	}
	if err := h.AuditLogRepo.CreateWithTx(tx, audit); err != nil {
		return fmt.Errorf("failed to create audit log: %v", err)
	}

	// 7. Update Project Balance
	if err := h.ProjectRepo.UpdateBalanceWithTx(tx, project.ID, afterBalance, project.SaldoTertunda); err != nil {
		return fmt.Errorf("failed to update project balance: %v", err)
	}

	// Commit Transaction
	return tx.Commit()
}

func (h *PaymentHandler) ReconcileTransactions(projectID uint) error {
	// This would fetch pending transactions from DB and check status via Duitku API
	// If successful in Duitku but pending locally, it calls ProcessSettlement
	fmt.Printf("[Reconciliation] Starting for Project %d\n", projectID)
	return nil
}

func (h *PaymentHandler) GetPaymentMethods(c *fiber.Ctx) error {
	amountStr := c.Query("amount")
	hasAmount := amountStr != ""
	var amount float64
	if hasAmount {
		fmt.Sscanf(amountStr, "%f", &amount)
	}

	var methods []models.PaymentMethod
	var err error

	project, ok := c.Locals("project").(*models.Project)
	if ok && project != nil {
		methods, err = h.PaymentMethodRepo.GetByProjectID(project.ID)
	} else {
		methods, err = h.PaymentMethodRepo.GetAllActive()
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil metode pembayaran"})
	}

	var methodItems []models.PaymentMethodItem
	for _, m := range methods {
		var totalFeePtr *float64
		if hasAmount {
			totalFee := m.FeeFlat + (amount * m.FeePercent)
			totalFeePtr = &totalFee
		}

		methodItems = append(methodItems, models.PaymentMethodItem{
			PaymentMethod: m.Code,
			PaymentName:   m.Name,
			PaymentImage:  m.ImageURL,
			TotalFee:      totalFeePtr,
			FeeFlat:       m.FeeFlat,
			FeePercent:    m.FeePercent,
		})
	}

	return c.JSON(models.PaymentMethodResponse{
		Methods: methodItems,
	})
}

func (h *PaymentHandler) PayByURL(c *fiber.Ctx) error {
	slug := c.Params("slug")
	amountStr := c.Params("amount")
	orderID := c.Query("order_id")
	redirect := c.Query("redirect")

	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)

	project, err := h.ProjectRepo.FindBySlug(slug)
	if err != nil {
		return c.Status(404).SendString("Proyek tidak ditemukan")
	}

	if project.Status != "Aktif" {
		return c.Status(403).SendString("Proyek ini sedang tidak aktif")
	}

	methods, err := h.PaymentMethodRepo.GetByProjectID(project.ID)
	if err != nil {
		return c.Status(500).SendString("Gagal mengambil metode pembayaran")
	}

	// Generate Premium HTML
	htmlTemplate := `<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pembayaran - {{PROJECT_NAME}}</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --primary: #6366f1;
            --primary-hover: #4f46e5;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text-main: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: 'Outfit', sans-serif; 
            background: var(--bg); 
            color: var(--text-main);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            width: 100%;
            max-width: 480px;
            background: var(--card-bg);
            border-radius: 24px;
            box-shadow: 0 20px 25px -5px rgb(0 0 0 / 0.1), 0 8px 10px -6px rgb(0 0 0 / 0.1);
            overflow: hidden;
            border: 1px solid var(--border);
        }
        .header {
            padding: 32px;
            background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
            color: white;
            text-align: center;
        }
        .header h1 { font-size: 24px; font-weight: 700; margin-bottom: 8px; }
        .header p { font-size: 14px; opacity: 0.9; }
        
        .summary {
            padding: 24px 32px;
            background: #f1f5f9;
            border-bottom: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .summary .amount { font-size: 20px; font-weight: 700; color: var(--primary); }
        .summary .order-id { font-size: 13px; color: var(--text-muted); }

        .content { padding: 32px; }
        .section-title { 
            font-size: 16px; 
            font-weight: 600; 
            margin-bottom: 20px; 
            color: var(--text-main);
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .section-title::before {
            content: '';
            display: block;
            width: 4px;
            height: 16px;
            background: var(--primary);
            border-radius: 2px;
        }

        .method-grid {
            display: grid;
            grid-template-columns: 1fr;
            gap: 12px;
        }
        .method-card {
            display: flex;
            align-items: center;
            padding: 16px;
            border: 1px solid var(--border);
            border-radius: 16px;
            cursor: pointer;
            transition: all 0.2s ease;
            text-decoration: none;
            color: inherit;
        }
        .method-card:hover {
            border-color: var(--primary);
            background: #f5f3ff;
            transform: translateY(-2px);
            box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1);
        }
        .method-card img {
            width: 48px;
            height: 48px;
            object-fit: contain;
            margin-right: 16px;
            filter: grayscale(0.2);
            transition: filter 0.2s;
        }
        .method-card:hover img { filter: grayscale(0); }
        .method-info { flex: 1; }
        .method-name { font-weight: 600; font-size: 15px; margin-bottom: 2px; }
        .method-fee { font-size: 12px; color: var(--text-muted); }
        .chevron { color: var(--border); transition: color 0.2s; }
        .method-card:hover .chevron { color: var(--primary); }

        .footer {
            padding: 20px;
            text-align: center;
            font-size: 12px;
            color: var(--text-muted);
            border-top: 1px solid var(--border);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <p>Checkout di</p>
            <h1>{{PROJECT_NAME}}</h1>
        </div>
        <div class="summary">
            <div>
                <div class="order-id">Order #{{ORDER_ID}}</div>
                <div class="amount">Rp {{TOTAL_AMOUNT}}</div>
            </div>
            <div style="text-align: right">
                <div class="order-id" style="font-size: 10px; text-transform: uppercase; letter-spacing: 0.05em">Total Tagihan</div>
            </div>
        </div>
        <div class="content">
            <div class="section-title">Pilih Metode Pembayaran</div>
            <div class="method-grid">
                {{METHOD_LIST}}
            </div>
        </div>
        <div class="footer">
            Powered by LinkBayar Payment Gateway
        </div>
    </div>
    <script>
        // Show error alert if exists
        const urlParams = new URLSearchParams(window.location.search);
        if (urlParams.has('error')) {
            alert("Gagal membuat transaksi: " + urlParams.get('error'));
            // Remove error from URL without refreshing
            const newUrl = window.location.pathname + window.location.search.replace(/[?&]error=[^&]+/, '').replace(/^&/, '?');
            window.history.replaceState({}, document.title, newUrl);
        }
    </script>
</body>
</html>`

	methodList := ""
	for _, m := range methods {
		fee := m.FeeFlat + (amount * m.FeePercent)
		if project.FeeByMerchant {
			fee = 0
		}
		execURL := "/pay/" + slug + "/" + amountStr + "/result?method=" + m.Code + "&order_id=" + orderID + "&redirect=" + redirect

		mCard := `<a href="{{EXEC_URL}}" class="method-card">
                    <img src="{{IMG_URL}}" alt="{{NAME}}">
                    <div class="method-info">
                        <div class="method-name">{{NAME}}</div>
                        <div class="method-fee">+ Biaya layanan Rp {{FEE}}</div>
                    </div>
                    <div class="chevron">
                        <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M7.5 15L12.5 10L7.5 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                    </div>
                </a>`
		mCard = strings.ReplaceAll(mCard, "{{EXEC_URL}}", execURL)
		mCard = strings.ReplaceAll(mCard, "{{IMG_URL}}", m.ImageURL)
		mCard = strings.ReplaceAll(mCard, "{{NAME}}", m.Name)
		mCard = strings.ReplaceAll(mCard, "{{FEE}}", formatRupiah(fee))
		methodList += mCard
	}

	html := htmlTemplate
	html = strings.ReplaceAll(html, "{{PROJECT_NAME}}", project.Nama)
	html = strings.ReplaceAll(html, "{{ORDER_ID}}", orderID)
	html = strings.ReplaceAll(html, "{{TOTAL_AMOUNT}}", formatRupiah(amount))
	html = strings.ReplaceAll(html, "{{METHOD_LIST}}", methodList)

	c.Type("html")
	return c.SendString(html)
}

func (h *PaymentHandler) PayByURLExec(c *fiber.Ctx) error {
	slug := c.Params("slug")
	amountStr := c.Params("amount")
	method := c.Query("method")
	orderID := c.Query("order_id")
	redirect := c.Query("redirect")

	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)

	project, err := h.ProjectRepo.FindBySlug(slug)
	if err != nil {
		return c.Status(404).SendString("Project not found")
	}

	// Check if transaction already exists
	tx, err := h.TransactionRepo.FindByProjectAndOrderID(project.ID, orderID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("[URL-PAY] Find error: %v\n", err)
	}

	var currentTx *models.Transaction
	var pm *models.PaymentMethod

	if tx != nil {
		currentTx = tx

		// If user wants to change payment method for existing pending transaction
		if currentTx.Status == "pending" && method != "" && method != currentTx.PaymentMethod {
			newPm, errP := h.PaymentMethodRepo.FindByCode(method)
			if errP == nil {
				fee := newPm.FeeFlat + (amount * newPm.FeePercent)
				// Generate a NEW unique gateway order ID for this change attempt
				newGatewayOrderID := fmt.Sprintf("P%d-%s-C%d", project.ID, orderID, time.Now().Unix())

				req := models.TransactionRequest{
					Project: project.Nama,
					OrderID: newGatewayOrderID,
					Amount:  amount,
					APIKey:  project.APIKey,
				}

				payment, errD := h.WijayaPay.CreateTransaction(project.Mode, newPm.GatewayCode, fee, req, project.FeeByMerchant)
				if errD == nil {
					errU := h.TransactionRepo.UpdatePaymentMethod(currentTx.ID, newGatewayOrderID, payment.Reference, payment.Fee, payment.TotalPayment, method, payment.PaymentNumber)
					if errU == nil {
						currentTx.GatewayOrderID = newGatewayOrderID
						currentTx.Reference = payment.Reference
						currentTx.Fee = payment.Fee
						currentTx.TotalPayment = payment.TotalPayment
						currentTx.PaymentMethod = method
						currentTx.PaymentNumber = payment.PaymentNumber
					}
				} else {
					// Redirect back with error message
					errMsg := url.QueryEscape(errD.Error())
					return c.Redirect("/pay/" + slug + "/" + amountStr + "?order_id=" + orderID + "&redirect=" + redirect + "&error=" + errMsg)
				}
			}
		}

		pm, _ = h.PaymentMethodRepo.FindByCode(currentTx.PaymentMethod)

		// If transaction exists but fee is 0 and it's pending, try to recalculate it
		if currentTx.Status == "pending" && currentTx.Fee == 0 && pm != nil {
			currentTx.Fee = pm.FeeFlat + (currentTx.Amount * pm.FeePercent)
		}
	} else {
		// Create new transaction
		pm, err = h.PaymentMethodRepo.FindByCode(method)
		if err != nil {
			return c.Status(400).SendString("Metode pembayaran tidak valid")
		}

		fee := pm.FeeFlat + (amount * pm.FeePercent)
		gatewayOrderID := fmt.Sprintf("P%d-%s", project.ID, orderID)

		req := models.TransactionRequest{
			Project: project.Nama,
			OrderID: gatewayOrderID,
			Amount:  amount,
			APIKey:  project.APIKey,
		}

		payment, err := h.WijayaPay.CreateTransaction(project.Mode, pm.GatewayCode, fee, req, project.FeeByMerchant)
		if err != nil {
			fmt.Printf("[URL-PAY] WijayaPay Create Error: %v\n", err)
			// Redirect back with error message as alert
			errMsg := url.QueryEscape(err.Error())
			return c.Redirect("/pay/" + slug + "/" + amountStr + "?order_id=" + orderID + "&redirect=" + redirect + "&error=" + errMsg)
		}

		currentTx = &models.Transaction{
			ProjectID:      project.ID,
			OrderID:        orderID,
			GatewayOrderID: gatewayOrderID,
			Reference:      payment.Reference,
			Amount:         amount,
			Fee:            payment.Fee,
			TotalPayment:   payment.TotalPayment,
			Status:         "pending",
			Mode:           project.Mode,
			PaymentMethod:  method,
			PaymentNumber:  payment.PaymentNumber,
			Jenis:          "url",
		}

		err = h.TransactionRepo.Create(currentTx)
		if err != nil {
			fmt.Printf("[URL-PAY] DB Create Error for %s: %v\n", orderID, err)
		} else {
			fmt.Printf("[URL-PAY] Transaction Created: %s (DB ID: %d)\n", orderID, currentTx.ID)
		}
	}

	// Calculate Dynamic HTML Parts
	isSuccess := currentTx.Status == "success"
	paymentLabel := "Nomor Virtual Account"
	if currentTx.PaymentMethod == "qris" {
		paymentLabel = "Scan kode QR di bawah ini"
	}

	var paymentInfoHTML string
	if currentTx.PaymentMethod == "qris" {
		paymentInfoHTML = `<div id="qrcode" class="qr-container"></div>
               <script>
                var typeNumber = 0;
                var errorCorrectionLevel = 'L';
                var qr = qrcode(typeNumber, errorCorrectionLevel);
                qr.addData('{{QR_DATA}}');
                qr.make();
                document.getElementById('qrcode').innerHTML = qr.createImgTag(6);
               </script>`
		paymentInfoHTML = strings.ReplaceAll(paymentInfoHTML, "{{QR_DATA}}", currentTx.PaymentNumber)
	} else {
		paymentInfoHTML = `<div class="payment-number" id="p_num">{{PAY_NUM}}</div>
           <button class="btn btn-outline" style="padding: 8px; font-size: 12px; width: auto; margin:0 auto;" onclick="copyText('{{PAY_NUM}}')">Salin Nomor</button>`
		paymentInfoHTML = strings.ReplaceAll(paymentInfoHTML, "{{PAY_NUM}}", currentTx.PaymentNumber)
	}

	var redirectHTML string
	if redirect != "" {
		redirectHTML = `<a href="{{REDIRECT_URL}}" class="btn">Kembali ke halaman merchant</a>`
		redirectHTML = strings.ReplaceAll(redirectHTML, "{{REDIRECT_URL}}", redirect)
	}

	backURL := "/pay/" + slug + "/" + amountStr + "?order_id=" + orderID + "&redirect=" + redirect
	backHTML := ""
	if !isSuccess {
		backHTML = `<a href="` + backURL + `" class="btn btn-outline">Ganti Metode Pembayaran</a>`
	}

	expiryStr := currentTx.CreatedAt.Add(1 * time.Hour).Format("02 Jan 2006, 15:04 WIB")

	// FIXED FEE LOGIC: Use explicit variables for display
	displayFee := 0.0
	totalToPay := currentTx.Amount

	// If merchant doesn't cover the fee, show it in the breakdown and add to total
	if !project.FeeByMerchant {
		displayFee = currentTx.Fee
		totalToPay = currentTx.Amount + currentTx.Fee
	}

	fmt.Printf("[URL-PAY] OrderID: %s, Subtotal: %f, Fee: %f, FeeByMerc: %v, DisplayFee: %f, Total: %f\n",
		currentTx.OrderID, currentTx.Amount, currentTx.Fee, project.FeeByMerchant, displayFee, totalToPay)

	htmlTemplate := `<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{TITLE}} - LinkBayar</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/qrcode-generator@1.4.4/qrcode.min.js"></script>
    <style>
        :root {
            --primary: #6366f1;
            --success: #10b981;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text-main: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: 'Outfit', sans-serif; 
            background: var(--bg); 
            color: var(--text-main);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            width: 100%;
            max-width: 440px;
            background: var(--card-bg);
            border-radius: 24px;
            box-shadow: 0 20px 25px -5px rgb(0 0 0 / 0.1);
            overflow: hidden;
            border: 1px solid var(--border);
        }
        .header {
            padding: 32px 24px;
            text-align: center;
            background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
            color: white;
        }
        .status-badge {
            display: inline-block;
            padding: 6px 14px;
            border-radius: 99px;
            font-size: 11px;
            font-weight: 700;
            margin-bottom: 12px;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            background: rgba(255,255,255,0.2);
            color: white;
            backdrop-filter: blur(4px);
        }

        .breakdown {
            padding: 24px 32px;
            background: #f8fafc;
            border-bottom: 1px solid var(--border);
        }
        .breakdown-item {
            display: flex;
            justify-content: space-between;
            margin-bottom: 8px;
            font-size: 14px;
            color: var(--text-muted);
        }
        .breakdown-item.total {
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px dashed var(--border);
            font-weight: 700;
            color: var(--primary);
            font-size: 18px;
        }

        .content { padding: 32px; text-align: center; }
        .method-display {
            margin-bottom: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            padding: 12px;
            background: #f1f5f9;
            border-radius: 12px;
        }
        .method-display img { width: 32px; height: 32px; object-fit: contain; }
        .method-display span { font-weight: 600; font-size: 15px; }

        .payment-box {
            background: #ffffff;
            border: 2px dashed #cbd5e1;
            border-radius: 16px;
            padding: 24px;
            margin-bottom: 24px;
        }
        .payment-number { 
            font-size: 24px; 
            font-weight: 700; 
            letter-spacing: 2px; 
            color: var(--text-main); 
            margin: 16px 0;
            word-break: break-all;
        }
        .qr-container { margin: 10px auto; max-width: 180px; }
        .qr-container img { width: 100%; height: auto; border-radius: 8px; }

        .success-state {
            padding: 40px 0;
        }
        .success-icon {
            width: 72px;
            height: 72px;
            background: var(--success);
            color: white;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0 auto 24px;
            box-shadow: 0 10px 15px -3px rgba(16, 185, 129, 0.4);
        }

        .btn {
            display: block;
            width: 100%;
            padding: 14px;
            background: var(--primary);
            color: white;
            text-decoration: none;
            border-radius: 12px;
            font-weight: 600;
            transition: all 0.2s;
            border: none;
            cursor: pointer;
            font-family: inherit;
        }
        .btn:hover { background: #4f46e5; transform: translateY(-1px); }
        .btn-outline {
            background: transparent;
            color: var(--primary);
            border: 1px solid var(--primary);
            margin-top: 12px;
        }
        .btn-outline:hover {
            color: white;
        }

        .footer-note { font-size: 12px; color: var(--text-muted); margin-top: 24px; }
    </style>
</head>
<body>
    <div class="container">
        {{PAGE_CONTENT}}
    </div>

    <script>
        function copyText(text) {
            navigator.clipboard.writeText(text).then(() => {
                alert('Berhasil disalin!');
            });
        }

        // Auto-refresh when success
        {{POLLING_SCRIPT}}
    </script>
</body>
</html>`

	var pageContent string
	if isSuccess {
		pageContent = `
        <div class="header" style="background: var(--success);">
            <div class="success-icon">
                <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
            </div>
            <h1 style="font-size: 22px;">Pembayaran Berhasil!</h1>
            <p style="font-size: 14px; opacity: 0.9; margin-top: 4px;">Terima kasih telah melakukan pembayaran</p>
        </div>
        <div class="content success-state">
            <div class="breakdown" style="border-radius: 16px; margin-bottom: 32px; text-align: left; border: 1px solid var(--border);">
                <div class="breakdown-item"><span>Project</span><span style="font-weight: 600;">{{PROJECT_NAME}}</span></div>
                <div class="breakdown-item"><span>No. Order</span><span>#{{ORDER_ID}}</span></div>
                <div class="breakdown-item"><span>Metode</span><span>{{METHOD_NAME}}</span></div>
                <div class="breakdown-item" style="margin-top: 12px; padding-top: 12px; border-top: 1px dashed var(--border);">
                    <span>Total Bayar</span>
                    <span style="color: var(--success); font-weight: 700; font-size: 18px;">Rp {{TOTAL_PAYMENT}}</span>
                </div>
            </div>
            {{REDIRECT_BUTTON}}
        </div>`
	} else {
		pageContent = `
        <div class="header">
            <div class="status-badge">Menunggu Pembayaran</div>
            <h1 style="font-size: 20px;">{{PROJECT_NAME}}</h1>
            <p style="font-size: 13px; opacity: 0.8; margin-top: 4px;">Order #{{ORDER_ID}}</p>
        </div>
        <div class="breakdown">
            <div class="breakdown-item">
                <span>Subtotal</span>
                <span>Rp {{AMOUNT}}</span>
            </div>
            <div class="breakdown-item">
                <span>Biaya Layanan</span>
                <span>Rp {{FEE}}</span>
            </div>
            <div class="breakdown-item total">
                <span>Total Tagihan</span>
                <span>Rp {{TOTAL_PAYMENT}}</span>
            </div>
        </div>
        <div class="content">
            <div class="method-display">
                {{METHOD_IMG_TAG}}
                <span>{{METHOD_NAME}}</span>
            </div>
            
            <div class="payment-box">
                <div style="font-size: 12px; color: var(--text-muted); margin-bottom: 8px;">{{PAYMENT_LABEL}}</div>
                {{PAYMENT_INFO}}
            </div>

            {{REDIRECT_BUTTON}}
            {{BACK_BUTTON}}

            <div class="footer-note">
                Bayar sebelum: <strong>{{EXPIRY}}</strong><br>
                <span style="font-size: 10px; opacity: 0.7;">Status akan otomatis diperbarui setelah pembayaran</span>
            </div>
        </div>`
	}

	html := strings.ReplaceAll(htmlTemplate, "{{PAGE_CONTENT}}", pageContent)
	html = strings.ReplaceAll(html, "{{TITLE}}", cond(isSuccess, "Pembayaran Berhasil", "Instruksi Pembayaran"))
	html = strings.ReplaceAll(html, "{{PROJECT_NAME}}", project.Nama)
	html = strings.ReplaceAll(html, "{{ORDER_ID}}", currentTx.OrderID)
	html = strings.ReplaceAll(html, "{{AMOUNT}}", formatRupiah(currentTx.Amount))
	html = strings.ReplaceAll(html, "{{FEE}}", formatRupiah(displayFee))
	html = strings.ReplaceAll(html, "{{TOTAL_PAYMENT}}", formatRupiah(totalToPay))
	html = strings.ReplaceAll(html, "{{METHOD_NAME}}", cond(pm != nil, pm.Name, currentTx.PaymentMethod))
	html = strings.ReplaceAll(html, "{{METHOD_IMG_TAG}}", cond(pm != nil, `<img src="`+pm.ImageURL+`" alt="Logo">`, ""))
	html = strings.ReplaceAll(html, "{{PAYMENT_LABEL}}", paymentLabel)
	html = strings.ReplaceAll(html, "{{PAYMENT_INFO}}", paymentInfoHTML)
	html = strings.ReplaceAll(html, "{{REDIRECT_BUTTON}}", redirectHTML)
	html = strings.ReplaceAll(html, "{{BACK_BUTTON}}", backHTML)
	html = strings.ReplaceAll(html, "{{EXPIRY}}", expiryStr)

	// Add Polling Script if pending
	pollingScript := ""
	if !isSuccess {
		pollingScript = `
        setInterval(function() {
            fetch('/pay/` + slug + `/status/` + orderID + `')
                .then(response => response.json())
                .then(data => {
                    if (data.status === 'success') {
                        window.location.reload();
                    }
                });
        }, 5000);`
	}
	html = strings.ReplaceAll(html, "{{POLLING_SCRIPT}}", pollingScript)

	c.Type("html")
	return c.SendString(html)
}

func (h *PaymentHandler) PayByURLStatus(c *fiber.Ctx) error {
	slug := c.Params("slug")
	orderID := c.Params("order_id")

	project, err := h.ProjectRepo.FindBySlug(slug)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}

	tx, err := h.TransactionRepo.FindByProjectAndOrderID(project.ID, orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	return c.JSON(fiber.Map{
		"status": tx.Status,
	})
}

func cond(c bool, t, f string) string {
	if c {
		return t
	}
	return f
}

func formatRupiah(amount float64) string {
	s := fmt.Sprintf("%.0f", amount)
	if len(s) <= 3 {
		return s
	}
	var res string
	for i, v := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			res += "."
		}
		res += string(v)
	}
	return res
}
