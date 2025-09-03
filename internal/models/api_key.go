package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// APIKey represents an API key for authentication
type APIKey struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"not null;size:100"`
	KeyHash     string         `json:"-" gorm:"uniqueIndex;not null;size:128"` // Store hash, not raw key
	KeyPrefix   string         `json:"key_prefix" gorm:"not null;size:16"`     // First 8 chars for identification
	ClerkUserID string         `json:"clerk_user_id" gorm:"not null;size:100;index"`
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	RateLimit   int            `json:"rate_limit" gorm:"default:100"` // requests per minute
	LastUsedAt  *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName sets the table name for the APIKey model
func (APIKey) TableName() string {
	return "api_keys"
}

// APIKeyCreateRequest represents the request to create an API key
type APIKeyCreateRequest struct {
	Name      string     `json:"name" binding:"required,min=1,max=100"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// APIKeyResponse represents the API key response (without sensitive data)
type APIKeyResponse struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	ClerkUserID string     `json:"clerk_user_id"`
	IsActive    bool       `json:"is_active"`
	RateLimit   int        `json:"rate_limit"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// APIKeyCreateResponse includes the raw key for initial response only
type APIKeyCreateResponse struct {
	APIKeyResponse
	RawKey string `json:"raw_key"` // Only returned on creation
}

// GenerateAPIKey generates a new API key string
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ign_" + hex.EncodeToString(bytes), nil
}

// IsExpired checks if the API key is expired
func (a *APIKey) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*a.ExpiresAt)
}

// CanUse checks if the API key can be used (active and not expired)
func (a *APIKey) CanUse() bool {
	return a.IsActive && !a.IsExpired()
}
