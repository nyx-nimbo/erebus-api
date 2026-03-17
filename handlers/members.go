package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ListUsers returns all users from the users collection.
func ListUsers(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users", "code": 500})
	}
	defer cursor.Close(ctx)

	var users []bson.M
	if err := cursor.All(ctx, &users); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode users", "code": 500})
	}
	if users == nil {
		users = []bson.M{}
	}

	return c.JSON(fiber.Map{"data": users})
}

// ListAgents returns all agents from the agents collection.
func ListAgents(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.Collection("agents").Find(ctx, bson.M{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch agents", "code": 500})
	}
	defer cursor.Close(ctx)

	var agents []bson.M
	if err := cursor.All(ctx, &agents); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode agents", "code": 500})
	}
	if agents == nil {
		agents = []bson.M{}
	}

	return c.JSON(fiber.Map{"data": agents})
}

// ListMembers returns a combined list of all users and agents as unified members.
func ListMembers(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var members []models.Member

	// Fetch users
	userCursor, err := db.Collection("users").Find(ctx, bson.M{})
	if err == nil {
		var users []bson.M
		if err := userCursor.All(ctx, &users); err == nil {
			for _, u := range users {
				m := models.Member{
					ID:   getStringField(u, "_id"),
					Name: getStringField(u, "name"),
					Email: getStringField(u, "email"),
					Type:   "user",
					Status: "offline",
					Picture: getStringField(u, "picture"),
				}
				if v, ok := u["lastSeen"]; ok {
					if s, ok := v.(string); ok {
						m.LastSeen = s
					}
				}
				if v, ok := u["updatedAt"]; ok {
					if t, ok := v.(time.Time); ok {
						m.LastSeen = t.Format(time.RFC3339)
					}
				}
				members = append(members, m)
			}
		}
		userCursor.Close(ctx)
	}

	// Fetch agents
	agentCursor, err := db.Collection("agents").Find(ctx, bson.M{})
	if err == nil {
		var agents []bson.M
		if err := agentCursor.All(ctx, &agents); err == nil {
			for _, a := range agents {
				status := "offline"
				if v, ok := a["status"]; ok {
					if s, ok := v.(string); ok && s != "" {
						status = s
					}
				}
				m := models.Member{
					ID:     getStringField(a, "_id"),
					Name:   getStringField(a, "name"),
					Type:   "agent",
					Status: status,
				}
				if v, ok := a["lastSeen"]; ok {
					if s, ok := v.(string); ok {
						m.LastSeen = s
					}
				}
				if v, ok := a["lastHeartbeat"]; ok {
					if s, ok := v.(string); ok {
						m.LastSeen = s
					}
				}
				members = append(members, m)
			}
		}
		agentCursor.Close(ctx)
	}

	if members == nil {
		members = []models.Member{}
	}

	return c.JSON(fiber.Map{"data": members})
}

func getStringField(doc bson.M, key string) string {
	if v, ok := doc[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case primitive.ObjectID:
			return val.Hex()
		default:
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}
