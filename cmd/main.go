package main

import (
	"log"
	"os"
	"payment_service/internal/config"
	"payment_service/internal/handlers"
	"payment_service/internal/middleware"
	"payment_service/internal/repository"
	"payment_service/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Initialize Database
	db := config.ConnectDB()
	defer db.Close()

	// Initialize Repositories
	projectRepo := repository.NewProjectRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	ledgerRepo := repository.NewLedgerRepository(db)
	auditLogRepo := repository.NewAuditLogRepository(db)
	paymentMethodRepo := repository.NewPaymentMethodRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	workerPool := services.NewWorkerPool(5) // 5 concurrent workers
	defer workerPool.Shutdown()

	emailService := services.NewEmailService()
	fonnteService := services.NewFonnteService()

	wijayapayService := services.NewWijayaPayService(
		services.WijayaPayConfig{
			MerchantCode: os.Getenv("WIJAYAPAY_MERCHANT_CODE"),
			APIKey:       os.Getenv("WIJAYAPAY_API_KEY"),
			BaseURL:      os.Getenv("WIJAYAPAY_BASE_URL"),
			AppURL:       os.Getenv("APP_URL"),
		},
	)

	paymentHandler := handlers.NewPaymentHandler(
		wijayapayService,
		transactionRepo,
		projectRepo,
		ledgerRepo,
		auditLogRepo,
		paymentMethodRepo,
		sessionRepo,
		workerPool,
		emailService,
		fonnteService,
		db,
	)

	app := fiber.New()
	app.Use(logger.New())

	// API Routes (Following documentation)
	api := app.Group("/api")

	// Apply Auth Middleware to all routes except webhook and health
	api.Use(middleware.AuthMiddleware(projectRepo))

	// C.2. Transaction create
	api.Post("/transactioncreate/:method", paymentHandler.CreateTransaction)

	// API Get Payment Methods
	api.Get("/get_metode_pembayaran", paymentHandler.GetPaymentMethods)

	// API Create Checkout Session (NEW)
	api.Post("/checkout-session", paymentHandler.CreateCheckoutSession)

	// C.4. Payment simulation
	api.Post("/paymentsimulation", paymentHandler.PaymentSimulation)

	// C.5. Transaction Cancel
	api.Post("/transactioncancel", paymentHandler.TransactionCancel)

	// E. Transaction Detail API
	api.Get("/transactiondetail", paymentHandler.TransactionDetail)

	// Webhook from WijayaPay
	app.Post("/webhook/wijayapay", paymentHandler.WijayaPayWebhook)

	// Webhook from Fonnte
	app.Post("/webhook/fonnte", paymentHandler.FonnteWebhook)

	// URL-based Integration (Integrasi Via URL - SESSION BASED)
	app.Get("/pay/:slug/:token", paymentHandler.PayBySession)
	app.Get("/pay/:slug/:token/result", paymentHandler.PayBySessionExec)
	app.Get("/pay/:slug/status/:order_id", paymentHandler.PayByURLStatus)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3005"
	}

	log.Fatal(app.Listen(":" + port))
}
