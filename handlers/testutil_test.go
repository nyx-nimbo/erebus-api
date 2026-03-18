package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nyx-nimbo/erebus-api/db"
)

const testJWTSecret = "test-secret-key"

// TestMain sets up the test database connection and runs all tests.
func TestMain(m *testing.M) {
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		log.Println("TEST_MONGODB_URI not set, skipping integration tests")
		os.Exit(0)
	}

	db.Connect(uri)
	db.SetDBName("nyx_test")

	// Initialize handlers that need config
	InitOAuth("test-client-id", "test-client-secret")
	InitChat("http://localhost:0/v1/chat/completions", "")

	code := m.Run()

	// Cleanup: drop the test database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.GetClient().Database("nyx_test").Drop(ctx); err != nil {
		log.Printf("Failed to drop test database: %v", err)
	}

	db.Disconnect()
	os.Exit(code)
}

// testAuthMiddleware replicates the AuthMiddleware from main package for test use.
func testAuthMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Missing authorization header", "code": 401})
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		if tokenStr == auth {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization format", "code": 401})
		}

		token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token", "code": 401})
		}

		claims, ok := token.Claims.(*jwt.MapClaims)
		if !ok || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token claims", "code": 401})
		}

		mc := *claims
		c.Locals("email", mc["email"])
		c.Locals("name", mc["name"])
		c.Locals("picture", mc["picture"])
		return c.Next()
	}
}

// setupTestApp creates a Fiber app with all routes registered, matching main.go.
func setupTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "Erebus API Test",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error(), "code": code})
		},
	})

	// Public routes
	app.Get("/api/health", HealthCheck)
	app.Get("/api/capabilities", Capabilities)
	app.Post("/api/auth/google", GoogleLogin(testJWTSecret))

	// Protected routes
	api := app.Group("/api", testAuthMiddleware(testJWTSecret))

	// Auth
	api.Get("/auth/me", GetMe)
	api.Post("/auth/refresh", RefreshToken(testJWTSecret))
	api.Post("/auth/google/connect", GoogleConnectURL)
	api.Post("/auth/google/callback", GoogleCallback)
	api.Get("/auth/google/status", GoogleConnectionStatus)
	api.Post("/auth/google/disconnect", GoogleDisconnect)

	// Clients
	api.Get("/clients", ListClients)
	api.Post("/clients", CreateClient)
	api.Get("/clients/:id", GetClient)
	api.Put("/clients/:id", UpdateClient)
	api.Delete("/clients/:id", DeleteClient)

	// Business Units
	api.Get("/clients/:clientId/units", ListBusinessUnits)
	api.Post("/clients/:clientId/units", CreateBusinessUnit)
	api.Put("/units/:id", UpdateBusinessUnit)
	api.Delete("/units/:id", DeleteBusinessUnit)

	// Projects
	api.Get("/projects", ListProjects)
	api.Post("/projects", CreateProject)
	api.Get("/projects/:id", GetProject)
	api.Put("/projects/:id", UpdateProject)
	api.Delete("/projects/:id", DeleteProject)
	api.Post("/projects/:id/convert-to-group", ConvertToGroup)
	api.Post("/projects/:id/move-to/:groupId", MoveToGroup)
	api.Post("/projects/:id/make-standalone", MakeStandalone)
	api.Get("/projects/:id/sub-projects", ListSubProjects)

	// Tasks
	api.Get("/tasks", ListAllTasks)
	api.Post("/tasks", CreateTaskFlat)
	api.Get("/tasks/:id", GetTask)
	api.Get("/projects/:projectId/tasks", ListTasks)
	api.Post("/projects/:projectId/tasks", CreateTask)
	api.Put("/tasks/:id", UpdateTask)
	api.Delete("/tasks/:id", DeleteTask)
	api.Post("/tasks/:id/claim", ClaimTask)

	// Ideas
	api.Get("/ideas", ListIdeas)
	api.Post("/ideas", CreateIdea)
	api.Get("/ideas/:id", GetIdea)
	api.Put("/ideas/:id", UpdateIdea)
	api.Delete("/ideas/:id", DeleteIdea)
	api.Post("/ideas/:id/research", AddResearch)
	api.Post("/ideas/:id/convert-to-project", ConvertIdeaToProject)

	// Chat
	api.Post("/chat/send", SendChat)
	api.Get("/chat/sessions", ListChatSessions)
	api.Get("/chat/sessions/:key", GetChatSession)
	api.Post("/chat/sessions", CreateChatSession)
	api.Delete("/chat/sessions/:key", DeleteChatSession)

	// Email
	api.Get("/email/inbox", ListEmails)
	api.Get("/email/:id", ReadEmail)
	api.Post("/email/send", SendEmail)

	// Calendar
	api.Get("/calendar/today", TodayEvents)
	api.Get("/calendar/upcoming", UpcomingEvents)
	api.Post("/calendar/events", CreateEvent)

	// Members
	api.Get("/users", ListUsers)
	api.Get("/agents", ListAgents)
	api.Get("/members", ListMembers)

	// Messages
	api.Post("/messages", SendMessage)
	api.Get("/messages", GetConversation)
	api.Get("/messages/conversations", ListConversations)
	api.Get("/messages/unread", GetUnreadCount)
	api.Put("/messages/:id/read", MarkRead)
	api.Delete("/messages/conversation", DeleteConversation)

	// Hivemind
	api.Get("/activity", GetActivity)
	api.Post("/knowledge/search", KnowledgeSearch)

	return app
}

// generateTestJWT creates a valid JWT token for testing.
func generateTestJWT(email, name string) string {
	claims := jwt.MapClaims{
		"email":   email,
		"name":    name,
		"picture": "https://example.com/photo.jpg",
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		panic("failed to generate test JWT: " + err.Error())
	}
	return tokenStr
}

// cleanupCollection drops a collection to reset state between tests.
func cleanupCollection(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db.Collection(name).Drop(ctx)
}
