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

// ListTasks returns tasks for a given project.
func ListTasks(c *fiber.Ctx) error {
	projectID := c.Params("projectId")

	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := db.Collection("tasks")
	filter := bson.M{"projectId": projectID}

	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count tasks", "code": 500})
	}

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tasks", "code": 500})
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode tasks", "code": 500})
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	return c.JSON(models.PaginatedResponse{
		Data:       tasks,
		Page:       page,
		Limit:      limit,
		TotalCount: total,
		TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
	})
}

// CreateTask creates a task under a project.
func CreateTask(c *fiber.Ctx) error {
	projectID := c.Params("projectId")

	var task models.Task
	if err := c.BodyParser(&task); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if task.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required", "code": 400})
	}

	task.ID = primitive.NewObjectID().Hex()
	task.ProjectID = projectID
	task.CreatedAt = time.Now().Format(time.RFC3339)
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	if task.Status == "" {
		task.Status = "todo"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("tasks").InsertOne(ctx, task)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task", "code": 500})
	}

	logActivity(c, "create", "task", task.ID, "Created task: "+task.Title)
	return c.Status(201).JSON(task)
}

// GetTask returns a single task by ID.
func GetTask(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var task models.Task
	err := db.Collection("tasks").FindOne(ctx, bson.M{"_id": id}).Decode(&task)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found", "code": 404})
	}

	return c.JSON(task)
}

// UpdateTask updates a task by ID.
func UpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var updates bson.M
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	delete(updates, "_id")
	delete(updates, "id")
	delete(updates, "projectId")
	delete(updates, "createdAt")
	updates["updatedAt"] = time.Now().Format(time.RFC3339)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("tasks").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update task", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found", "code": 404})
	}

	var task models.Task
	db.Collection("tasks").FindOne(ctx, bson.M{"_id": id}).Decode(&task)

	logActivity(c, "update", "task", id, "Updated task")
	return c.JSON(task)
}

// DeleteTask deletes a task by ID.
func DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("tasks").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete task", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found", "code": 404})
	}

	logActivity(c, "delete", "task", id, "Deleted task")
	return c.JSON(fiber.Map{"message": "Task deleted"})
}

// ClaimTask assigns the current user to a task.
func ClaimTask(c *fiber.Ctx) error {
	id := c.Params("id")

	email, _ := c.Locals("email").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("tasks").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"claimedBy": email,
			"status":    "in_progress",
			"updatedAt": time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to claim task", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Task not found", "code": 404})
	}

	var task models.Task
	db.Collection("tasks").FindOne(ctx, bson.M{"_id": id}).Decode(&task)

	logActivity(c, "claim", "task", id, "Claimed task")
	return c.JSON(task)
}

// ListAllTasks returns all tasks, optionally filtered by projectId query param.
func ListAllTasks(c *fiber.Ctx) error {
	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := db.Collection("tasks")
	filter := bson.M{}

	// Optional projectId filter
	if pid := c.Query("projectId"); pid != "" {
		filter["projectId"] = pid
	}

	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count tasks", "code": 500})
	}

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tasks", "code": 500})
	}
	defer cursor.Close(ctx)

	var tasks []models.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode tasks", "code": 500})
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	return c.JSON(fiber.Map{
		"data":       tasks,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": int(math.Ceil(float64(total) / float64(limit))),
	})
}

// CreateTaskFlat creates a task with projectId in the body instead of URL param.
func CreateTaskFlat(c *fiber.Ctx) error {
	var task models.Task
	if err := c.BodyParser(&task); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	task.ID = primitive.NewObjectID().Hex()
	task.CreatedAt = time.Now().Format(time.RFC3339)
	task.UpdatedAt = time.Now().Format(time.RFC3339)
	if task.Status == "" {
		task.Status = "todo"
	}

	coll := db.Collection("tasks")
	if _, err := coll.InsertOne(ctx, task); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task", "code": 500})
	}

	return c.Status(201).JSON(task)
}
