package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessage(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("messages"); cleanupCollection("users"); cleanupCollection("agents") })

	body, _ := json.Marshal(map[string]string{
		"toId":    "recipient@example.com",
		"content": "Hello, how are you?",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
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

	if result["content"] != "Hello, how are you?" {
		t.Errorf("expected content 'Hello, how are you?', got %v", result["content"])
	}
	if result["fromId"] != "test@example.com" {
		t.Errorf("expected fromId 'test@example.com', got %v", result["fromId"])
	}
	if result["toId"] != "recipient@example.com" {
		t.Errorf("expected toId 'recipient@example.com', got %v", result["toId"])
	}
	if result["read"] != false {
		t.Errorf("expected read false, got %v", result["read"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected id in response")
	}
}

func TestGetConversation(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("messages"); cleanupCollection("users"); cleanupCollection("agents") })

	// Send a message first
	body, _ := json.Marshal(map[string]string{
		"toId":    "friend@example.com",
		"content": "Hey friend!",
	})
	sendReq := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	sendReq.Header.Set("Authorization", "Bearer "+token)
	sendReq.Header.Set("Content-Type", "application/json")
	sendResp, _ := app.Test(sendReq, -1)
	sendResp.Body.Close()

	// Get conversation
	req := httptest.NewRequest(http.MethodGet, "/api/messages?with=friend@example.com", nil)
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
		t.Errorf("expected at least 1 message, got %d", len(data))
	}
}

func TestListConversations(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("messages"); cleanupCollection("users"); cleanupCollection("agents") })

	// Send messages to two different recipients
	for _, toID := range []string{"alice@example.com", "bob@example.com"} {
		body, _ := json.Marshal(map[string]string{
			"toId":    toID,
			"content": "Hello " + toID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		resp.Body.Close()
	}

	// List conversations
	req := httptest.NewRequest(http.MethodGet, "/api/messages/conversations", nil)
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
		t.Errorf("expected at least 2 conversations, got %d", len(data))
	}
}

func TestGetUnreadCount(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("messages"); cleanupCollection("users"); cleanupCollection("agents") })

	// Get unread count (should be 0 initially)
	req := httptest.NewRequest(http.MethodGet, "/api/messages/unread", nil)
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

	count, ok := result["count"].(float64)
	if !ok {
		t.Fatal("expected count to be a number")
	}
	if count != 0 {
		t.Errorf("expected unread count 0, got %v", count)
	}
}

func TestMarkRead(t *testing.T) {
	app := setupTestApp()
	token := generateTestJWT("test@example.com", "Test User")
	t.Cleanup(func() { cleanupCollection("messages"); cleanupCollection("users"); cleanupCollection("agents") })

	// Send a message
	body, _ := json.Marshal(map[string]string{
		"toId":    "someone@example.com",
		"content": "Mark me as read",
	})
	sendReq := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	sendReq.Header.Set("Authorization", "Bearer "+token)
	sendReq.Header.Set("Content-Type", "application/json")

	sendResp, _ := app.Test(sendReq, -1)
	sendBody, _ := io.ReadAll(sendResp.Body)
	sendResp.Body.Close()

	var sent map[string]interface{}
	json.Unmarshal(sendBody, &sent)
	msgID := sent["id"].(string)

	// Mark as read
	req := httptest.NewRequest(http.MethodPut, "/api/messages/"+msgID+"/read", nil)
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

	if result["message"] != "Message marked as read" {
		t.Errorf("expected message 'Message marked as read', got %v", result["message"])
	}
}
