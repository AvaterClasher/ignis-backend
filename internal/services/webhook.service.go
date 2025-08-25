package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ignis/internal/models"

	log "github.com/sirupsen/logrus"
)

// WebhookService handles webhook operations
type WebhookService struct {
	dbService  *DBService
	httpClient *http.Client
}

// NewWebhookService creates a new webhook service
func NewWebhookService(dbService *DBService) *WebhookService {
	return &WebhookService{
		dbService: dbService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateWebhook creates a new webhook configuration
func (s *WebhookService) CreateWebhook(req models.WebhookCreateRequest, clerkUserID string) (*models.WebhookResponse, error) {
	webhook := models.Webhook{
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
		IsActive:    true,
		ClerkUserID: clerkUserID,
	}

	err := s.dbService.Create(&webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	log.WithFields(log.Fields{
		"webhook_id":    webhook.ID,
		"url":           webhook.URL,
		"events":        webhook.Events,
		"clerk_user_id": clerkUserID,
	}).Info("Webhook created")

	return s.toWebhookResponse(webhook), nil
}

// GetWebhooksByUser retrieves all webhooks for a user
func (s *WebhookService) GetWebhooksByUser(clerkUserID string) ([]models.WebhookResponse, error) {
	var webhooks []models.Webhook
	err := s.dbService.FindWhere(&webhooks, "clerk_user_id = ?", clerkUserID)
	if err != nil {
		return nil, err
	}

	var responses []models.WebhookResponse
	for _, webhook := range webhooks {
		responses = append(responses, *s.toWebhookResponse(webhook))
	}

	return responses, nil
}

// GetWebhookByID retrieves a webhook by ID for a specific user
func (s *WebhookService) GetWebhookByID(id uint, clerkUserID string) (*models.WebhookResponse, error) {
	var webhook models.Webhook
	err := s.dbService.FindOne(&webhook, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found")
	}

	return s.toWebhookResponse(webhook), nil
}

// UpdateWebhook updates a webhook configuration
func (s *WebhookService) UpdateWebhook(id uint, clerkUserID string, req models.WebhookUpdateRequest) (*models.WebhookResponse, error) {
	var webhook models.Webhook
	err := s.dbService.FindOne(&webhook, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found")
	}

	// Update fields if provided
	if req.URL != "" {
		webhook.URL = req.URL
	}
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}
	if len(req.Events) > 0 {
		webhook.Events = req.Events
	}
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
	}

	err = s.dbService.Update(&webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	log.WithFields(log.Fields{
		"webhook_id":    id,
		"clerk_user_id": clerkUserID,
	}).Info("Webhook updated")

	return s.toWebhookResponse(webhook), nil
}

// DeleteWebhook soft deletes a webhook
func (s *WebhookService) DeleteWebhook(id uint, clerkUserID string) error {
	var webhook models.Webhook
	err := s.dbService.FindOne(&webhook, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return fmt.Errorf("webhook not found")
	}

	err = s.dbService.Delete(&webhook, webhook.ID)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	log.WithFields(log.Fields{
		"webhook_id":    id,
		"clerk_user_id": clerkUserID,
	}).Info("Webhook deleted")

	return nil
}

// SendWebhookEvent sends a webhook event for a job
func (s *WebhookService) SendWebhookEvent(job *models.JobWebhookResponse, clerkUserID string, eventType models.WebhookEventType) error {
	// Find all active webhooks for the user that are subscribed to this event type
	var webhooks []models.Webhook
	err := s.dbService.FindWhere(&webhooks, "clerk_user_id = ? AND is_active = ?", clerkUserID, true)
	if err != nil {
		log.WithError(err).Error("Failed to fetch webhooks for user")
		return err
	}

	// Filter webhooks by event type
	var subscribedWebhooks []models.Webhook
	for _, webhook := range webhooks {
		for _, event := range webhook.Events {
			if event == eventType {
				subscribedWebhooks = append(subscribedWebhooks, webhook)
				break
			}
		}
	}

	if len(subscribedWebhooks) == 0 {
		log.WithFields(log.Fields{
			"job_id":     job.JobID,
			"event_type": eventType,
			"user_id":    clerkUserID,
		}).Debug("No webhooks subscribed to this event type")
		return nil
	}

	// Create webhook payload
	payload := models.JobWebhookPayload{
		Event:     eventType,
		Timestamp: time.Now(),
		Job:       *job,
	}

	// Send to all subscribed webhooks
	for _, webhook := range subscribedWebhooks {
		go s.sendWebhookEventAsync(webhook, payload, job.JobID)
	}

	return nil
}

// sendWebhookEventAsync sends a webhook event asynchronously with retries
func (s *WebhookService) sendWebhookEventAsync(webhook models.Webhook, payload models.JobWebhookPayload, jobID string) {
	// Create webhook event record
	webhookEvent := models.WebhookEvent{
		WebhookID:    webhook.ID,
		EventType:    payload.Event,
		JobID:        jobID,
		AttemptCount: 0,
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.WithError(err).Error("Failed to marshal webhook payload")
		return
	}
	webhookEvent.Payload = string(payloadBytes)

	// Save event record
	err = s.dbService.Create(&webhookEvent)
	if err != nil {
		log.WithError(err).Error("Failed to create webhook event record")
		return
	}

	// Send webhook with retries
	s.sendWebhookWithRetries(&webhookEvent, webhook, payloadBytes)
}

// sendWebhookWithRetries sends a webhook with exponential backoff retries
func (s *WebhookService) sendWebhookWithRetries(webhookEvent *models.WebhookEvent, webhook models.Webhook, payloadBytes []byte) {
	maxRetries := 3
	baseDelay := time.Second * 2

	for attempt := 0; attempt < maxRetries; attempt++ {
		webhookEvent.AttemptCount = attempt + 1

		// Create HTTP request
		req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			log.WithError(err).Error("Failed to create webhook request")
			continue
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Ignis-Webhooks/1.0")
		req.Header.Set("X-Webhook-Event", string(webhookEvent.EventType))
		req.Header.Set("X-Webhook-Delivery", fmt.Sprintf("%d", webhookEvent.ID))

		// Add HMAC signature if secret is provided
		if webhook.Secret != "" {
			signature := s.generateHMACSignature(payloadBytes, webhook.Secret)
			req.Header.Set("X-Webhook-Signature", "sha256="+signature)
		}

		// Send request
		resp, err := s.httpClient.Do(req)
		if err != nil {
			log.WithFields(log.Fields{
				"webhook_id": webhook.ID,
				"attempt":    attempt + 1,
				"error":      err.Error(),
			}).Warn("Webhook delivery failed")

			// Update event record with error
			webhookEvent.Response = err.Error()
			s.dbService.Update(webhookEvent)

			// Wait before retry
			if attempt < maxRetries-1 {
				delay := time.Duration(attempt+1) * baseDelay
				time.Sleep(delay)
			}
			continue
		}

		// Read response
		var responseBody bytes.Buffer
		if resp.Body != nil {
			responseBody.ReadFrom(resp.Body)
			resp.Body.Close()
		}

		// Update event record
		webhookEvent.StatusCode = resp.StatusCode
		webhookEvent.Response = responseBody.String()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success
			webhookEvent.Delivered = true
			s.dbService.Update(webhookEvent)

			log.WithFields(log.Fields{
				"webhook_id":  webhook.ID,
				"status_code": resp.StatusCode,
				"attempt":     attempt + 1,
			}).Info("Webhook delivered successfully")
			return
		}

		// Log failure
		log.WithFields(log.Fields{
			"webhook_id":  webhook.ID,
			"status_code": resp.StatusCode,
			"attempt":     attempt + 1,
			"response":    responseBody.String(),
		}).Warn("Webhook delivery failed with non-2xx status")

		s.dbService.Update(webhookEvent)

		// Wait before retry
		if attempt < maxRetries-1 {
			delay := time.Duration(attempt+1) * baseDelay
			time.Sleep(delay)
		}
	}

	// All retries failed, schedule for later retry
	nextRetry := time.Now().Add(time.Hour) // Retry after 1 hour
	webhookEvent.NextRetryAt = &nextRetry
	s.dbService.Update(webhookEvent)

	log.WithFields(log.Fields{
		"webhook_id": webhook.ID,
		"attempts":   maxRetries,
	}).Error("Webhook delivery failed after all retries")
}

// generateHMACSignature generates HMAC SHA256 signature for webhook payload
func (s *WebhookService) generateHMACSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// toWebhookResponse converts Webhook model to WebhookResponse
func (s *WebhookService) toWebhookResponse(webhook models.Webhook) *models.WebhookResponse {
	return &models.WebhookResponse{
		ID:          webhook.ID,
		URL:         webhook.URL,
		Events:      webhook.Events,
		IsActive:    webhook.IsActive,
		ClerkUserID: webhook.ClerkUserID,
		CreatedAt:   webhook.CreatedAt,
		UpdatedAt:   webhook.UpdatedAt,
	}
}

// GetWebhookEvents retrieves webhook events for a webhook
func (s *WebhookService) GetWebhookEvents(webhookID uint, clerkUserID string, limit int, offset int) ([]models.WebhookEventResponse, error) {
	// First verify webhook belongs to user
	var webhook models.Webhook
	err := s.dbService.FindOne(&webhook, "id = ? AND clerk_user_id = ?", webhookID, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found")
	}

	// Get events with pagination
	var events []models.WebhookEvent
	query := "webhook_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?"
	err = s.dbService.GetDB().Where(query, webhookID, limit, offset).Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch webhook events: %w", err)
	}

	var responses []models.WebhookEventResponse
	for _, event := range events {
		responses = append(responses, models.WebhookEventResponse{
			ID:           event.ID,
			WebhookID:    event.WebhookID,
			EventType:    event.EventType,
			JobID:        event.JobID,
			Delivered:    event.Delivered,
			StatusCode:   event.StatusCode,
			AttemptCount: event.AttemptCount,
			NextRetryAt:  event.NextRetryAt,
			CreatedAt:    event.CreatedAt,
			UpdatedAt:    event.UpdatedAt,
		})
	}

	return responses, nil
}
