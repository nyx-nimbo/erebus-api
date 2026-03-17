package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

const gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"

// ListEmails lists emails from Gmail inbox.
func ListEmails(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required for Gmail access", "code": 400})
	}

	maxResults := c.Query("maxResults", "20")
	q := c.Query("q", "")

	url := fmt.Sprintf("%s/messages?maxResults=%s", gmailAPIBase, maxResults)
	if q != "" {
		url += "&q=" + q
	}

	data, err := gmailRequest("GET", url, token, nil)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Gmail API error: " + err.Error(), "code": 502})
	}

	return c.JSON(data)
}

// ReadEmail reads a single email by ID.
func ReadEmail(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required", "code": 400})
	}

	emailID := c.Params("id")
	url := fmt.Sprintf("%s/messages/%s?format=full", gmailAPIBase, emailID)

	data, err := gmailRequest("GET", url, token, nil)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Gmail API error: " + err.Error(), "code": 502})
	}

	return c.JSON(data)
}

// SendEmail sends an email via Gmail.
func SendEmail(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required", "code": 400})
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	to, _ := body["to"].(string)
	subject, _ := body["subject"].(string)
	message, _ := body["body"].(string)

	if to == "" || subject == "" {
		return c.Status(400).JSON(fiber.Map{"error": "to and subject are required", "code": 400})
	}

	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s", to, subject, message)
	encoded := base64.URLEncoding.EncodeToString([]byte(raw))

	payload := map[string]string{"raw": encoded}
	url := fmt.Sprintf("%s/messages/send", gmailAPIBase)

	data, err := gmailRequest("POST", url, token, payload)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Gmail API error: " + err.Error(), "code": 502})
	}

	return c.JSON(data)
}

func gmailRequest(method, url, token string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	return result, nil
}
