package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// APIKeyAuthMiddleware validates API key authentication
type APIKeyAuthMiddleware struct {
	apiKeyService *services.APIKeyService
	rateLimiter   *services.RateLimiterService
}

// NewAPIKeyAuthMiddleware creates a new API key authentication middleware
func NewAPIKeyAuthMiddleware(apiKeyService *services.APIKeyService, rateLimiter *services.RateLimiterService) *APIKeyAuthMiddleware {
	return &APIKeyAuthMiddleware{
		apiKeyService: apiKeyService,
		rateLimiter:   rateLimiter,
	}
}

// APIKeyAuth middleware that validates API key and applies rate limiting
func (m *APIKeyAuthMiddleware) APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for API key in header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header with Bearer scheme
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			c.Abort()
			return
		}

		// Validate API key
		apiKeyData, err := m.apiKeyService.ValidateAPIKey(apiKey)
		if err != nil {
			log.WithError(err).Warn("Invalid API key")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired API key"})
			c.Abort()
			return
		}

		// Check rate limits for this API key
		if m.rateLimiter != nil {
			endpoint := c.FullPath()
			rateLimitKey := services.GetAPIKeyRateLimitKey(strconv.Itoa(int(apiKeyData.ID)), endpoint)

			allowed, err := m.rateLimiter.Allow(rateLimitKey, apiKeyData.RateLimit, time.Minute)
			if err != nil {
				log.WithError(err).Error("Rate limiter error")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limiter error"})
				c.Abort()
				return
			}

			if !allowed {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded",
					"rate_limit": gin.H{
						"limit":  apiKeyData.RateLimit,
						"window": "1 minute",
					},
				})
				c.Abort()
				return
			}
		}

		// Store API key data and user ID in context
		c.Set("api_key", apiKeyData)
		c.Set("clerk_user_id", apiKeyData.ClerkUserID)
		c.Set("auth_type", "api_key")

		log.WithFields(log.Fields{
			"api_key_id":    apiKeyData.ID,
			"clerk_user_id": apiKeyData.ClerkUserID,
			"endpoint":      c.FullPath(),
		}).Debug("API key authenticated")

		c.Next()
	}
}

// RequireAPIKeyAuth middleware that strictly requires API key authentication
func (m *APIKeyAuthMiddleware) RequireAPIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First run the API key auth
		m.APIKeyAuth()(c)

		// If it was aborted, don't continue
		if c.IsAborted() {
			return
		}

		// Ensure we have an API key in context
		_, exists := c.Get("api_key")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key authentication required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// FlexibleAuth middleware that accepts either Clerk auth or API key auth
func FlexibleAuth(apiKeyMiddleware *APIKeyAuthMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if API key is provided
		apiKey := c.GetHeader("X-API-Key")
		authHeader := c.GetHeader("Authorization")

		hasAPIKey := apiKey != "" || strings.HasPrefix(authHeader, "Bearer ign_")
		hasClerkAuth := strings.HasPrefix(authHeader, "Bearer ") && !strings.HasPrefix(authHeader, "Bearer ign_")

		if hasAPIKey {
			// Use API key authentication
			apiKeyMiddleware.APIKeyAuth()(c)
		} else if hasClerkAuth {
			// Use Clerk authentication
			RequireClerkAuth()(c)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required - provide either API key or user token"})
			c.Abort()
			return
		}
	}
}

// GetAPIKeyFromContext extracts the API key data from Gin context
func GetAPIKeyFromContext(c *gin.Context) (*models.APIKey, bool) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		return nil, false
	}

	apiKeyData, ok := apiKey.(*models.APIKey)
	return apiKeyData, ok
}

// GetAuthTypeFromContext returns the authentication type used
func GetAuthTypeFromContext(c *gin.Context) string {
	authType, exists := c.Get("auth_type")
	if !exists {
		return "unknown"
	}

	authTypeStr, ok := authType.(string)
	if !ok {
		return "unknown"
	}

	return authTypeStr
}
