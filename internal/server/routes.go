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
    AllowHeaders:     []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With"},
    AllowCredentials: true,
}))

	// Initialize Clerk
	middleware.InitClerk()

	// Initialize services
	dbService := services.NewDBService(s.db)

	// Run migrations - only for Job model now, User is handled by Clerk
	err := dbService.AutoMigrate(&models.Job{})
	if err != nil {
		panic("Failed to run migrations: " + err.Error())
	}

	// Initialize job service without userService dependency
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	jobService, err := services.NewJobService(dbService, natsURL)
	if err != nil {
		panic("Failed to initialize job service: " + err.Error())
	}

	jobController := controllers.NewJobController(jobService)

	// Health routes (public)
	r.GET("/", s.HelloWorldHandler)
	r.GET("/health", s.healthHandler)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public routes (no authentication required)
		public := v1.Group("/public")
		{
			public.GET("/health", s.healthHandler)
		}

		// Protected routes (require Clerk authentication)
		protected := v1.Group("/")
		protected.Use(middleware.RequireClerkAuth())
		{
			// Job routes
			jobs := protected.Group("/jobs")
			{
				jobs.POST("", jobController.CreateJob)
				jobs.GET("", jobController.GetAllJobs)
				jobs.GET("/my", jobController.GetMyJobs)
				jobs.GET("/:id", jobController.GetJob)
				jobs.GET("/job_id/:job_id", jobController.GetJobByJobID)
				jobs.GET("/status/:status", jobController.GetJobsByStatus)
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
