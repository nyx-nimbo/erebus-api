package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	openClawURL   string
	openClawToken string
)

func InitChat(url, token string) {
	openClawURL = url
	openClawToken = token
}

type chatSendRequest struct {
	SessionKey string               `json:"sessionKey"`
	Message    string               `json:"message"`
	Model      string               `json:"model,omitempty"`
	Messages   []models.ChatMessage `json:"messages,omitempty"`
}

// SendChat proxies a chat completion request to OpenClaw as SSE.
func SendChat(c *fiber.Ctx) error {
	var req chatSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if req.Message == "" && len(req.Messages) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Message or messages required", "code": 400})
	}

	messages := req.Messages
	if len(messages) == 0 {
		messages = []models.ChatMessage{{Role: "user", Content: req.Message}}
	}

	payload := map[string]interface{}{
		"messages": messages,
		"stream":   true,
	}
	if req.Model != "" {
		payload["model"] = req.Model
	}

	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequest("POST", openClawURL, bytes.NewReader(body))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create request", "code": 500})
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if openClawToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+openClawToken)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to reach chat backend", "code": 502})
	}
	defer resp.Body.Close()

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(w, "%s\n", line)
			w.Flush()
		}
	})

	return nil
}

// ListChatSessions returns all chat sessions.
func ListChatSessions(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.Collection("chat_sessions").Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch sessions", "code": 500})
	}
	defer cursor.Close(ctx)

	var sessions []models.ChatSession
	if err := cursor.All(ctx, &sessions); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode sessions", "code": 500})
	}
	if sessions == nil {
		sessions = []models.ChatSession{}
	}

	return c.JSON(fiber.Map{"data": sessions})
}

// CreateChatSession creates a new chat session.
func CreateChatSession(c *fiber.Ctx) error {
	var session models.ChatSession
	if err := c.BodyParser(&session); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if session.Key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Session key is required", "code": 400})
	}

	session.ID = primitive.NewObjectID()
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("chat_sessions").InsertOne(ctx, session)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create session", "code": 500})
	}

	return c.Status(201).JSON(session)
}

// DeleteChatSession deletes a chat session by key.
func DeleteChatSession(c *fiber.Ctx) error {
	key := c.Params("key")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("chat_sessions").DeleteOne(ctx, bson.M{"key": key})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete session", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found", "code": 404})
	}

	return c.JSON(fiber.Map{"message": "Session deleted"})
}
