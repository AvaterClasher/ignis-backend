package services

import (
	"crypto/sha256"
	"fmt"
	"time"

	"ignis/internal/models"

	log "github.com/sirupsen/logrus"
)

// APIKeyService handles business logic for API keys
type APIKeyService struct {
	dbService *DBService
}

// NewAPIKeyService creates a new instance of APIKeyService
func NewAPIKeyService(dbService *DBService) *APIKeyService {
	return &APIKeyService{
		dbService: dbService,
	}
}

// CreateAPIKey creates a new API key for a user
func (s *APIKeyService) CreateAPIKey(req models.APIKeyCreateRequest, clerkUserID string) (*models.APIKeyCreateResponse, error) {
	// Generate raw API key
	rawKey, err := models.GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the key for storage
	keyHash := s.hashAPIKey(rawKey)

	// Extract prefix for identification (first 16 chars including "ign_")
	keyPrefix := rawKey[:16]

	// Create API key record
	apiKey := models.APIKey{
		Name:        req.Name,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		ClerkUserID: clerkUserID,
		IsActive:    true,
		RateLimit:   5,
		ExpiresAt:   req.ExpiresAt,
	}

	err = s.dbService.Create(&apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	log.WithFields(log.Fields{
		"api_key_id":    apiKey.ID,
		"name":          apiKey.Name,
		"clerk_user_id": clerkUserID,
		"rate_limit":    apiKey.RateLimit,
	}).Info("API key created")

	// Return response with raw key (only time it's exposed)
	response := &models.APIKeyCreateResponse{
		APIKeyResponse: models.APIKeyResponse{
			ID:          apiKey.ID,
			Name:        apiKey.Name,
			KeyPrefix:   apiKey.KeyPrefix,
			ClerkUserID: apiKey.ClerkUserID,
			IsActive:    apiKey.IsActive,
			RateLimit:   apiKey.RateLimit,
			ExpiresAt:   apiKey.ExpiresAt,
			CreatedAt:   apiKey.CreatedAt,
			UpdatedAt:   apiKey.UpdatedAt,
		},
		RawKey: rawKey,
	}

	return response, nil
}

// GetAPIKeysByUser retrieves all API keys for a user
func (s *APIKeyService) GetAPIKeysByUser(clerkUserID string) ([]models.APIKeyResponse, error) {
	var apiKeys []models.APIKey
	err := s.dbService.FindWhere(&apiKeys, "clerk_user_id = ?", clerkUserID)
	if err != nil {
		return nil, err
	}

	var responses []models.APIKeyResponse
	for _, apiKey := range apiKeys {
		responses = append(responses, s.toAPIKeyResponse(apiKey))
	}

	return responses, nil
}

// GetAPIKeyByID retrieves an API key by ID for a specific user
func (s *APIKeyService) GetAPIKeyByID(id uint, clerkUserID string) (*models.APIKeyResponse, error) {
	var apiKey models.APIKey
	err := s.dbService.FindOne(&apiKey, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	response := s.toAPIKeyResponse(apiKey)
	return &response, nil
}

// DeleteAPIKey soft deletes an API key
func (s *APIKeyService) DeleteAPIKey(id uint, clerkUserID string) error {
	var apiKey models.APIKey
	err := s.dbService.FindOne(&apiKey, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return fmt.Errorf("API key not found")
	}

	err = s.dbService.Delete(&apiKey, apiKey.ID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	log.WithFields(log.Fields{
		"api_key_id":    id,
		"clerk_user_id": clerkUserID,
	}).Info("API key deleted")

	return nil
}

// UpdateAPIKey updates an API key's properties
func (s *APIKeyService) UpdateAPIKey(id uint, clerkUserID string, isActive bool) error {
	var apiKey models.APIKey
	err := s.dbService.FindOne(&apiKey, "id = ? AND clerk_user_id = ?", id, clerkUserID)
	if err != nil {
		return fmt.Errorf("API key not found")
	}

	apiKey.IsActive = isActive
	err = s.dbService.Update(&apiKey)
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	log.WithFields(log.Fields{
		"api_key_id":    id,
		"clerk_user_id": clerkUserID,
		"is_active":     isActive,
	}).Info("API key updated")

	return nil
}

// ValidateAPIKey validates an API key and returns the associated user info
func (s *APIKeyService) ValidateAPIKey(rawKey string) (*models.APIKey, error) {
	if rawKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Hash the provided key
	keyHash := s.hashAPIKey(rawKey)

	// Find the API key by hash
	var apiKey models.APIKey
	err := s.dbService.FindOne(&apiKey, "key_hash = ?", keyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check if key can be used
	if !apiKey.CanUse() {
		return nil, fmt.Errorf("API key is disabled or expired")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsedAt = &now
	_ = s.dbService.Update(&apiKey) // Don't fail if this fails

	return &apiKey, nil
}

// hashAPIKey creates a SHA256 hash of the API key
func (s *APIKeyService) hashAPIKey(rawKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(rawKey))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// toAPIKeyResponse converts APIKey model to APIKeyResponse
func (s *APIKeyService) toAPIKeyResponse(apiKey models.APIKey) models.APIKeyResponse {
	return models.APIKeyResponse{
		ID:          apiKey.ID,
		Name:        apiKey.Name,
		KeyPrefix:   apiKey.KeyPrefix,
		ClerkUserID: apiKey.ClerkUserID,
		IsActive:    apiKey.IsActive,
		RateLimit:   apiKey.RateLimit,
		LastUsedAt:  apiKey.LastUsedAt,
		ExpiresAt:   apiKey.ExpiresAt,
		CreatedAt:   apiKey.CreatedAt,
		UpdatedAt:   apiKey.UpdatedAt,
	}
}
