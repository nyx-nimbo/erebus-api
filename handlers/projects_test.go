package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProject(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "Test Project",
		"description": "A test project",
		"stack":       "Go + MongoDB",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
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

	if result["name"] != "Test Project" {
		t.Errorf("expected name 'Test Project', got %v", result["name"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected id in response")
	}
	if result["status"] != "active" {
		t.Errorf("expected default status 'active', got %v", result["status"])
	}
}

func TestListProjects(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create two projects
	for _, name := range []string{"Project A", "Project B"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		resp.Body.Close()
	}

	// List projects
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
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
		t.Errorf("expected at least 2 projects, got %d", len(data))
	}
}

func TestGetProject(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a project
	body, _ := json.Marshal(map[string]string{"name": "Get Test Project"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	projectID := created["id"].(string)

	// Get the project
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID, nil)
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

	// GetProject returns {project: ..., subProjects: ...}
	project, ok := result["project"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'project' key in response")
	}
	if project["name"] != "Get Test Project" {
		t.Errorf("expected name 'Get Test Project', got %v", project["name"])
	}

	subProjects, ok := result["subProjects"].([]interface{})
	if !ok {
		t.Fatal("expected 'subProjects' key in response")
	}
	if len(subProjects) != 0 {
		t.Errorf("expected 0 sub-projects, got %d", len(subProjects))
	}
}

func TestUpdateProject(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a project
	body, _ := json.Marshal(map[string]string{"name": "Original Project"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	projectID := created["id"].(string)

	// Update
	updateBody, _ := json.Marshal(map[string]string{
		"name":        "Updated Project",
		"description": "Updated desc",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID, bytes.NewReader(updateBody))
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

	if result["name"] != "Updated Project" {
		t.Errorf("expected name 'Updated Project', got %v", result["name"])
	}
}

func TestDeleteProject(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a project
	body, _ := json.Marshal(map[string]string{"name": "Delete Me Project"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	projectID := created["id"].(string)

	// Delete
	req := httptest.NewRequest(http.MethodDelete, "/api/projects/"+projectID, nil)
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

	if result["message"] != "Project deleted" {
		t.Errorf("expected message 'Project deleted', got %v", result["message"])
	}

	// Verify gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, _ := app.Test(getReq, -1)
	getResp.Body.Close()

	if getResp.StatusCode != 404 {
		t.Errorf("expected 404 after deletion, got %d", getResp.StatusCode)
	}
}

func TestConvertToGroup(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a project
	body, _ := json.Marshal(map[string]string{"name": "To Be Group"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	projectID := created["id"].(string)

	// Convert to group
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/convert-to-group", nil)
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

	if result["isGroup"] != true {
		t.Errorf("expected isGroup to be true, got %v", result["isGroup"])
	}
}

func TestMoveToGroup(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a group project
	groupBody, _ := json.Marshal(map[string]interface{}{"name": "My Group", "isGroup": true})
	groupReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(groupBody))
	groupReq.Header.Set("Authorization", "Bearer "+token)
	groupReq.Header.Set("Content-Type", "application/json")

	groupResp, _ := app.Test(groupReq, -1)
	groupRespBody, _ := io.ReadAll(groupResp.Body)
	groupResp.Body.Close()

	var group map[string]interface{}
	json.Unmarshal(groupRespBody, &group)
	groupID := group["id"].(string)

	// Create a project to move
	projBody, _ := json.Marshal(map[string]string{"name": "Moveable Project"})
	projReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(projBody))
	projReq.Header.Set("Authorization", "Bearer "+token)
	projReq.Header.Set("Content-Type", "application/json")

	projResp, _ := app.Test(projReq, -1)
	projRespBody, _ := io.ReadAll(projResp.Body)
	projResp.Body.Close()

	var proj map[string]interface{}
	json.Unmarshal(projRespBody, &proj)
	projID := proj["id"].(string)

	// Move to group
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projID+"/move-to/"+groupID, nil)
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

	if result["parentId"] != groupID {
		t.Errorf("expected parentId '%s', got %v", groupID, result["parentId"])
	}
}

func TestListSubProjects(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("projects"); cleanupCollection("activity") })

	// Create a group
	groupBody, _ := json.Marshal(map[string]interface{}{"name": "Parent Group", "isGroup": true})
	groupReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(groupBody))
	groupReq.Header.Set("Authorization", "Bearer "+token)
	groupReq.Header.Set("Content-Type", "application/json")

	groupResp, _ := app.Test(groupReq, -1)
	groupRespBody, _ := io.ReadAll(groupResp.Body)
	groupResp.Body.Close()

	var group map[string]interface{}
	json.Unmarshal(groupRespBody, &group)
	groupID := group["id"].(string)

	// Create a sub-project by using parentId
	subBody, _ := json.Marshal(map[string]interface{}{"name": "Sub Project", "parentId": groupID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(subBody))
	subReq.Header.Set("Authorization", "Bearer "+token)
	subReq.Header.Set("Content-Type", "application/json")

	subResp, _ := app.Test(subReq, -1)
	subResp.Body.Close()

	// List sub-projects
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+groupID+"/sub-projects", nil)
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
		t.Fatal("expected 'data' array in response")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 sub-project, got %d", len(data))
	}
}
