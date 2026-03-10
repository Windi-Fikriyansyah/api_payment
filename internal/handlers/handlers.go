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
	"time"

	"github.com/gofiber/fiber/v2"
)

type PaymentHandler struct {
	Duitku            *services.DuitkuService
	TransactionRepo   *repository.TransactionRepository
	ProjectRepo       *repository.ProjectRepository
	LedgerRepo        *repository.LedgerRepository
	AuditLogRepo      *repository.AuditLogRepository
	PaymentMethodRepo *repository.PaymentMethodRepository
	WorkerPool        *services.WorkerPool
	DB                *sql.DB
}

func NewPaymentHandler(
	duitku *services.DuitkuService,
	transactionRepo *repository.TransactionRepository,
	projectRepo *repository.ProjectRepository,
	ledgerRepo *repository.LedgerRepository,
	auditLogRepo *repository.AuditLogRepository,
	paymentMethodRepo *repository.PaymentMethodRepository,
	workerPool *services.WorkerPool,
	db *sql.DB,
) *PaymentHandler {
	return &PaymentHandler{
		Duitku:            duitku,
		TransactionRepo:   transactionRepo,
		ProjectRepo:       projectRepo,
		LedgerRepo:        ledgerRepo,
		AuditLogRepo:      auditLogRepo,
		PaymentMethodRepo: paymentMethodRepo,
		WorkerPool:        workerPool,
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
	req.Project = project.Nama

	// Fetch Payment Method from DB
	pm, err := h.PaymentMethodRepo.FindByCode(method)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Metode pembayaran tidak valid"})
	}

	// Calculate Fee based on DB
	fee := pm.FeeFlat + (req.Amount * pm.FeePercent)

	payment, err := h.Duitku.CreateTransaction(project.Mode, pm.DuitkuCode, fee, req, project.FeeByMerchant)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	payment.ExpiredAt = time.Now().Add(24 * time.Hour)

	transaction := &models.Transaction{
		ProjectID:     project.ID,
		OrderID:       req.OrderID,
		Reference:     payment.Reference,
		Amount:        req.Amount,
		Fee:           payment.Fee,
		TotalPayment:  payment.TotalPayment,
		Status:        "pending",
		Mode:          project.Mode,
		PaymentMethod: method,
		PaymentNumber: payment.PaymentNumber,
	}

	err = h.TransactionRepo.Create(transaction)
	if err != nil {
		fmt.Printf("Database Error saving transaction: %v\n", err)
	}

	return c.Status(200).JSON(models.PaymentResponse{
		Payment: *payment,
	})
}

func (h *PaymentHandler) DuitkuWebhook(c *fiber.Ctx) error {
	// Duitku can send as Form or JSON
	var payload struct {
		MerchantCode    string `json:"merchantCode" form:"merchantCode"`
		Amount          string `json:"amount" form:"amount"`
		MerchantOrderId string `json:"merchantOrderId" form:"merchantOrderId"`
		Signature       string `json:"signature" form:"signature"`
		ResultCode      string `json:"resultCode" form:"resultCode"`
	}

	if err := c.BodyParser(&payload); err != nil {
		// If BodyParser fails, try manual form values
		payload.MerchantCode = c.FormValue("merchantCode")
		payload.Amount = c.FormValue("amount")
		payload.MerchantOrderId = c.FormValue("merchantOrderId")
		payload.Signature = c.FormValue("signature")
		payload.ResultCode = c.FormValue("resultCode")
	}

	fmt.Printf("Incoming Webhook: OrderID=%s, Amount=%s, Result=%s\n",
		payload.MerchantOrderId, payload.Amount, payload.ResultCode)

	// Verify Signature
	err := h.Duitku.VerifyCallback(payload.MerchantOrderId, payload.Amount, payload.Signature)
	if err != nil {
		fmt.Printf("Webhook Signature Invalid: %v\n", err)
		return c.Status(400).SendString("Invalid signature")
	}

	// Fetch transaction to see if it exists
	tx, err := h.TransactionRepo.FindByOrderID(payload.MerchantOrderId)
	if err != nil {
		fmt.Printf("Webhook Error: Transaction %s not found in database\n", payload.MerchantOrderId)
		return c.Status(200).SendString("OK")
	}

	// 3. Process Settlement Atomically
	if payload.ResultCode == "00" {
		err = h.ProcessSettlement(payload.MerchantOrderId)
		if err != nil {
			fmt.Printf("Settlement Error for %s: %v\n", payload.MerchantOrderId, err)
			return c.Status(500).SendString("Internal Error")
		}

		fmt.Printf("Transaction %s processed successfully\n", payload.MerchantOrderId)

		// 4. Forward Callback asynchronously
		tx, _ = h.TransactionRepo.FindByOrderID(payload.MerchantOrderId)
		project, _ := h.TransactionRepo.FindProjectByTransactionOrderID(payload.MerchantOrderId)

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
	} else {
		// Failure or Pending
		// We can update status to failed here if it's not success
		// For simplicity, let's just log
		fmt.Printf("Transaction %s failed with code %s\n", payload.MerchantOrderId, payload.ResultCode)
	}

	return c.Status(200).SendString("OK")
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

	tx, err := h.TransactionRepo.FindByOrderID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	if tx.Status == "success" {
		return c.Status(200).JSON(fiber.Map{"message": "Transaction already paid"})
	}

	err = h.ProcessSettlement(req.OrderID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to settle: " + err.Error()})
	}

	project, _ := h.TransactionRepo.FindProjectByTransactionOrderID(req.OrderID)
	if err == nil && project.WebhookURL != "" {
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

	// Find transaction in database
	tx, err := h.TransactionRepo.FindByOrderID(req.OrderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Transaction not found"})
	}

	// Validate transaction is in a cancellable state
	if tx.Status != "pending" {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("Transaction cannot be cancelled, current status: %s", tx.Status),
		})
	}

	// Call Duitku cancel API
	err = h.Duitku.CancelTransaction(project.Mode, req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update transaction status in database atomically (partial transaction for status change)
	dtx, dtxe := h.DB.Begin()
	if dtxe == nil {
		h.TransactionRepo.UpdateStatusWithTx(dtx, req.OrderID, "cancelled")
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

	tx, err := h.TransactionRepo.FindByOrderID(orderID)
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

func (h *PaymentHandler) ProcessSettlement(orderID string) error {
	// Start Database Transaction
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Fetch Transaction with Idempotency Check
	transaction, err := h.TransactionRepo.FindByOrderID(orderID)
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
	if err := h.TransactionRepo.UpdateStatusWithTx(tx, orderID, "success"); err != nil {
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
	amountStr := c.Query("amount", "0")
	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)

	methods, err := h.PaymentMethodRepo.GetAllActive()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil metode pembayaran"})
	}

	var methodItems []models.PaymentMethodItem
	for _, m := range methods {
		totalFee := m.FeeFlat + (amount * m.FeePercent)
		methodItems = append(methodItems, models.PaymentMethodItem{
			PaymentMethod: m.Code,
			PaymentName:   m.Name,
			PaymentImage:  m.ImageURL,
			TotalFee:      totalFee,
		})
	}

	return c.JSON(models.PaymentMethodResponse{
		Methods: methodItems,
	})
}
