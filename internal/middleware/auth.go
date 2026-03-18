package middleware

import (
	"fmt"
	"payment_service/internal/repository"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(repo *repository.ProjectRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		// Support Authorization Header (Bearer token)
		if apiKey == "" {
			authHeader := c.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Fallback: Check body if it's a JSON request
		if apiKey == "" && (c.Method() == "POST" || c.Method() == "PUT") {
			var body struct {
				APIKey string `json:"api_key"`
			}
			_ = c.BodyParser(&body)
			apiKey = body.APIKey
		}

		// Clean the API Key
		apiKey = strings.TrimSpace(apiKey)
		apiKey = strings.Trim(apiKey, "\"")

		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API Key is required",
			})
		}

		fmt.Printf("[Auth] Receiving Request: %s %s | API Key: '%s'\n", c.Method(), c.Path(), apiKey)

		project, err := repo.FindByAPIKey(apiKey)
		if err != nil {
			fmt.Printf("Auth Error: %v for API Key: %s\n", err, apiKey)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API Key",
			})
		}

		if project.Status != "Aktif" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Project is not active. Current status: " + project.Status,
			})
		}

		// Store project in context for later use
		c.Locals("project", project)

		return c.Next()
	}
}
