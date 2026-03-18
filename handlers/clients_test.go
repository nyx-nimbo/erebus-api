package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateClient(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("clients"); cleanupCollection("activity") })

	body, _ := json.Marshal(map[string]string{
		"name":         "Acme Corp",
		"contactEmail": "contact@acme.com",
		"company":      "Acme",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
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
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["name"] != "Acme Corp" {
		t.Errorf("expected name 'Acme Corp', got %v", result["name"])
	}
	if result["contactEmail"] != "contact@acme.com" {
		t.Errorf("expected contactEmail 'contact@acme.com', got %v", result["contactEmail"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected id in response")
	}
	if result["status"] != "active" {
		t.Errorf("expected default status 'active', got %v", result["status"])
	}
}

func TestCreateClient_MissingName(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")

	body, _ := json.Marshal(map[string]string{
		"contactEmail": "contact@acme.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if result["error"] == nil {
		t.Error("expected error message in response")
	}
}

func TestListClients(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("clients"); cleanupCollection("activity") })

	// Create two clients
	for _, name := range []string{"Client A", "Client B"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		resp.Body.Close()
	}

	// List clients
	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
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
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(data))
	}

	totalCount, ok := result["totalCount"].(float64)
	if !ok {
		t.Fatal("expected totalCount in response")
	}
	if totalCount < 2 {
		t.Errorf("expected totalCount >= 2, got %v", totalCount)
	}
}

func TestGetClient(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("clients"); cleanupCollection("activity") })

	// Create a client
	body, _ := json.Marshal(map[string]string{"name": "Get Test Client"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	clientID := created["id"].(string)

	// Get the client
	req := httptest.NewRequest(http.MethodGet, "/api/clients/"+clientID, nil)
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

	if result["name"] != "Get Test Client" {
		t.Errorf("expected name 'Get Test Client', got %v", result["name"])
	}
	if result["id"] != clientID {
		t.Errorf("expected id '%s', got %v", clientID, result["id"])
	}
}

func TestUpdateClient(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("clients"); cleanupCollection("activity") })

	// Create a client
	body, _ := json.Marshal(map[string]string{"name": "Original Name"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	clientID := created["id"].(string)

	// Update the client
	updateBody, _ := json.Marshal(map[string]string{"name": "Updated Name", "notes": "some notes"})
	req := httptest.NewRequest(http.MethodPut, "/api/clients/"+clientID, bytes.NewReader(updateBody))
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

	if result["name"] != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %v", result["name"])
	}
	if result["notes"] != "some notes" {
		t.Errorf("expected notes 'some notes', got %v", result["notes"])
	}
}

func TestDeleteClient(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("clients"); cleanupCollection("activity") })

	// Create a client
	body, _ := json.Marshal(map[string]string{"name": "Delete Me"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/clients", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq, -1)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	createBody, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()

	var created map[string]interface{}
	json.Unmarshal(createBody, &created)
	clientID := created["id"].(string)

	// Delete the client
	req := httptest.NewRequest(http.MethodDelete, "/api/clients/"+clientID, nil)
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

	if result["message"] != "Client deleted" {
		t.Errorf("expected message 'Client deleted', got %v", result["message"])
	}

	// Verify it is gone
	getReq := httptest.NewRequest(http.MethodGet, "/api/clients/"+clientID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)

	getResp, err := app.Test(getReq, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	getResp.Body.Close()

	if getResp.StatusCode != 404 {
		t.Errorf("expected status 404 after deletion, got %d", getResp.StatusCode)
	}
}
