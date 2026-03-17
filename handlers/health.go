package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
)

func HealthCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dbStatus := "connected"
	if err := db.GetClient().Ping(ctx, nil); err != nil {
		dbStatus = "disconnected"
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"service":  "erebus-api",
		"database": dbStatus,
		"time":     time.Now().UTC(),
	})
}

func Capabilities(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"service": "erebus-api",
		"version": "1.0.0",
		"endpoints": []string{
			"auth", "clients", "projects", "tasks",
			"ideas", "chat", "email", "calendar",
			"health", "activity", "knowledge",
		},
		"features": fiber.Map{
			"streaming":   true,
			"pagination":  true,
			"oauth":       "google",
			"chatBackend": "openclaw",
		},
	})
}
