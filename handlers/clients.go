package handlers

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nyx-nimbo/erebus-api/db"
	"github.com/nyx-nimbo/erebus-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ListClients returns paginated clients.
func ListClients(c *fiber.Ctx) error {
	page, limit := parsePagination(c)
	skip := int64((page - 1) * limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := db.Collection("clients")

	total, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count clients", "code": 500})
	}

	opts := options.Find().SetSkip(skip).SetLimit(int64(limit)).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch clients", "code": 500})
	}
	defer cursor.Close(ctx)

	var clients []models.Client
	if err := cursor.All(ctx, &clients); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode clients", "code": 500})
	}
	if clients == nil {
		clients = []models.Client{}
	}

	return c.JSON(models.PaginatedResponse{
		Data:       clients,
		Page:       page,
		Limit:      limit,
		TotalCount: total,
		TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
	})
}

// CreateClient creates a new client.
func CreateClient(c *fiber.Ctx) error {
	var client models.Client
	if err := c.BodyParser(&client); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if client.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required", "code": 400})
	}

	client.ID = primitive.NewObjectID()
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()
	if client.Status == "" {
		client.Status = "active"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Collection("clients").InsertOne(ctx, client)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create client", "code": 500})
	}

	logActivity(c, "create", "client", client.ID.Hex(), "Created client: "+client.Name)
	return c.Status(201).JSON(client)
}

// GetClient returns a single client by ID.
func GetClient(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var client models.Client
	err = db.Collection("clients").FindOne(ctx, bson.M{"_id": id}).Decode(&client)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Client not found", "code": 404})
	}

	return c.JSON(client)
}

// UpdateClient updates a client by ID.
func UpdateClient(c *fiber.Ctx) error {
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

	result, err := db.Collection("clients").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update client", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Client not found", "code": 404})
	}

	var client models.Client
	db.Collection("clients").FindOne(ctx, bson.M{"_id": id}).Decode(&client)

	logActivity(c, "update", "client", id.Hex(), "Updated client")
	return c.JSON(client)
}

// DeleteClient deletes a client by ID.
func DeleteClient(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("clients").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete client", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Client not found", "code": 404})
	}

	// Also delete associated business units
	db.Collection("business_units").DeleteMany(ctx, bson.M{"clientId": id})

	logActivity(c, "delete", "client", id.Hex(), "Deleted client")
	return c.JSON(fiber.Map{"message": "Client deleted"})
}

// ListBusinessUnits returns business units for a client.
func ListBusinessUnits(c *fiber.Ctx) error {
	clientID, err := primitive.ObjectIDFromHex(c.Params("clientId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid client ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.Collection("business_units").Find(ctx, bson.M{"clientId": clientID},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch units", "code": 500})
	}
	defer cursor.Close(ctx)

	var units []models.BusinessUnit
	if err := cursor.All(ctx, &units); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode units", "code": 500})
	}
	if units == nil {
		units = []models.BusinessUnit{}
	}

	return c.JSON(fiber.Map{"data": units})
}

// CreateBusinessUnit creates a business unit under a client.
func CreateBusinessUnit(c *fiber.Ctx) error {
	clientID, err := primitive.ObjectIDFromHex(c.Params("clientId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid client ID", "code": 400})
	}

	var unit models.BusinessUnit
	if err := c.BodyParser(&unit); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}
	if unit.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required", "code": 400})
	}

	unit.ID = primitive.NewObjectID()
	unit.ClientID = clientID
	unit.CreatedAt = time.Now()
	unit.UpdatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = db.Collection("business_units").InsertOne(ctx, unit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create unit", "code": 500})
	}

	logActivity(c, "create", "business_unit", unit.ID.Hex(), "Created business unit: "+unit.Name)
	return c.Status(201).JSON(unit)
}

// UpdateBusinessUnit updates a business unit by ID.
func UpdateBusinessUnit(c *fiber.Ctx) error {
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
	delete(updates, "clientId")
	delete(updates, "createdAt")
	updates["updatedAt"] = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("business_units").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update unit", "code": 500})
	}
	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Unit not found", "code": 404})
	}

	var unit models.BusinessUnit
	db.Collection("business_units").FindOne(ctx, bson.M{"_id": id}).Decode(&unit)

	return c.JSON(unit)
}

// DeleteBusinessUnit deletes a business unit by ID.
func DeleteBusinessUnit(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID", "code": 400})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.Collection("business_units").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete unit", "code": 500})
	}
	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Unit not found", "code": 404})
	}

	return c.JSON(fiber.Map{"message": "Business unit deleted"})
}

// Helpers

func parsePagination(c *fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
