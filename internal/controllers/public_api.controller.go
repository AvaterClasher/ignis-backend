package controllers

import (
	"fmt"
	"net/http"

	"ignis/internal/middleware"
	"ignis/internal/models"
	"ignis/internal/services"

	"github.com/gin-gonic/gin"
)

// PublicAPIController handles public API requests for external consumers
type PublicAPIController struct {
	jobService *services.JobService
}

// NewPublicAPIController creates a new instance of PublicAPIController
func NewPublicAPIController(jobService *services.JobService) *PublicAPIController {
	return &PublicAPIController{
		jobService: jobService,
	}
}

// ExecuteCodeRequest represents the public API request for code execution
type ExecuteCodeRequest struct {
	Language string `json:"language" binding:"required,min=1,max=50"`
	Code     string `json:"code" binding:"required,min=1"`
}

// ExecuteCodeResponse represents the public API response for code execution
type ExecuteCodeResponse struct {
	JobID    string           `json:"job_id"`
	Language string           `json:"language"`
	Status   models.JobStatus `json:"status"`
	Message  string           `json:"message,omitempty"`
}

// JobStatusResponse represents the public API response for job status
type JobStatusResponse struct {
	JobID        string           `json:"job_id"`
	Language     string           `json:"language"`
	Status       models.JobStatus `json:"status"`
	Message      string           `json:"message,omitempty"`
	Error        string           `json:"error,omitempty"`
	StdOut       string           `json:"stdout,omitempty"`
	StdErr       string           `json:"stderr,omitempty"`
	ExecDuration int              `json:"exec_duration,omitempty"`
	MemUsage     int64            `json:"mem_usage,omitempty"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

// ExecuteCode handles POST /public/execute - Submit code for execution
func (c *PublicAPIController) ExecuteCode(ctx *gin.Context) {
	// Get API key data from context (API key auth required)
	apiKey, exists := middleware.GetAPIKeyFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "API key authentication required"})
		return
	}

	var req ExecuteCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to job create request
	jobReq := models.JobCreateRequest{
		Language: req.Language,
		Code:     req.Code,
	}

	// Create job using the API key's associated user ID
	job, err := c.jobService.CreateJob(jobReq, apiKey.ClerkUserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return simplified response for public API
	response := ExecuteCodeResponse{
		JobID:    job.JobID,
		Language: job.Language,
		Status:   job.Status,
		Message:  "Code submitted for execution",
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": response})
}

// GetJobStatus handles GET /public/jobs/:job_id - Get job execution status and results
func (c *PublicAPIController) GetJobStatus(ctx *gin.Context) {
	// Get API key data from context (API key auth required)
	apiKey, exists := middleware.GetAPIKeyFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "API key authentication required"})
		return
	}

	jobID := ctx.Param("job_id")
	if jobID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Job ID is required"})
		return
	}

	// Get job by job ID
	job, err := c.jobService.GetJobByJobID(jobID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	// Verify the job belongs to the API key's user
	if job.ClerkUserID != apiKey.ClerkUserID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied - job belongs to different user"})
		return
	}

	// Return simplified response for public API
	response := JobStatusResponse{
		JobID:        job.JobID,
		Language:     job.Language,
		Status:       job.Status,
		Message:      job.Message,
		Error:        job.Error,
		StdOut:       job.StdOut,
		StdErr:       job.StdErr,
		ExecDuration: job.ExecDuration,
		MemUsage:     job.MemUsage,
		CreatedAt:    job.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    job.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	ctx.JSON(http.StatusOK, gin.H{"data": response})
}

// GetMyJobs handles GET /public/jobs - Get all jobs for the authenticated API key user
func (c *PublicAPIController) GetMyJobs(ctx *gin.Context) {
	// Get API key data from context (API key auth required)
	apiKey, exists := middleware.GetAPIKeyFromContext(ctx)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "API key authentication required"})
		return
	}

	// Get pagination parameters
	limit := 50 // Default limit
	offset := 0 // Default offset

	if limitParam := ctx.Query("limit"); limitParam != "" {
		if parsedLimit := parseInt(limitParam, 1, 100); parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetParam := ctx.Query("offset"); offsetParam != "" {
		if parsedOffset := parseInt(offsetParam, 0, 999999); parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	jobs, err := c.jobService.GetJobsByClerkUserID(apiKey.ClerkUserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply pagination
	total := len(jobs)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedJobs := jobs[start:end]

	// Convert to simplified response format
	var responses []JobStatusResponse
	for _, job := range paginatedJobs {
		responses = append(responses, JobStatusResponse{
			JobID:        job.JobID,
			Language:     job.Language,
			Status:       job.Status,
			Message:      job.Message,
			Error:        job.Error,
			StdOut:       job.StdOut,
			StdErr:       job.StdErr,
			ExecDuration: job.ExecDuration,
			MemUsage:     job.MemUsage,
			CreatedAt:    job.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    job.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": responses,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"count":  len(responses),
		},
	})
}

// GetAPIStatus handles GET /public/status - Get API status and basic info
func (c *PublicAPIController) GetAPIStatus(ctx *gin.Context) {
	// This endpoint can be used to check API connectivity and get basic info
	response := gin.H{
		"status":      "operational",
		"version":     "1.0.0",
		"service":     "Ignis Code Execution API",
		"description": "Submit code for execution and retrieve results",
		"endpoints": gin.H{
			"execute": "POST /public/execute",
			"status":  "GET /public/jobs/{job_id}",
			"jobs":    "GET /public/jobs",
		},
		"supported_languages": []string{
			"python", "go", 
		},
	}

	ctx.JSON(http.StatusOK, response)
}

// Helper function to parse integer with bounds
func parseInt(str string, min, max int) int {
	var result int
	if _, err := fmt.Sscanf(str, "%d", &result); err != nil {
		return -1
	}
	if result < min || result > max {
		return -1
	}
	return result
}
