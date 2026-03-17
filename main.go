package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/handlers"
)

func main() {
	cfg := LoadConfig()

	// Connect to MongoDB
	if cfg.MongoURI != "" {
		db.Connect(cfg.MongoURI)
		defer db.Disconnect()
	} else {
		log.Println("WARNING: MONGODB_URI not set, database features will not work")
	}

	// Init OAuth and chat
	handlers.InitOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret)
	handlers.InitChat(cfg.OpenClawURL, cfg.OpenClawToken)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Erebus API",
		ErrorHandler: errorHandler,
	})

	// Global middleware
	app.Use(RequestLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     joinOrigins(cfg.CORSOrigins),
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Google-Token",
		AllowCredentials: true,
	}))

	// Public routes
	app.Get("/api/health", handlers.HealthCheck)
	app.Get("/api/capabilities", handlers.Capabilities)
	app.Post("/api/auth/google", handlers.GoogleLogin(cfg.JWTSecret))

	// Protected routes
	api := app.Group("/api", AuthMiddleware(cfg.JWTSecret))

	// Auth
	api.Get("/auth/me", handlers.GetMe)
	api.Post("/auth/refresh", handlers.RefreshToken(cfg.JWTSecret))

	// Clients
	api.Get("/clients", handlers.ListClients)
	api.Post("/clients", handlers.CreateClient)
	api.Get("/clients/:id", handlers.GetClient)
	api.Put("/clients/:id", handlers.UpdateClient)
	api.Delete("/clients/:id", handlers.DeleteClient)

	// Business Units
	api.Get("/clients/:clientId/units", handlers.ListBusinessUnits)
	api.Post("/clients/:clientId/units", handlers.CreateBusinessUnit)
	api.Put("/units/:id", handlers.UpdateBusinessUnit)
	api.Delete("/units/:id", handlers.DeleteBusinessUnit)

	// Projects
	api.Get("/projects", handlers.ListProjects)
	api.Post("/projects", handlers.CreateProject)
	api.Get("/projects/:id", handlers.GetProject)
	api.Put("/projects/:id", handlers.UpdateProject)
	api.Delete("/projects/:id", handlers.DeleteProject)
	api.Post("/projects/:id/convert-to-group", handlers.ConvertToGroup)
	api.Post("/projects/:id/move-to/:groupId", handlers.MoveToGroup)
	api.Post("/projects/:id/make-standalone", handlers.MakeStandalone)
	api.Get("/projects/:id/sub-projects", handlers.ListSubProjects)

	// Tasks
	api.Get("/projects/:projectId/tasks", handlers.ListTasks)
	api.Post("/projects/:projectId/tasks", handlers.CreateTask)
	api.Put("/tasks/:id", handlers.UpdateTask)
	api.Delete("/tasks/:id", handlers.DeleteTask)
	api.Post("/tasks/:id/claim", handlers.ClaimTask)

	// Ideas
	api.Get("/ideas", handlers.ListIdeas)
	api.Post("/ideas", handlers.CreateIdea)
	api.Get("/ideas/:id", handlers.GetIdea)
	api.Put("/ideas/:id", handlers.UpdateIdea)
	api.Delete("/ideas/:id", handlers.DeleteIdea)
	api.Post("/ideas/:id/research", handlers.AddResearch)
	api.Post("/ideas/:id/convert-to-project", handlers.ConvertIdeaToProject)

	// Chat
	api.Post("/chat/send", handlers.SendChat)
	api.Get("/chat/sessions", handlers.ListChatSessions)
	api.Post("/chat/sessions", handlers.CreateChatSession)
	api.Delete("/chat/sessions/:key", handlers.DeleteChatSession)

	// Email
	api.Get("/email/inbox", handlers.ListEmails)
	api.Get("/email/:id", handlers.ReadEmail)
	api.Post("/email/send", handlers.SendEmail)

	// Calendar
	api.Get("/calendar/today", handlers.TodayEvents)
	api.Get("/calendar/upcoming", handlers.UpcomingEvents)
	api.Post("/calendar/events", handlers.CreateEvent)

	// Hivemind
	api.Get("/activity", handlers.GetActivity)
	api.Post("/knowledge/search", handlers.KnowledgeSearch)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		app.Shutdown()
	}()

	log.Printf("Erebus API starting on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error(), "code": code})
}

func joinOrigins(origins []string) string {
	result := ""
	for i, o := range origins {
		if i > 0 {
			result += ","
		}
		result += o
	}
	return result
}
