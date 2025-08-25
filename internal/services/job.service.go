package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ignis/internal/models"

	"github.com/nats-io/nats.go"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

// JobService handles business logic for jobs
type JobService struct {
	dbService      *DBService
	natsConn       *nats.Conn
	ctx            context.Context
	webhookService *WebhookService
}

// NewJobService creates a new instance of JobService
func NewJobService(dbService *DBService, natsURL string, webhookService *WebhookService) (*JobService, error) {
	// Connect to NATS
	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1), nats.ReconnectWait(2*time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	ctx := context.Background()

	service := &JobService{
		dbService:      dbService,
		natsConn:       nc,
		ctx:            ctx,
		webhookService: webhookService,
	}

	// Start listening for job status updates
	go service.listenForJobStatusUpdates()

	return service, nil
}

// CreateJob creates a new job and publishes it to NATS
func (s *JobService) CreateJob(req models.JobCreateRequest, clerkUserID string) (*models.JobResponse, error) {
	// Generate unique job ID
	jobID := xid.New().String()

	// Create job in database
	job := models.Job{
		JobID:       jobID,
		Language:    strings.TrimSpace(req.Language),
		Code:        strings.TrimSpace(req.Code),
		Status:      models.JobStatusReceived,
		ClerkUserID: clerkUserID,
	}

	err := s.dbService.Create(&job)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	// Publish job to NATS
	benchJob := models.BenchJob{
		ID:       jobID,
		Language: job.Language,
		Code:     job.Code,
	}

	jobData, err := json.Marshal(benchJob)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job data: %w", err)
	}

	err = s.natsConn.Publish("jobs", jobData)
	if err != nil {
		return nil, fmt.Errorf("failed to publish job to NATS: %w", err)
	}

	log.WithFields(log.Fields{
		"job_id":        jobID,
		"language":      job.Language,
		"clerk_user_id": job.ClerkUserID,
	}).Info("Job created and published to NATS")

	return s.toJobResponse(job)
}

// GetJobByID retrieves a job by ID
func (s *JobService) GetJobByID(id uint) (*models.JobResponse, error) {
	var job models.Job
	err := s.dbService.GetByID(&job, id)
	if err != nil {
		return nil, err
	}

	return s.toJobResponse(job)
}

// GetJobByJobID retrieves a job by job ID
func (s *JobService) GetJobByJobID(jobID string) (*models.JobResponse, error) {
	var job models.Job
	err := s.dbService.FindOne(&job, "job_id = ?", jobID)
	if err != nil {
		return nil, fmt.Errorf("job not found")
	}

	return s.toJobResponse(job)
}

// GetAllJobs retrieves all jobs
func (s *JobService) GetAllJobs() ([]models.JobResponse, error) {
	var jobs []models.Job
	err := s.dbService.GetAll(&jobs)
	if err != nil {
		return nil, err
	}

	var jobResponses []models.JobResponse
	for _, job := range jobs {
		jobResponse, err := s.toJobResponse(job)
		if err != nil {
			return nil, err
		}
		jobResponses = append(jobResponses, *jobResponse)
	}

	return jobResponses, nil
}

// GetJobsByClerkUserID retrieves jobs for a specific Clerk user
func (s *JobService) GetJobsByClerkUserID(clerkUserID string) ([]models.JobResponse, error) {
	var jobs []models.Job
	err := s.dbService.FindWhere(&jobs, "clerk_user_id = ?", clerkUserID)
	if err != nil {
		return nil, err
	}

	var jobResponses []models.JobResponse
	for _, job := range jobs {
		jobResponse, err := s.toJobResponse(job)
		if err != nil {
			return nil, err
		}
		jobResponses = append(jobResponses, *jobResponse)
	}

	return jobResponses, nil
}

// GetJobsByStatus retrieves jobs by status
func (s *JobService) GetJobsByStatus(status models.JobStatus) ([]models.JobResponse, error) {
	var jobs []models.Job
	err := s.dbService.FindWhere(&jobs, "status = ?", status)
	if err != nil {
		return nil, err
	}

	var jobResponses []models.JobResponse
	for _, job := range jobs {
		jobResponse, err := s.toJobResponse(job)
		if err != nil {
			return nil, err
		}
		jobResponses = append(jobResponses, *jobResponse)
	}

	return jobResponses, nil
}

// listenForJobStatusUpdates listens for job status updates from NATS
func (s *JobService) listenForJobStatusUpdates() {
	// Subscribe to job status updates
	_, err := s.natsConn.Subscribe("job_status.*", func(msg *nats.Msg) {
		var statusUpdate models.JobStatusUpdate
		err := json.Unmarshal(msg.Data, &statusUpdate)
		if err != nil {
			log.WithError(err).Error("Failed to unmarshal job status update")
			return
		}

		// Update job in database
		err = s.updateJobStatus(statusUpdate)
		if err != nil {
			log.WithError(err).WithField("job_id", statusUpdate.ID).Error("Failed to update job status")
		}
	})

	if err != nil {
		log.WithError(err).Fatal("Failed to subscribe to job status updates")
	}

	log.Info("Listening for job status updates from NATS")
}

// updateJobStatus updates job status in the database
func (s *JobService) updateJobStatus(statusUpdate models.JobStatusUpdate) error {
	var job models.Job
	err := s.dbService.FindOne(&job, "job_id = ?", statusUpdate.ID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	// Map status string to JobStatus enum
	var status models.JobStatus
	switch statusUpdate.Status {
	case "received":
		status = models.JobStatusReceived
	case "running":
		status = models.JobStatusRunning
	case "done":
		status = models.JobStatusCompleted
	case "failed":
		status = models.JobStatusFailed
	default:
		return fmt.Errorf("unknown status: %s", statusUpdate.Status)
	}

	// Update job fields
	job.Status = status
	job.Message = statusUpdate.Message
	job.Error = statusUpdate.Error
	job.StdErr = statusUpdate.StdErr
	job.StdOut = statusUpdate.StdOut
	job.ExecDuration = statusUpdate.ExecDuration
	job.MemUsage = statusUpdate.MemUsage

	err = s.dbService.Update(&job)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	log.WithFields(log.Fields{
		"job_id": statusUpdate.ID,
		"status": statusUpdate.Status,
	}).Info("Job status updated")

	// Send webhook event if job is completed or failed and webhook service is available
	if s.webhookService != nil && (status == models.JobStatusCompleted || status == models.JobStatusFailed) {
		jobResponse, err := s.toWebhookJobResponse(job)
		if err != nil {
			log.WithError(err).Error("Failed to convert job to response for webhook")
		} else {
			var eventType models.WebhookEventType
			if status == models.JobStatusCompleted {
				eventType = models.WebhookEventJobCompleted
			} else {
				eventType = models.WebhookEventJobFailed
			}

			err = s.webhookService.SendWebhookEvent(jobResponse, job.ClerkUserID, eventType)
			if err != nil {
				log.WithError(err).WithField("job_id", statusUpdate.ID).Error("Failed to send webhook event")
			}
		}
	}

	return nil
}

// toJobResponse converts Job model to JobResponse
func (s *JobService) toJobResponse(job models.Job) (*models.JobResponse, error) {
	jobResponse := &models.JobResponse{
		ID:           job.ID,
		JobID:        job.JobID,
		Language:     job.Language,
		Code:         job.Code,
		Status:       job.Status,
		Message:      job.Message,
		Error:        job.Error,
		StdErr:       job.StdErr,
		StdOut:       job.StdOut,
		ExecDuration: job.ExecDuration,
		MemUsage:     job.MemUsage,
		ClerkUserID:  job.ClerkUserID,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}

	return jobResponse, nil
}

func (s *JobService) toWebhookJobResponse(job models.Job) (*models.JobWebhookResponse, error) {
	jobWebhookResponse := &models.JobWebhookResponse{
		JobID:        job.JobID,
		Language:     job.Language,
		Code:         job.Code,
		Status:       job.Status,
		Message:      job.Message,
		Error:        job.Error,
		StdErr:       job.StdErr,
		StdOut:       job.StdOut,
		ExecDuration: job.ExecDuration,
		MemUsage:     job.MemUsage,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}

	return jobWebhookResponse, nil
}

// Close closes the NATS connection
func (s *JobService) Close() error {
	if s.natsConn != nil {
		s.natsConn.Close()
	}
	return nil
}
