package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents an authenticated user from Google OAuth.
type User struct {
	ID      primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email   string             `json:"email" bson:"email"`
	Name    string             `json:"name" bson:"name"`
	Picture string             `json:"picture" bson:"picture"`
}

// Client represents a client/customer.
type Client struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name"`
	Email     string             `json:"email,omitempty" bson:"email,omitempty"`
	Phone     string             `json:"phone,omitempty" bson:"phone,omitempty"`
	Company   string             `json:"company,omitempty" bson:"company,omitempty"`
	Notes     string             `json:"notes,omitempty" bson:"notes,omitempty"`
	Status    string             `json:"status" bson:"status"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// BusinessUnit represents a division within a client.
type BusinessUnit struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ClientID  primitive.ObjectID `json:"clientId" bson:"clientId"`
	Name      string             `json:"name" bson:"name"`
	Contact   string             `json:"contact,omitempty" bson:"contact,omitempty"`
	Email     string             `json:"email,omitempty" bson:"email,omitempty"`
	Notes     string             `json:"notes,omitempty" bson:"notes,omitempty"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// Project represents a project or project group.
type Project struct {
	ID          primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	Name        string              `json:"name" bson:"name"`
	Description string              `json:"description,omitempty" bson:"description,omitempty"`
	ClientID    *primitive.ObjectID `json:"clientId,omitempty" bson:"clientId,omitempty"`
	ParentID    *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty"`
	IsGroup     bool                `json:"isGroup" bson:"isGroup"`
	Status      string              `json:"status" bson:"status"`
	Priority    string              `json:"priority,omitempty" bson:"priority,omitempty"`
	Tags        []string            `json:"tags,omitempty" bson:"tags,omitempty"`
	StartDate   *time.Time          `json:"startDate,omitempty" bson:"startDate,omitempty"`
	DueDate     *time.Time          `json:"dueDate,omitempty" bson:"dueDate,omitempty"`
	CreatedAt   time.Time           `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt" bson:"updatedAt"`
}

// Task represents a task within a project.
type Task struct {
	ID          primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	ProjectID   primitive.ObjectID  `json:"projectId" bson:"projectId"`
	Title       string              `json:"title" bson:"title"`
	Description string              `json:"description,omitempty" bson:"description,omitempty"`
	Status      string              `json:"status" bson:"status"`
	Priority    string              `json:"priority,omitempty" bson:"priority,omitempty"`
	AssignedTo  string              `json:"assignedTo,omitempty" bson:"assignedTo,omitempty"`
	ClaimedBy   string              `json:"claimedBy,omitempty" bson:"claimedBy,omitempty"`
	DueDate     *time.Time          `json:"dueDate,omitempty" bson:"dueDate,omitempty"`
	ParentID    *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty"`
	CreatedAt   time.Time           `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt" bson:"updatedAt"`
}

// Idea represents an idea or concept being explored.
type Idea struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	Status      string             `json:"status" bson:"status"`
	Category    string             `json:"category,omitempty" bson:"category,omitempty"`
	Tags        []string           `json:"tags,omitempty" bson:"tags,omitempty"`
	Research    []ResearchEntry    `json:"research,omitempty" bson:"research,omitempty"`
	CreatedAt   time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// ResearchEntry is a research note attached to an idea.
type ResearchEntry struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Content   string             `json:"content" bson:"content"`
	Source    string             `json:"source,omitempty" bson:"source,omitempty"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}

// ChatSession represents a chat session with OpenClaw.
type ChatSession struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Key       string             `json:"key" bson:"key"`
	Title     string             `json:"title" bson:"title"`
	Model     string             `json:"model,omitempty" bson:"model,omitempty"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" bson:"updatedAt"`
}

// ChatMessage represents a message in a chat session.
type ChatMessage struct {
	Role    string `json:"role" bson:"role"`
	Content string `json:"content" bson:"content"`
}

// ActivityEntry represents an activity log entry.
type ActivityEntry struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Action    string             `json:"action" bson:"action"`
	Entity    string             `json:"entity" bson:"entity"`
	EntityID  string             `json:"entityId,omitempty" bson:"entityId,omitempty"`
	Details   string             `json:"details,omitempty" bson:"details,omitempty"`
	User      string             `json:"user" bson:"user"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}

// Pagination helpers

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalCount int64       `json:"totalCount"`
	TotalPages int64       `json:"totalPages"`
}

// Error response

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}
