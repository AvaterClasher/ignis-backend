package server

import (
	"net/http"
	"os"

	"ignis/internal/controllers"
	"ignis/internal/middleware"
	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"PUT", "PATCH", "POST", "GET", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With", "X-API-Key"},
		AllowCredentials: true,
	}))

	// Initialize Clerk
	middleware.InitClerk()

	// Initialize services
	dbService := services.NewDBService(s.db)

	// Run migrations for all models
	err := dbService.AutoMigrate(&models.Job{}, &models.APIKey{}, &models.Webhook{}, &models.WebhookEvent{})
	if err != nil {
		panic("Failed to run migrations: " + err.Error())
	}

	// Initialize rate limiter service
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "" // Will fall back to in-memory
	}
	rateLimiterService := services.NewRateLimiterService(redisURL)

	// Initialize API key service
	apiKeyService := services.NewAPIKeyService(dbService)

	// Initialize webhook service
	webhookService := services.NewWebhookService(dbService)

	// Initialize job service with webhook service
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	jobService, err := services.NewJobService(dbService, natsURL, webhookService)
	if err != nil {
		panic("Failed to initialize job service: " + err.Error())
	}

	// Initialize controllers
	jobController := controllers.NewJobController(jobService)
	apiKeyController := controllers.NewAPIKeyController(apiKeyService)
	webhookController := controllers.NewWebhookController(webhookService)
	publicAPIController := controllers.NewPublicAPIController(jobService)

	// Initialize middleware
	apiKeyMiddleware := middleware.NewAPIKeyAuthMiddleware(apiKeyService, rateLimiterService)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(rateLimiterService)

	// Health routes (public)
	r.GET("/", s.HelloWorldHandler)
	r.GET("/health", s.healthHandler)

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(rateLimitMiddleware.StandardGlobalRateLimit()) // Apply global rate limiting
	{
		// Public routes (no authentication required)
		public := v1.Group("/public")
		{
			public.GET("/health", s.healthHandler)
			public.GET("/status", publicAPIController.GetAPIStatus)
		}

		// Public API routes (API key authentication required)
		publicAPI := v1.Group("/public")
		publicAPI.Use(apiKeyMiddleware.RequireAPIKeyAuth())
		{
			publicAPI.POST("/execute", publicAPIController.ExecuteCode)
			publicAPI.GET("/jobs", publicAPIController.GetMyJobs)
			publicAPI.GET("/jobs/:job_id", publicAPIController.GetJobStatus)
		}

		// Protected routes (require Clerk authentication only - for API key/webhook management)
		protected := v1.Group("/")
		protected.Use(middleware.RequireClerkAuth())
		protected.Use(rateLimitMiddleware.StandardUserRateLimit())
		{
			// API Key management routes
			apiKeys := protected.Group("/api-keys")
			{
				apiKeys.POST("", apiKeyController.CreateAPIKey)
				apiKeys.GET("", apiKeyController.GetAPIKeys)
				apiKeys.GET("/:id", apiKeyController.GetAPIKey)
				apiKeys.PATCH("/:id", apiKeyController.UpdateAPIKey)
				apiKeys.DELETE("/:id", apiKeyController.DeleteAPIKey)
			}

			// Webhook management routes
			webhooks := protected.Group("/webhooks")
			{
				webhooks.POST("", webhookController.CreateWebhook)
				webhooks.GET("", webhookController.GetWebhooks)
				webhooks.GET("/:id", webhookController.GetWebhook)
				webhooks.PATCH("/:id", webhookController.UpdateWebhook)
				webhooks.DELETE("/:id", webhookController.DeleteWebhook)
				webhooks.GET("/:id/events", webhookController.GetWebhookEvents)
			}
		}

		// Flexible auth routes (accept either Clerk auth or API key auth)
		flexible := v1.Group("/")
		flexible.Use(middleware.FlexibleAuth(apiKeyMiddleware))
		{
			// Job routes - support both auth methods
			jobs := flexible.Group("/jobs")
			{
				jobs.POST("", jobController.CreateJob)
				jobs.GET("/my", jobController.GetMyJobs)
				jobs.GET("/:id", jobController.GetJob)
				jobs.GET("/job_id/:job_id", jobController.GetJobByJobID)
			}
		}
	}

	return r
}

func (s *Server) HelloWorldHandler(c *gin.Context) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	c.JSON(http.StatusOK, resp)
}

func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, s.db.Health())
}
