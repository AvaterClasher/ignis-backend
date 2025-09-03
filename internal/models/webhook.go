package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	WebhookEventJobCompleted WebhookEventType = "job.completed"
	WebhookEventJobFailed    WebhookEventType = "job.failed"
)

// WebhookEventTypes is a custom type for handling JSON serialization of event types slice
type WebhookEventTypes []WebhookEventType

// Value implements the driver.Valuer interface for database storage
func (w WebhookEventTypes) Value() (driver.Value, error) {
	if w == nil {
		return nil, nil
	}
	return json.Marshal(w)
}

// Scan implements the sql.Scanner interface for database retrieval
func (w *WebhookEventTypes) Scan(value interface{}) error {
	if value == nil {
		*w = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into WebhookEventTypes", value)
	}

	return json.Unmarshal(bytes, w)
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID          uint              `json:"id" gorm:"primaryKey"`
	URL         string            `json:"url" gorm:"not null;size:500"`
	Secret      string            `json:"-" gorm:"size:100"` // HMAC secret for signature verification
	Events      WebhookEventTypes `json:"events" gorm:"type:json;not null"`
	IsActive    bool              `json:"is_active" gorm:"default:true"`
	ClerkUserID string            `json:"clerk_user_id" gorm:"not null;size:100;index"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeletedAt   gorm.DeletedAt    `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName sets the table name for the Webhook model
func (Webhook) TableName() string {
	return "webhooks"
}

// WebhookEvent represents a webhook event delivery
type WebhookEvent struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	WebhookID    uint             `json:"webhook_id" gorm:"not null;index"`
	Webhook      Webhook          `json:"webhook,omitempty" gorm:"foreignKey:WebhookID"`
	EventType    WebhookEventType `json:"event_type" gorm:"not null;size:50"`
	JobID        string           `json:"job_id" gorm:"not null;size:50;index"`
	Payload      string           `json:"payload" gorm:"type:text;not null"`
	Delivered    bool             `json:"delivered" gorm:"default:false"`
	StatusCode   int              `json:"status_code,omitempty"`
	Response     string           `json:"response,omitempty" gorm:"type:text"`
	AttemptCount int              `json:"attempt_count" gorm:"default:0"`
	NextRetryAt  *time.Time       `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// TableName sets the table name for the WebhookEvent model
func (WebhookEvent) TableName() string {
	return "webhook_events"
}

// WebhookCreateRequest represents the request to create a webhook
type WebhookCreateRequest struct {
	URL    string            `json:"url" binding:"required,url,max=500"`
	Secret string            `json:"secret,omitempty" binding:"max=100"`
	Events WebhookEventTypes `json:"events" binding:"required,min=1"`
}

// WebhookUpdateRequest represents the request to update a webhook
type WebhookUpdateRequest struct {
	URL      string            `json:"url,omitempty" binding:"omitempty,url,max=500"`
	Secret   string            `json:"secret,omitempty" binding:"max=100"`
	Events   WebhookEventTypes `json:"events,omitempty" binding:"omitempty,min=1"`
	IsActive *bool             `json:"is_active,omitempty"`
}

// WebhookResponse represents the webhook response
type WebhookResponse struct {
	ID          uint              `json:"id"`
	URL         string            `json:"url"`
	Events      WebhookEventTypes `json:"events"`
	IsActive    bool              `json:"is_active"`
	ClerkUserID string            `json:"clerk_user_id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// WebhookEventResponse represents the webhook event response
type WebhookEventResponse struct {
	ID           uint             `json:"id"`
	WebhookID    uint             `json:"webhook_id"`
	EventType    WebhookEventType `json:"event_type"`
	JobID        string           `json:"job_id"`
	Delivered    bool             `json:"delivered"`
	StatusCode   int              `json:"status_code,omitempty"`
	AttemptCount int              `json:"attempt_count"`
	NextRetryAt  *time.Time       `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// JobWebhookPayload represents the payload sent to webhooks for job events
type JobWebhookPayload struct {
	Event     WebhookEventType   `json:"event"`
	Timestamp time.Time          `json:"timestamp"`
	Job       JobWebhookResponse `json:"job"`
}
