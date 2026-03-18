package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateIdea(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	body, _ := json.Marshal(map[string]interface{}{
		"title":       "My Great Idea",
		"description": "An amazing concept",
		"category":    "product",
		"tags":        []string{"ai", "ml"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(body))
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

	if result["title"] != "My Great Idea" {
		t.Errorf("expected title 'My Great Idea', got %v", result["title"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected id in response")
	}
	if result["status"] != "new" {
		t.Errorf("expected default status 'new', got %v", result["status"])
	}
}

func TestListIdeas(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	// Create two ideas
	for _, title := range []string{"Idea A", "Idea B"} {
		body, _ := json.Marshal(map[string]string{"title": title})
		req := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		resp.Body.Close()
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/ideas", nil)
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
		t.Errorf("expected at least 2 ideas, got %d", len(data))
	}
}

func TestGetIdea(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	// Create an idea
	body, _ := json.Marshal(map[string]string{"title": "Get This Idea"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	ideaID := created["id"].(string)

	// Get
	req := httptest.NewRequest(http.MethodGet, "/api/ideas/"+ideaID, nil)
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

	if result["title"] != "Get This Idea" {
		t.Errorf("expected title 'Get This Idea', got %v", result["title"])
	}
}

func TestUpdateIdea(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	// Create an idea
	body, _ := json.Marshal(map[string]string{"title": "Old Idea"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	ideaID := created["id"].(string)

	// Update
	updateBody, _ := json.Marshal(map[string]string{
		"title":    "Updated Idea",
		"status":   "exploring",
		"category": "tech",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/ideas/"+ideaID, bytes.NewReader(updateBody))
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

	if result["title"] != "Updated Idea" {
		t.Errorf("expected title 'Updated Idea', got %v", result["title"])
	}
	if result["status"] != "exploring" {
		t.Errorf("expected status 'exploring', got %v", result["status"])
	}
}

func TestDeleteIdea(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	// Create an idea
	body, _ := json.Marshal(map[string]string{"title": "Delete Me Idea"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	ideaID := created["id"].(string)

	// Delete
	req := httptest.NewRequest(http.MethodDelete, "/api/ideas/"+ideaID, nil)
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

	if result["message"] != "Idea deleted" {
		t.Errorf("expected message 'Idea deleted', got %v", result["message"])
	}

	// Verify gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/ideas/"+ideaID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getResp, _ := app.Test(getReq, -1)
	getResp.Body.Close()

	if getResp.StatusCode != 404 {
		t.Errorf("expected 404 after deletion, got %d", getResp.StatusCode)
	}
}

func TestAddResearch(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("ideas"); cleanupCollection("activity") })

	// Create an idea
	ideaBody, _ := json.Marshal(map[string]string{"title": "Research Idea"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/ideas", bytes.NewReader(ideaBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, _ := app.Test(createReq, -1)
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	ideaID := created["id"].(string)

	// Add research entry
	researchBody, _ := json.Marshal(map[string]string{
		"type":    "article",
		"title":   "Interesting Article",
		"content": "This is a research finding about the topic.",
		"source":  "https://example.com/article",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ideas/"+ideaID+"/research", bytes.NewReader(researchBody))
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

	if result["content"] != "This is a research finding about the topic." {
		t.Errorf("expected content in response, got %v", result["content"])
	}
	if result["type"] != "article" {
		t.Errorf("expected type 'article', got %v", result["type"])
	}

	// Verify the idea now has research entries
	getReq := httptest.NewRequest(http.MethodGet, "/api/ideas/"+ideaID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)

	getResp, _ := app.Test(getReq, -1)
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()

	var idea map[string]interface{}
	json.Unmarshal(getBody, &idea)

	research, ok := idea["research"].([]interface{})
	if !ok {
		t.Fatal("expected research to be an array")
	}
	if len(research) != 1 {
		t.Errorf("expected 1 research entry, got %d", len(research))
	}
}
