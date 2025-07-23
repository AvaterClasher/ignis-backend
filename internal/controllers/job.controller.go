package controllers

import (
	"net/http"
	"strconv"

	"ignis/internal/middleware"
	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-gonic/gin"
)

// JobController handles HTTP requests for jobs
type JobController struct {
	jobService *services.JobService
}

// NewJobController creates a new instance of JobController
func NewJobController(jobService *services.JobService) *JobController {
	return &JobController{
		jobService: jobService,
	}
}

// CreateJob handles POST /jobs
func (c *JobController) CreateJob(ctx *gin.Context) {
	// Get user ID from Clerk middleware
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.JobCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := c.jobService.CreateJob(req, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": job})
}

// GetJob handles GET /jobs/:id
func (c *JobController) GetJob(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, err := c.jobService.GetJobByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": job})
}

// GetJobByJobID handles GET /jobs/job_id/:job_id
func (c *JobController) GetJobByJobID(ctx *gin.Context) {
	jobID := ctx.Param("job_id")
	if jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	job, err := c.jobService.GetJobByJobID(jobID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": job})
}

// GetAllJobs handles GET /jobs
func (c *JobController) GetAllJobs(ctx *gin.Context) {
	jobs, err := c.jobService.GetAllJobs()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}

// GetJobsByUser handles GET /users/:id/jobs - now gets jobs for current authenticated user
func (c *JobController) GetJobsByUser(ctx *gin.Context) {
	// Get user ID from Clerk middleware
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	jobs, err := c.jobService.GetJobsByClerkUserID(userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}

// GetMyJobs handles GET /jobs/my - gets jobs for current authenticated user
func (c *JobController) GetMyJobs(ctx *gin.Context) {
	// Get user ID from Clerk middleware
	userID, exists := middleware.GetUserIDFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	jobs, err := c.jobService.GetJobsByClerkUserID(userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}

// GetJobsByStatus handles GET /jobs/status/:status
func (c *JobController) GetJobsByStatus(ctx *gin.Context) {
	statusParam := ctx.Param("status")
	status := models.JobStatus(statusParam)

	// Validate status
	switch status {
	case models.JobStatusReceived, models.JobStatusRunning, models.JobStatusCompleted, models.JobStatusFailed:
		// Valid status
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Valid values: received, running, completed, failed"})
		return
	}

	jobs, err := c.jobService.GetJobsByStatus(status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}
