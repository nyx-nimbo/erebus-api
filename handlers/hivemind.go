package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetActivity returns recent activity log entries.
func GetActivity(c *fiber.Ctx) error {
	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := db.Collection("activity").Find(ctx, bson.M{}, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch activity", "code": 500})
	}
	defer cursor.Close(ctx)

	var entries []models.ActivityEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode activity", "code": 500})
	}
	if entries == nil {
		entries = []models.ActivityEntry{}
	}

	return c.JSON(fiber.Map{"data": entries})
}

// KnowledgeSearch performs a text search across collections.
func KnowledgeSearch(c *fiber.Ctx) error {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.BodyParser(&req); err != nil || req.Query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Query is required", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Search across projects, tasks, ideas, and activity using regex
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": req.Query, "$options": "i"}},
			{"title": bson.M{"$regex": req.Query, "$options": "i"}},
			{"description": bson.M{"$regex": req.Query, "$options": "i"}},
		},
	}

	var results []map[string]interface{}

	collections := []struct {
		name     string
		category string
	}{
		{"projects", "project"},
		{"tasks", "task"},
		{"ideas", "idea"},
		{"clients", "client"},
	}

	for _, col := range collections {
		cursor, err := db.Collection(col.name).Find(ctx, filter,
			options.Find().SetLimit(10).SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
		if err != nil {
			continue
		}

		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)

		for _, doc := range docs {
			results = append(results, map[string]interface{}{
				"category": col.category,
				"data":     doc,
			})
		}
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return c.JSON(fiber.Map{"results": results, "query": req.Query})
}

// logActivity records an activity entry (used by other handlers).
func logActivity(c *fiber.Ctx, action, entityType, entityID, summary string) {
	entry := models.ActivityEntry{
		ID:         primitive.NewObjectID().Hex(),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    summary,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db.Collection("activity").InsertOne(ctx, entry)
}
