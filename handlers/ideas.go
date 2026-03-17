package handlers

import (
	"context"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListIdeas returns all ideas with optional status filter.
func ListIdeas(c *fiber.Ctx) error {
	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}

	coll := db.Collection("ideas")

	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count ideas", "code": 500})
	}

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch ideas", "code": 500})
	}
	defer cursor.Close(ctx)

	var ideas []models.Idea
	if err := cursor.All(ctx, &ideas); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode ideas", "code": 500})
	}
	if ideas == nil {
		ideas = []models.Idea{}
	}

	return c.JSON(models.PaginatedResponse{
		Data:       ideas,
		Page:       page,
		Limit:      limit,
		TotalCount: total,
		TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
	})
}

// CreateIdea creates a new idea.
func CreateIdea(c *fiber.Ctx) error {
	var idea models.Idea
	if err := c.BodyParser(&idea); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if idea.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required", "code": 400})
	}

	idea.ID = primitive.NewObjectID()
	idea.CreatedAt = time.Now()
	idea.UpdatedAt = time.Now()
	if idea.Status == "" {
		idea.Status = "new"
	}
	if idea.Research == nil {
		idea.Research = []models.ResearchEntry{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("ideas").InsertOne(ctx, idea)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create idea", "code": 500})
	}

	logActivity(c, "create", "idea", idea.ID.Hex(), "Created idea: "+idea.Title)
	return c.Status(201).JSON(idea)
}

// GetIdea returns a single idea with its research entries.
func GetIdea(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var idea models.Idea
	err = db.Collection("ideas").FindOne(ctx, bson.M{"_id": id}).Decode(&idea)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Idea not found", "code": 404})
	}

	return c.JSON(idea)
}

// UpdateIdea updates an idea by ID.
func UpdateIdea(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	var updates bson.M
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	delete(updates, "_id")
	delete(updates, "id")
	delete(updates, "createdAt")
	updates["updatedAt"] = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("ideas").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update idea", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Idea not found", "code": 404})
	}

	var idea models.Idea
	db.Collection("ideas").FindOne(ctx, bson.M{"_id": id}).Decode(&idea)

	logActivity(c, "update", "idea", id.Hex(), "Updated idea")
	return c.JSON(idea)
}

// DeleteIdea deletes an idea by ID.
func DeleteIdea(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("ideas").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete idea", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Idea not found", "code": 404})
	}

	logActivity(c, "delete", "idea", id.Hex(), "Deleted idea")
	return c.JSON(fiber.Map{"message": "Idea deleted"})
}

// AddResearch adds a research entry to an idea.
func AddResearch(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	var entry models.ResearchEntry
	if err := c.BodyParser(&entry); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if entry.Content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Content is required", "code": 400})
	}

	entry.ID = primitive.NewObjectID()
	entry.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("ideas").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$push": bson.M{"research": entry},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add research", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Idea not found", "code": 404})
	}

	logActivity(c, "create", "research", id.Hex(), "Added research to idea")
	return c.Status(201).JSON(entry)
}

// ConvertIdeaToProject converts an idea into a project.
func ConvertIdeaToProject(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var idea models.Idea
	err = db.Collection("ideas").FindOne(ctx, bson.M{"_id": id}).Decode(&idea)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Idea not found", "code": 404})
	}

	project := models.Project{
		ID:          primitive.NewObjectID(),
		Name:        idea.Title,
		Description: idea.Description,
		Status:      "active",
		Tags:        idea.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err = db.Collection("projects").InsertOne(ctx, project)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create project", "code": 500})
	}

	// Mark idea as converted
	db.Collection("ideas").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"status": "converted", "updatedAt": time.Now()},
	})

	logActivity(c, "convert", "idea", id.Hex(), "Converted idea to project: "+project.Name)
	return c.Status(201).JSON(fiber.Map{"project": project, "idea": idea})
}
