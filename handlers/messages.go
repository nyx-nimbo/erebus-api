package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SendMessage creates a new message.
func SendMessage(c *fiber.Ctx) error {
	var req struct {
		ToID    string `json:"toId"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if req.ToID == "" || req.Content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "toId and content are required", "code": 400})
	}

	fromEmail, _ := c.Locals("email").(string)
	fromName, _ := c.Locals("name").(string)

	// Look up recipient name
	toName := resolveRecipientName(req.ToID)

	msg := models.Message{
		ID:        primitive.NewObjectID().Hex(),
		FromID:    fromEmail,
		FromName:  fromName,
		FromType:  "user",
		ToID:      req.ToID,
		ToName:    toName,
		Content:   req.Content,
		Read:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("messages").InsertOne(ctx, msg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to send message", "code": 500})
	}

	// Forward to WebSocket server for real-time delivery
	go forwardToWS(msg)

	return c.Status(201).JSON(msg)
}

// GetConversation returns messages between the current user and another user/agent.
func GetConversation(c *fiber.Ctx) error {
	withID := c.Query("with")
	if withID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "with parameter is required", "code": 400})
	}

	currentUser, _ := c.Locals("email").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"$or": []bson.M{
			{"fromId": currentUser, "toId": withID},
			{"fromId": withID, "toId": currentUser},
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cursor, err := db.Collection("messages").Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch messages", "code": 500})
	}
	defer cursor.Close(ctx)

	var messages []models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode messages", "code": 500})
	}
	if messages == nil {
		messages = []models.Message{}
	}

	return c.JSON(fiber.Map{"data": messages})
}

// ListConversations returns all conversations for the current user, grouped by other party.
func ListConversations(c *fiber.Ctx) error {
	currentUser, _ := c.Locals("email").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all messages involving this user
	filter := bson.M{
		"$or": []bson.M{
			{"fromId": currentUser},
			{"toId": currentUser},
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := db.Collection("messages").Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch conversations", "code": 500})
	}
	defer cursor.Close(ctx)

	var messages []models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode messages", "code": 500})
	}

	// Group by other party
	convMap := make(map[string]*models.Conversation)
	for _, msg := range messages {
		var otherID, otherName, otherType string
		if msg.FromID == currentUser {
			otherID = msg.ToID
			otherName = msg.ToName
			otherType = "user" // default, could be agent
		} else {
			otherID = msg.FromID
			otherName = msg.FromName
			otherType = msg.FromType
		}

		if _, exists := convMap[otherID]; !exists {
			convMap[otherID] = &models.Conversation{
				MemberID:      otherID,
				MemberName:    otherName,
				MemberType:    otherType,
				LastMessage:   msg.Content,
				LastMessageAt: msg.CreatedAt,
				UnreadCount:   0,
			}
		}

		// Count unread messages from this person to current user
		if msg.ToID == currentUser && !msg.Read {
			convMap[otherID].UnreadCount++
		}
	}

	conversations := make([]models.Conversation, 0, len(convMap))
	for _, conv := range convMap {
		conversations = append(conversations, *conv)
	}

	return c.JSON(fiber.Map{"data": conversations})
}

// MarkRead marks a message as read.
func MarkRead(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("messages").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to mark message as read", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Message not found", "code": 404})
	}

	return c.JSON(fiber.Map{"message": "Message marked as read"})
}

// GetUnreadCount returns the count of unread messages for the current user.
func GetUnreadCount(c *fiber.Ctx) error {
	currentUser, _ := c.Locals("email").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := db.Collection("messages").CountDocuments(ctx, bson.M{
		"toId": currentUser,
		"read": false,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count unread messages", "code": 500})
	}

	return c.JSON(fiber.Map{"count": count})
}

// resolveRecipientName looks up the name of a recipient by their ID (email for users, _id for agents).
func resolveRecipientName(id string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try users first (by email)
	var user bson.M
	err := db.Collection("users").FindOne(ctx, bson.M{"email": id}).Decode(&user)
	if err == nil {
		if name, ok := user["name"].(string); ok {
			return name
		}
	}

	// Try agents (by _id)
	var agent bson.M
	err = db.Collection("agents").FindOne(ctx, bson.M{"_id": id}).Decode(&agent)
	if err == nil {
		if name, ok := agent["name"].(string); ok {
			return name
		}
	}

	return id
}

// forwardToWS sends a message to the WebSocket server for real-time delivery
func forwardToWS(msg models.Message) {
	wsURL := os.Getenv("EREBUS_WS_URL")
	if wsURL == "" {
		wsURL = "https://ws-erebus.nimbo.pro"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "erebus-jwt-secret-v1-change-me"
	}

	// Generate a service JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": msg.FromID,
		"name":  msg.FromName,
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	})
	tokenStr, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Printf("WS forward: JWT error: %v", err)
		return
	}

	payload := map[string]string{
		"toId":    msg.ToID,
		"content": msg.Content,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", wsURL+"/api/send", bytes.NewReader(body))
	if err != nil {
		log.Printf("WS forward: request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WS forward: send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("WS forward: status %d: %s", resp.StatusCode, string(respBody))
	}
}
