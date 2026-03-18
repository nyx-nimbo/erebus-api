package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUsers(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
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
	// The array may be empty if no users have been upserted yet, but it should exist
	_ = data
}

func TestListAgents(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
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
	_ = data
}

func TestListMembers(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/members", nil)
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
	// Members is a combined list of users + agents. May be empty in test.
	_ = data
}
