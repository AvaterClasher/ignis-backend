package controllers

import (
	"net/http"
	"strconv"

	"ignis/internal/middleware"
	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-gonic/gin"
)

// WebhookController handles HTTP requests for webhook management
type WebhookController struct {
	webhookService *services.WebhookService
}

// NewWebhookController creates a new instance of WebhookController
func NewWebhookController(webhookService *services.WebhookService) *WebhookController {
	return &WebhookController{
		webhookService: webhookService,
	}
}

// CreateWebhook handles POST /webhooks
func (c *WebhookController) CreateWebhook(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.WebhookCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhook, err := c.webhookService.CreateWebhook(req, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": webhook})
}

// GetWebhooks handles GET /webhooks
func (c *WebhookController) GetWebhooks(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	webhooks, err := c.webhookService.GetWebhooksByUser(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": webhooks})
}

// GetWebhook handles GET /webhooks/:id
func (c *WebhookController) GetWebhook(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	webhook, err := c.webhookService.GetWebhookByID(uint(id), userID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": webhook})
}

// UpdateWebhook handles PUT/PATCH /webhooks/:id
func (c *WebhookController) UpdateWebhook(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var req models.WebhookUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	webhook, err := c.webhookService.UpdateWebhook(uint(id), userID, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": webhook})
}

// DeleteWebhook handles DELETE /webhooks/:id
func (c *WebhookController) DeleteWebhook(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	err = c.webhookService.DeleteWebhook(uint(id), userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Webhook deleted successfully"})
}

// GetWebhookEvents handles GET /webhooks/:id/events
func (c *WebhookController) GetWebhookEvents(ctx *gin.Context) {
	// Get user ID from context (Clerk authentication required)
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	// Parse pagination parameters
	limitParam := ctx.DefaultQuery("limit", "50")
	offsetParam := ctx.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitParam)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil || offset < 0 {
		offset = 0
	}

	events, err := c.webhookService.GetWebhookEvents(uint(id), userID, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": events,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
		},
	})
}
