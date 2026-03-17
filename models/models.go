package models

import (

)

// User represents an authenticated user from Google OAuth.
type User struct {
	ID      string `json:"id" bson:"_id,omitempty"`
	Email   string `json:"email" bson:"email"`
	Name    string `json:"name" bson:"name"`
	Picture string `json:"picture" bson:"picture"`
}

// Client represents a client/customer.
type Client struct {
	ID           string `json:"id" bson:"_id,omitempty"`
	Name         string `json:"name" bson:"name"`
	ContactEmail string `json:"contactEmail,omitempty" bson:"contactEmail,omitempty"`
	Phone        string `json:"phone,omitempty" bson:"phone,omitempty"`
	Company      string `json:"company,omitempty" bson:"company,omitempty"`
	Notes        string `json:"notes,omitempty" bson:"notes,omitempty"`
	Status       string `json:"status" bson:"status"`
	CreatedAt    string `json:"createdAt" bson:"createdAt"`
	UpdatedAt    string `json:"updatedAt" bson:"updatedAt"`
}

// BusinessUnit represents a division within a client.
type BusinessUnit struct {
	ID        string `json:"id" bson:"_id,omitempty"`
	ClientID  string `json:"clientId" bson:"clientId"`
	Name      string `json:"name" bson:"name"`
	Contact   string `json:"contact,omitempty" bson:"contact,omitempty"`
	Email     string `json:"email,omitempty" bson:"email,omitempty"`
	Notes     string `json:"notes,omitempty" bson:"notes,omitempty"`
	CreatedAt string `json:"createdAt" bson:"createdAt"`
	UpdatedAt string `json:"updatedAt" bson:"updatedAt"`
}

// PortEntry represents a port binding for a project.
type PortEntry struct {
	Port     int    `json:"port" bson:"port"`
	Service  string `json:"service" bson:"service"`
	Protocol string `json:"protocol" bson:"protocol"`
}

// Project represents a project or project group.
type Project struct {
	ID             string      `json:"id" bson:"_id,omitempty"`
	Name           string      `json:"name" bson:"name"`
	Description    string      `json:"description,omitempty" bson:"description,omitempty"`
	ClientID       string      `json:"clientId,omitempty" bson:"clientId,omitempty"`
	BusinessUnitID string      `json:"businessUnitId,omitempty" bson:"businessUnitId,omitempty"`
	ParentID       string      `json:"parentId,omitempty" bson:"parentId,omitempty"`
	IsGroup        bool        `json:"isGroup" bson:"isGroup"`
	Status         string      `json:"status" bson:"status"`
	Stack          string      `json:"stack,omitempty" bson:"stack,omitempty"`
	RepoURL        string      `json:"repoUrl,omitempty" bson:"repoUrl,omitempty"`
	Ports          []PortEntry `json:"ports,omitempty" bson:"ports,omitempty"`
	Priority       string      `json:"priority,omitempty" bson:"priority,omitempty"`
	Tags           []string    `json:"tags,omitempty" bson:"tags,omitempty"`
	StartDate      string      `json:"startDate,omitempty" bson:"startDate,omitempty"`
	DueDate        string      `json:"dueDate,omitempty" bson:"dueDate,omitempty"`
	CreatedAt      string      `json:"createdAt" bson:"createdAt"`
	UpdatedAt      string      `json:"updatedAt" bson:"updatedAt"`
}

// Task represents a task within a project.
type Task struct {
	ID             string `json:"id" bson:"_id,omitempty"`
	ProjectID      string `json:"projectId" bson:"projectId"`
	Title          string `json:"title" bson:"title"`
	Description    string `json:"description,omitempty" bson:"description,omitempty"`
	Status         string `json:"status" bson:"status"`
	Priority       string `json:"priority,omitempty" bson:"priority,omitempty"`
	AssignedTo     string `json:"assignedTo,omitempty" bson:"assignedTo,omitempty"`
	ClaimedBy      string `json:"claimedBy,omitempty" bson:"claimedBy,omitempty"`
	EstimatedHours float64 `json:"estimatedHours,omitempty" bson:"estimatedHours,omitempty"`
	DueDate        string `json:"dueDate,omitempty" bson:"dueDate,omitempty"`
	CompletedAt    string `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	CreatedAt      string `json:"createdAt" bson:"createdAt"`
	UpdatedAt      string `json:"updatedAt" bson:"updatedAt"`
}

// Idea represents an idea or concept being explored.
type Idea struct {
	ID          string          `json:"id" bson:"_id,omitempty"`
	Title       string          `json:"title" bson:"title"`
	Description string          `json:"description,omitempty" bson:"description,omitempty"`
	Status      string          `json:"status" bson:"status"`
	Category    string          `json:"category,omitempty" bson:"category,omitempty"`
	Priority    string          `json:"priority,omitempty" bson:"priority,omitempty"`
	Tags        []string        `json:"tags,omitempty" bson:"tags,omitempty"`
	Research    []ResearchEntry `json:"research,omitempty" bson:"research,omitempty"`
	ProjectID   string          `json:"projectId,omitempty" bson:"projectId,omitempty"`
	CreatedAt   string          `json:"createdAt" bson:"createdAt"`
	UpdatedAt   string          `json:"updatedAt" bson:"updatedAt"`
}

// ResearchEntry is a research note attached to an idea.
type ResearchEntry struct {
	Type      string `json:"type,omitempty" bson:"type,omitempty"`
	Title     string `json:"title,omitempty" bson:"title,omitempty"`
	Content   string `json:"content" bson:"content"`
	Source    string `json:"source,omitempty" bson:"source,omitempty"`
	Timestamp string `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
}

// ChatSession represents a chat session with OpenClaw.
type ChatSession struct {
	ID        string `json:"id" bson:"_id,omitempty"`
	Key       string `json:"key" bson:"key"`
	Title     string `json:"title" bson:"title"`
	Model     string `json:"model,omitempty" bson:"model,omitempty"`
	CreatedAt string `json:"createdAt" bson:"createdAt"`
	UpdatedAt string `json:"updatedAt" bson:"updatedAt"`
}

// ChatMessage represents a message in a chat session.
type ChatMessage struct {
	Role    string `json:"role" bson:"role"`
	Content string `json:"content" bson:"content"`
}

// ActivityEntry represents an activity log entry.
type ActivityEntry struct {
	ID         string `json:"id" bson:"_id,omitempty"`
	InstanceID string `json:"instanceId,omitempty" bson:"instanceId,omitempty"`
	Action     string `json:"action" bson:"action"`
	EntityType string `json:"entityType,omitempty" bson:"entityType,omitempty"`
	EntityID   string `json:"entityId,omitempty" bson:"entityId,omitempty"`
	Summary    string `json:"summary,omitempty" bson:"summary,omitempty"`
	Data       string `json:"data,omitempty" bson:"data,omitempty"`
	Timestamp  string `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
}

// Pagination helpers

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalCount int64       `json:"totalCount"`
	TotalPages int64       `json:"totalPages"`
}

// Message represents an internal message between users/agents.
type Message struct {
	ID        string `json:"id" bson:"_id,omitempty"`
	FromID    string `json:"fromId" bson:"fromId"`
	FromName  string `json:"fromName" bson:"fromName"`
	FromType  string `json:"fromType" bson:"fromType"`
	ToID      string `json:"toId" bson:"toId"`
	ToName    string `json:"toName" bson:"toName"`
	Content   string `json:"content" bson:"content"`
	Read      bool   `json:"read" bson:"read"`
	CreatedAt string `json:"createdAt" bson:"createdAt"`
}

// Member represents a unified user or agent entry.
type Member struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	LastSeen string `json:"lastSeen,omitempty"`
	Picture  string `json:"picture,omitempty"`
}

// Conversation represents a message thread summary.
type Conversation struct {
	MemberID      string  `json:"memberId"`
	MemberName    string  `json:"memberName"`
	MemberType    string  `json:"memberType"`
	LastMessage   string  `json:"lastMessage"`
	LastMessageAt string  `json:"lastMessageAt"`
	UnreadCount   int     `json:"unreadCount"`
}

// Error response

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}
