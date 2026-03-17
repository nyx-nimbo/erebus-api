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

// ListProjects returns top-level projects (parentId=null).
func ListProjects(c *fiber.Ctx) error {
	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := db.Collection("projects")
	filter := bson.M{}

	// Optional: filter by parentId query param
	if pid := c.Query("parentId"); pid != "" {
		filter["parentId"] = pid
	} else if c.Query("topLevel") == "true" {
		filter["$or"] = []bson.M{
			{"parentId": nil},
			{"parentId": ""},
			{"parentId": bson.M{"$exists": false}},
		}
	}

	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count projects", "code": 500})
	}

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch projects", "code": 500})
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err := cursor.All(ctx, &projects); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode projects", "code": 500})
	}
	if projects == nil {
		projects = []models.Project{}
	}

	return c.JSON(models.PaginatedResponse{
		Data:       projects,
		Page:       page,
		Limit:      limit,
		TotalCount: total,
		TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
	})
}

// CreateProject creates a new project.
func CreateProject(c *fiber.Ctx) error {
	var project models.Project
	if err := c.BodyParser(&project); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if project.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required", "code": 400})
	}

	project.ID = primitive.NewObjectID().Hex()
	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)
	if project.Status == "" {
		project.Status = "active"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("projects").InsertOne(ctx, project)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create project", "code": 500})
	}

	logActivity(c, "create", "project", project.ID, "Created project: "+project.Name)
	return c.Status(201).JSON(project)
}

// GetProject returns a project with its sub-projects.
func GetProject(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var project models.Project
	err := db.Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&project)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	// Fetch sub-projects
	cursor, err := db.Collection("projects").Find(ctx, bson.M{"parentId": id},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return c.JSON(fiber.Map{"project": project, "subProjects": []models.Project{}})
	}
	defer cursor.Close(ctx)

	var subs []models.Project
	cursor.All(ctx, &subs)
	if subs == nil {
		subs = []models.Project{}
	}

	return c.JSON(fiber.Map{"project": project, "subProjects": subs})
}

// UpdateProject updates a project by ID.
func UpdateProject(c *fiber.Ctx) error {
	id := c.Params("id")

	var updates bson.M
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	delete(updates, "_id")
	delete(updates, "id")
	delete(updates, "createdAt")
	updates["updatedAt"] = time.Now().Format(time.RFC3339)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("projects").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update project", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	var project models.Project
	db.Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&project)

	logActivity(c, "update", "project", id, "Updated project")
	return c.JSON(project)
}

// DeleteProject deletes a project and its sub-projects and tasks.
func DeleteProject(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("projects").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete project", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	// Cascade delete sub-projects and tasks
	db.Collection("projects").DeleteMany(ctx, bson.M{"parentId": id})
	db.Collection("tasks").DeleteMany(ctx, bson.M{"projectId": id})

	logActivity(c, "delete", "project", id, "Deleted project")
	return c.JSON(fiber.Map{"message": "Project deleted"})
}

// ConvertToGroup converts a project into a group.
func ConvertToGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("projects").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"isGroup": true, "updatedAt": time.Now().Format(time.RFC3339)},
	})
	if err != nil || result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	var project models.Project
	db.Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&project)

	logActivity(c, "update", "project", id, "Converted to group")
	return c.JSON(project)
}

// MoveToGroup moves a project under a group.
func MoveToGroup(c *fiber.Ctx) error {
	id := c.Params("id")
	groupID := c.Params("groupId")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify group exists and is a group
	var group models.Project
	err := db.Collection("projects").FindOne(ctx, bson.M{"_id": groupID}).Decode(&group)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Group not found", "code": 404})
	}

	result, err := db.Collection("projects").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"parentId": groupID, "updatedAt": time.Now().Format(time.RFC3339)},
	})
	if err != nil || result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	var project models.Project
	db.Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&project)

	logActivity(c, "update", "project", id, "Moved to group: "+group.Name)
	return c.JSON(project)
}

// MakeStandalone removes a project from its parent group.
func MakeStandalone(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("projects").UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set":   bson.M{"updatedAt": time.Now().Format(time.RFC3339)},
		"$unset": bson.M{"parentId": ""},
	})
	if err != nil || result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found", "code": 404})
	}

	var project models.Project
	db.Collection("projects").FindOne(ctx, bson.M{"_id": id}).Decode(&project)

	logActivity(c, "update", "project", id, "Made standalone")
	return c.JSON(project)
}

// ListSubProjects returns sub-projects for a parent project.
func ListSubProjects(c *fiber.Ctx) error {
	id := c.Params("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.Collection("projects").Find(ctx, bson.M{"parentId": id},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch sub-projects", "code": 500})
	}
	defer cursor.Close(ctx)

	var subs []models.Project
	cursor.All(ctx, &subs)
	if subs == nil {
		subs = []models.Project{}
	}

	return c.JSON(fiber.Map{"data": subs})
}
