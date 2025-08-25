package controllers

import (
	"net/http"
	"strconv"

	"ignis/internal/middleware"
	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-gonic/gin"
)

// APIKeyController handles HTTP requests for API key management
type APIKeyController struct {
	apiKeyService *services.APIKeyService
}

// NewAPIKeyController creates a new instance of APIKeyController
func NewAPIKeyController(apiKeyService *services.APIKeyService) *APIKeyController {
	return &APIKeyController{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKey handles POST /api-keys
func (c *APIKeyController) CreateAPIKey(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.APIKeyCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiKey, err := c.apiKeyService.CreateAPIKey(req, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": apiKey})
}

// GetAPIKeys handles GET /api-keys
func (c *APIKeyController) GetAPIKeys(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	apiKeys, err := c.apiKeyService.GetAPIKeysByUser(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": apiKeys})
}

// GetAPIKey handles GET /api-keys/:id
func (c *APIKeyController) GetAPIKey(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	apiKey, err := c.apiKeyService.GetAPIKeyByID(uint(id), userID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": apiKey})
}

// UpdateAPIKey handles PUT/PATCH /api-keys/:id
func (c *APIKeyController) UpdateAPIKey(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = c.apiKeyService.UpdateAPIKey(uint(id), userID, req.IsActive)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated API key
	apiKey, err := c.apiKeyService.GetAPIKeyByID(uint(id), userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated API key"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": apiKey})
}

// DeleteAPIKey handles DELETE /api-keys/:id
func (c *APIKeyController) DeleteAPIKey(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	err = c.apiKeyService.DeleteAPIKey(uint(id), userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "API key deleted successfully"})
}
