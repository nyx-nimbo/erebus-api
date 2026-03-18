package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// createTestProject is a helper that creates a project and returns its ID.
func createTestProject(t *testing.T, app *fiber.App, token, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name})
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	id, ok := result["id"].(string)
	if !ok {
		t.Fatalf("failed to get project id from response: %s", string(respBody))
	}
	return id
}

func TestCreateTask(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Task Test Project")

	body, _ := json.Marshal(map[string]string{
		"title":       "My Test Task",
		"description": "Task description",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["title"] != "My Test Task" {
		t.Errorf("expected title 'My Test Task', got %v", result["title"])
	}
	if result["projectId"] != projectID {
		t.Errorf("expected projectId '%s', got %v", projectID, result["projectId"])
	}
	if result["status"] != "todo" {
		t.Errorf("expected default status 'todo', got %v", result["status"])
	}
}

func TestCreateTaskFlat(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Flat Task Project")

	body, _ := json.Marshal(map[string]string{
		"title":     "Flat Task",
		"projectId": projectID,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["title"] != "Flat Task" {
		t.Errorf("expected title 'Flat Task', got %v", result["title"])
	}
	if result["projectId"] != projectID {
		t.Errorf("expected projectId '%s', got %v", projectID, result["projectId"])
	}
}

func TestListTasks(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "List Tasks Project")

	// Create two tasks
	for _, title := range []string{"Task 1", "Task 2"} {
		body, _ := json.Marshal(map[string]string{"title": title})
		req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		resp.Body.Close()
	}

	// List tasks for project
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 tasks, got %d", len(data))
	}
}

func TestListAllTasks(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "All Tasks Project")

	// Create a task
	body, _ := json.Marshal(map[string]string{"title": "A Global Task"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := app.Test(createReq, -1)
	createResp.Body.Close()

	// List all tasks
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) < 1 {
		t.Errorf("expected at least 1 task, got %d", len(data))
	}
}

func TestGetTask(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Get Task Project")

	// Create a task
	body, _ := json.Marshal(map[string]string{"title": "Findable Task"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	taskID := created["id"].(string)

	// Get the task
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["title"] != "Findable Task" {
		t.Errorf("expected title 'Findable Task', got %v", result["title"])
	}
	if result["id"] != taskID {
		t.Errorf("expected id '%s', got %v", taskID, result["id"])
	}
}

func TestUpdateTask(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Update Task Project")

	// Create a task
	body, _ := json.Marshal(map[string]string{"title": "Old Title"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	taskID := created["id"].(string)

	// Update the task
	updateBody, _ := json.Marshal(map[string]string{
		"title":  "New Title",
		"status": "in_progress",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/tasks/"+taskID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["title"] != "New Title" {
		t.Errorf("expected title 'New Title', got %v", result["title"])
	}
	if result["status"] != "in_progress" {
		t.Errorf("expected status 'in_progress', got %v", result["status"])
	}
}

func TestDeleteTask(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Delete Task Project")

	// Create a task
	body, _ := json.Marshal(map[string]string{"title": "Delete Me Task"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	taskID := created["id"].(string)

	// Delete
	req := httptest.NewRequest(http.MethodDelete, "/api/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["message"] != "Task deleted" {
		t.Errorf("expected message 'Task deleted', got %v", result["message"])
	}

	// Verify gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, _ := app.Test(getReq, -1)
	getResp.Body.Close()

	if getResp.StatusCode != 404 {
		t.Errorf("expected 404 after deletion, got %d", getResp.StatusCode)
	}
}

func TestClaimTask(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() {
		cleanupCollection("projects")
		cleanupCollection("tasks")
		cleanupCollection("activity")
	})

	projectID := createTestProject(t, app, token, "Claim Task Project")

	// Create a task
	body, _ := json.Marshal(map[string]string{"title": "Claimable Task"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	taskID := created["id"].(string)

	// Claim the task
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+taskID+"/claim", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["claimedBy"] != "test@example.com" {
		t.Errorf("expected claimedBy 'test@example.com', got %v", result["claimedBy"])
	}
	if result["status"] != "in_progress" {
		t.Errorf("expected status 'in_progress', got %v", result["status"])
	}
}
