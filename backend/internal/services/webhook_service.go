package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
)

// WebhookService handles webhook delivery with retries and logging
type WebhookService struct {
	repo   *repository.WorkspaceRepository
	client *http.Client
}

// WebhookPayload represents the webhook event payload
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// NewWebhookService creates a new webhook service
func NewWebhookService(repo *repository.WorkspaceRepository) *WebhookService {
	return &WebhookService{
		repo: repo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TriggerEvent sends webhook notifications for an event in a workspace
func (s *WebhookService) TriggerEvent(workspaceID uuid.UUID, event string, data interface{}) {
	go func() {
		webhooks, err := s.repo.GetWebhooks(workspaceID)
		if err != nil {
			log.Printf("webhook: failed to fetch webhooks for workspace %s: %v", workspaceID, err)
			return
		}

		payload := WebhookPayload{
			Event:     event,
			Timestamp: time.Now().UTC(),
			Data:      data,
		}

		for _, wh := range webhooks {
			if !wh.IsActive {
				continue
			}
			// Check if event is subscribed
			if !s.isEventSubscribed(wh.Events, event) {
				continue
			}
			go s.deliverWebhook(wh, payload)
		}
	}()
}

func (s *WebhookService) isEventSubscribed(events string, event string) bool {
	// Simple comma-separated check
	for _, e := range splitEvents(events) {
		if e == event || e == "*" {
			return true
		}
	}
	return false
}

func splitEvents(events string) []string {
	var result []string
	current := ""
	for _, c := range events {
		if c == ',' {
			if trimmed := trim(current); trimmed != "" {
				result = append(result, trimmed)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if trimmed := trim(current); trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}

func trim(s string) string {
	start, end := 0, len(s)-1
	for start <= end && s[start] == ' ' {
		start++
	}
	for end >= start && s[end] == ' ' {
		end--
	}
	if start > end {
		return ""
	}
	return s[start : end+1]
}

// deliverWebhook sends the webhook with retry logic (up to 3 attempts)
func (s *WebhookService) deliverWebhook(wh models.Webhook, payload WebhookPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: failed to marshal payload: %v", err)
		return
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		logEntry := models.WebhookLog{
			WebhookID: wh.ID,
			Event:     payload.Event,
			Payload:   string(body),
		}

		req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
		if err != nil {
			logEntry.Success = false
			logEntry.Error = err.Error()
			s.repo.CreateWebhookLog(&logEntry)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "QRit-Webhook/1.0")
		req.Header.Set("X-QRit-Event", payload.Event)
		req.Header.Set("X-QRit-Delivery", uuid.New().String())

		// Sign payload with secret if configured
		if wh.Secret != "" {
			signature := s.signPayload(body, wh.Secret)
			req.Header.Set("X-QRit-Signature", "sha256="+signature)
		}

		startTime := time.Now()
		resp, err := s.client.Do(req)
		duration := time.Since(startTime).Milliseconds()

		logEntry.Duration = int(duration)

		if err != nil {
			logEntry.Success = false
			logEntry.Error = err.Error()
			s.repo.CreateWebhookLog(&logEntry)

			// Exponential backoff
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*attempt) * time.Second)
			}
			continue
		}

		logEntry.StatusCode = resp.StatusCode
		logEntry.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		resp.Body.Close()

		if !logEntry.Success {
			logEntry.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}

		s.repo.CreateWebhookLog(&logEntry)

		if logEntry.Success {
			// Reset fail count on success
			now := time.Now()
			wh.LastTriggered = &now
			wh.FailCount = 0
			s.repo.UpdateWebhookHealth(&wh)
			return
		}

		// Increment fail count
		wh.FailCount++
		if wh.FailCount >= 10 {
			wh.IsActive = false // Auto-disable after 10 consecutive failures
		}
		s.repo.UpdateWebhookHealth(&wh)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
	}
}

func (s *WebhookService) signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
