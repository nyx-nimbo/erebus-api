package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

const calendarAPIBase = "https://www.googleapis.com/calendar/v3/calendars/primary"

// TodayEvents returns today's calendar events.
func TodayEvents(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required", "code": 400})
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	url := fmt.Sprintf("%s/events?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime",
		calendarAPIBase,
		startOfDay.Format(time.RFC3339),
		endOfDay.Format(time.RFC3339),
	)

	data, err := calendarRequest("GET", url, token, nil)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Calendar API error: " + err.Error(), "code": 502})
	}

	return c.JSON(data)
}

// UpcomingEvents returns events for the next N days.
func UpcomingEvents(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required", "code": 400})
	}

	days, _ := strconv.Atoi(c.Query("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}

	now := time.Now()
	end := now.Add(time.Duration(days) * 24 * time.Hour)

	url := fmt.Sprintf("%s/events?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime",
		calendarAPIBase,
		now.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)

	data, err := calendarRequest("GET", url, token, nil)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Calendar API error: " + err.Error(), "code": 502})
	}

	return c.JSON(data)
}

// CreateEvent creates a calendar event.
func CreateEvent(c *fiber.Ctx) error {
	token := c.Get("X-Google-Token")
	if token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "X-Google-Token header required", "code": 400})
	}

	var event map[string]interface{}
	if err := c.BodyParser(&event); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body", "code": 400})
	}

	url := fmt.Sprintf("%s/events", calendarAPIBase)
	data, err := calendarRequest("POST", url, token, event)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Calendar API error: " + err.Error(), "code": 502})
	}

	return c.Status(201).JSON(data)
}

func calendarRequest(method, url, token string, body interface{}) (map[string]interface{}, error) {
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
