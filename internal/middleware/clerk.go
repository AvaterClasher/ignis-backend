package middleware

import (
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/gin-gonic/gin"
)

// UserIDKey is the key used to store user ID in Gin context
const UserIDKey = "clerk_user_id"

// InitClerk initializes the Clerk SDK with the secret key
func InitClerk() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		panic("CLERK_SECRET_KEY environment variable is required")
	}
	clerk.SetKey(secretKey)
}

// ClerkAuthMiddleware is a Gin middleware that validates Clerk sessions
// and extracts the user ID to the context
func ClerkAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a temporary response writer to capture the response
		tempWriter := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     http.StatusOK,
		}
		c.Writer = tempWriter

		// Create the Clerk middleware handler
		clerkMiddleware := clerkhttp.WithHeaderAuthorization()

		// Wrap the handler
		handler := clerkMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract claims from context
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}

			// Store user ID in Gin context for use in handlers
			c.Set(UserIDKey, claims.Subject)
			c.Set("auth_type", "clerk")

			// Update the request context in Gin context
			c.Request = r.WithContext(r.Context())

			// Continue to next handler
			c.Next()
		}))

		// Execute the handler
		handler.ServeHTTP(tempWriter, c.Request)

		// If the status code was set to unauthorized by Clerk, abort
		if tempWriter.statusCode == http.StatusUnauthorized || tempWriter.statusCode == http.StatusForbidden {
			c.JSON(tempWriter.statusCode, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
	}
}

// RequireClerkAuth is a stricter version that requires authentication
func RequireClerkAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create the Clerk middleware handler with RequireHeaderAuthorization
		clerkMiddleware := clerkhttp.RequireHeaderAuthorization()

		// Create a temporary response writer
		tempWriter := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     http.StatusOK,
		}

		// Wrap the handler
		handler := clerkMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract claims from context
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}

			// Store user ID in Gin context for use in handlers
			c.Set(UserIDKey, claims.Subject)
			c.Set("auth_type", "clerk")

			// Update the request context in Gin context
			c.Request = r.WithContext(r.Context())

			// Continue to next handler
			c.Next()
		}))

		// Execute the handler
		handler.ServeHTTP(tempWriter, c.Request)

		// If unauthorized, abort
		if tempWriter.statusCode >= 400 {
			c.JSON(tempWriter.statusCode, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
	}
}

// GetUserIDFromContext extracts the user ID from Gin context
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return "", false
	}

	userIDStr, ok := userID.(string)
	return userIDStr, ok
}

// responseWriter wraps gin.ResponseWriter to capture status codes
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
