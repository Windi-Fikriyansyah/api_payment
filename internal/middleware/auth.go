package middleware

import (
	"fmt"
	"payment_service/internal/repository"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(repo *repository.ProjectRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API Key is required",
			})
		}

		project, err := repo.FindByAPIKey(apiKey)
		if err != nil {
			fmt.Printf("Auth Error: %v for API Key: %s\n", err, apiKey)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API Key",
			})
		}

		// Store project in context for later use
		c.Locals("project", project)

		return c.Next()
	}
}
