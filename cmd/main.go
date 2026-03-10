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

	duitkuService := services.NewDuitkuService(
		services.DuitkuConfig{
			MerchantCode: os.Getenv("DUITKU_MERCHANT_CODE"),
			APIKey:       os.Getenv("DUITKU_API_KEY"),
			BaseURL:      os.Getenv("DUITKU_BASE_URL"),
		},
		services.DuitkuConfig{
			MerchantCode: os.Getenv("DUITKU_PROD_MERCHANT_CODE"),
			APIKey:       os.Getenv("DUITKU_PROD_API_KEY"),
			BaseURL:      os.Getenv("DUITKU_PROD_BASE_URL"),
		},
	)
	paymentHandler := handlers.NewPaymentHandler(duitkuService, transactionRepo)

	app := fiber.New()
	app.Use(logger.New())

	// API Routes (Following documentation)
	api := app.Group("/api")

	// Apply Auth Middleware to all routes except webhook and health
	api.Use(middleware.AuthMiddleware(projectRepo))

	// C.2. Transaction create
	api.Post("/transactioncreate/:method", paymentHandler.CreateTransaction)

	// C.4. Payment simulation
	api.Post("/paymentsimulation", paymentHandler.PaymentSimulation)

	// C.5. Transaction Cancel
	api.Post("/transactioncancel", paymentHandler.TransactionCancel)

	// E. Transaction Detail API
	api.Get("/transactiondetail", paymentHandler.TransactionDetail)

	// Webhook from Duitku
	app.Post("/webhook/duitku", paymentHandler.DuitkuWebhook)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("Payment Service is UP")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))
}
