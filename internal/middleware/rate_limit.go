package middleware

import (
	"net/http"
	"strconv"
	"time"

	"ignis/internal/services"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Limit          int           // requests per window
	Window         time.Duration // time window
	SkipSuccessful bool          // whether to skip rate limiting for successful requests
	SkipOnError    bool          // whether to skip rate limiting when rate limiter has errors
	HeaderPrefix   string        // prefix for rate limit headers
}

// DefaultRateLimitConfig provides sensible defaults
var DefaultRateLimitConfig = RateLimitConfig{
	Limit:          100,
	Window:         time.Minute,
	SkipSuccessful: false,
	SkipOnError:    true,
	HeaderPrefix:   "X-RateLimit",
}

// RateLimitMiddleware handles rate limiting for authenticated requests
type RateLimitMiddleware struct {
	rateLimiter *services.RateLimiterService
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(rateLimiter *services.RateLimiterService) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimiter: rateLimiter,
	}
}

// RateLimit creates a rate limiting middleware with the given configuration
func (m *RateLimitMiddleware) RateLimit(config RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.rateLimiter == nil {
			// If no rate limiter configured, skip
			c.Next()
			return
		}

		// Determine the rate limit key and limits based on auth type
		rateLimitKey, limit := m.getRateLimitKeyAndLimit(c, config)
		if rateLimitKey == "" {
			// If we can't determine rate limit key, skip or use global
			rateLimitKey = services.GetGlobalRateLimitKey(c.FullPath())
			limit = config.Limit
		}

		// Check rate limit
		allowed, err := m.rateLimiter.Allow(rateLimitKey, limit, config.Window)
		if err != nil {
			log.WithError(err).Error("Rate limiter error")
			if !config.SkipOnError {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limiter error"})
				c.Abort()
				return
			}
			// Continue if SkipOnError is true
			c.Next()
			return
		}

		// Add rate limit headers
		m.addRateLimitHeaders(c, config, limit, allowed)

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"rate_limit": gin.H{
					"limit":  limit,
					"window": config.Window.String(),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GlobalRateLimit applies rate limiting to all requests regardless of authentication
func (m *RateLimitMiddleware) GlobalRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Limit:          limit,
		Window:         window,
		SkipSuccessful: false,
		SkipOnError:    true,
		HeaderPrefix:   "X-RateLimit-Global",
	}

	return func(c *gin.Context) {
		if m.rateLimiter == nil {
			c.Next()
			return
		}

		// Use global rate limit key
		rateLimitKey := services.GetGlobalRateLimitKey(c.FullPath())

		// Check rate limit
		allowed, err := m.rateLimiter.Allow(rateLimitKey, limit, window)
		if err != nil {
			log.WithError(err).Error("Global rate limiter error")
			if !config.SkipOnError {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limiter error"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Add rate limit headers
		m.addRateLimitHeaders(c, config, limit, allowed)

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Global rate limit exceeded",
				"rate_limit": gin.H{
					"limit":  limit,
					"window": window.String(),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserRateLimit applies rate limiting specifically for user requests
func (m *RateLimitMiddleware) UserRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Limit:          limit,
		Window:         window,
		SkipSuccessful: false,
		SkipOnError:    true,
		HeaderPrefix:   "X-RateLimit-User",
	}

	return m.RateLimit(config)
}

// getRateLimitKeyAndLimit determines the appropriate rate limit key and limit based on authentication
func (m *RateLimitMiddleware) getRateLimitKeyAndLimit(c *gin.Context, config RateLimitConfig) (string, int) {
	authType := GetAuthTypeFromContext(c)
	endpoint := c.FullPath()

	switch authType {
	case "api_key":
		// API key authentication - use API key specific limits
		if apiKey, exists := GetAPIKeyFromContext(c); exists {
			rateLimitKey := services.GetAPIKeyRateLimitKey(strconv.Itoa(int(apiKey.ID)), endpoint)
			return rateLimitKey, apiKey.RateLimit
		}

	case "clerk":
		// Clerk user authentication - use user specific limits
		if userID, exists := GetUserIDFromContext(c); exists {
			rateLimitKey := services.GetUserRateLimitKey(userID, endpoint)
			return rateLimitKey, config.Limit
		}

	default:
		// Unknown auth type - use global rate limiting
		rateLimitKey := services.GetGlobalRateLimitKey(endpoint)
		return rateLimitKey, config.Limit
	}

	return "", config.Limit
}

// addRateLimitHeaders adds rate limiting headers to the response
func (m *RateLimitMiddleware) addRateLimitHeaders(c *gin.Context, config RateLimitConfig, limit int, allowed bool) {
	// Add standard rate limit headers
	c.Header(config.HeaderPrefix+"-Limit", strconv.Itoa(limit))
	c.Header(config.HeaderPrefix+"-Window", config.Window.String())

	if allowed {
		c.Header(config.HeaderPrefix+"-Remaining", "available") // Could implement remaining count
	} else {
		c.Header(config.HeaderPrefix+"-Remaining", "0")
	}

	// Add reset time (approximate)
	resetTime := time.Now().Add(config.Window)
	c.Header(config.HeaderPrefix+"-Reset", strconv.FormatInt(resetTime.Unix(), 10))
}

// Helper functions for common rate limit configurations

// StandardUserRateLimit applies standard rate limiting for user requests (100/min)
func (m *RateLimitMiddleware) StandardUserRateLimit() gin.HandlerFunc {
	return m.UserRateLimit(100, time.Minute)
}

// StrictUserRateLimit applies strict rate limiting for user requests (20/min)
func (m *RateLimitMiddleware) StrictUserRateLimit() gin.HandlerFunc {
	return m.UserRateLimit(20, time.Minute)
}

// LenientUserRateLimit applies lenient rate limiting for user requests (500/min)
func (m *RateLimitMiddleware) LenientUserRateLimit() gin.HandlerFunc {
	return m.UserRateLimit(500, time.Minute)
}

// StandardGlobalRateLimit applies standard global rate limiting (1000/min)
func (m *RateLimitMiddleware) StandardGlobalRateLimit() gin.HandlerFunc {
	return m.GlobalRateLimit(1000, time.Minute)
}
