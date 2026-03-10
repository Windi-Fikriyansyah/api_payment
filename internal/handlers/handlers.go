package handlers

import (
	"bytes"
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
	Duitku          *services.DuitkuService
	TransactionRepo *repository.TransactionRepository
}

func NewPaymentHandler(duitku *services.DuitkuService, transactionRepo *repository.TransactionRepository) *PaymentHandler {
	return &PaymentHandler{
		Duitku:          duitku,
		TransactionRepo: transactionRepo,
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

	payment, err := h.Duitku.CreateTransaction(project.Mode, method, req)
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

	// Update record if success (resultCode "00")
	if payload.ResultCode == "00" {
		err = h.TransactionRepo.UpdateStatus(payload.MerchantOrderId, "success")
		if err != nil {
			fmt.Printf("Database Error updating status for %s: %v\n", payload.MerchantOrderId, err)
		} else {
			fmt.Printf("Transaction %s marked as SUCCESS\n", payload.MerchantOrderId)
		}

		// Fetch project data for webhook URL
		project, err := h.TransactionRepo.FindProjectByTransactionOrderID(payload.MerchantOrderId)
		if err == nil && project.WebhookURL != "" {
			fmt.Printf("Forwarding callback to project webhook: %s\n", project.WebhookURL)
			go h.SendCallback(project.WebhookURL, models.WebhookPayload{
				Amount:        tx.Amount,
				OrderID:       tx.OrderID,
				Project:       project.Nama,
				Status:        "success",
				PaymentMethod: tx.PaymentMethod,
				CompletedAt:   time.Now(),
			})
		} else if err != nil {
			fmt.Printf("Webhook Error: Could not find project for order %s\n", payload.MerchantOrderId)
		}
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

	err = h.TransactionRepo.UpdateStatus(req.OrderID, "success")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update status"})
	}

	project, err := h.TransactionRepo.FindProjectByTransactionOrderID(req.OrderID)
	if err == nil && project.WebhookURL != "" {
		go h.SendCallback(project.WebhookURL, models.WebhookPayload{
			Amount:        tx.Amount,
			OrderID:       tx.OrderID,
			Project:       project.Nama,
			Status:        "success",
			PaymentMethod: tx.PaymentMethod,
			CompletedAt:   time.Now(),
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

	// Update transaction status in database
	err = h.TransactionRepo.UpdateStatus(req.OrderID, "cancelled")
	if err != nil {
		fmt.Printf("Database Error updating cancel status for %s: %v\n", req.OrderID, err)
	}

	// Send webhook callback to project
	if project.WebhookURL != "" {
		go h.SendCallback(project.WebhookURL, models.WebhookPayload{
			Amount:        tx.Amount,
			OrderID:       tx.OrderID,
			Project:       project.Nama,
			Status:        "cancelled",
			PaymentMethod: tx.PaymentMethod,
			CompletedAt:   time.Now(),
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
