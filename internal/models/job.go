package models

import (
	"time"

	"gorm.io/gorm"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusReceived  JobStatus = "received"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job represents a job in the system
type Job struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	JobID        string         `json:"job_id" gorm:"uniqueIndex;not null;size:50"`
	Language     string         `json:"language" gorm:"not null;size:50"`
	Code         string         `json:"code" gorm:"type:text;not null"`
	Status       JobStatus      `json:"status" gorm:"type:varchar(20);default:'received'"`
	Message      string         `json:"message,omitempty" gorm:"type:text"`
	Error        string         `json:"error,omitempty" gorm:"type:text"`
	StdErr       string         `json:"stderr,omitempty" gorm:"type:text"`
	StdOut       string         `json:"stdout,omitempty" gorm:"type:text"`
	ExecDuration int            `json:"exec_duration,omitempty"`
	MemUsage     int64          `json:"mem_usage,omitempty"`
	ClerkUserID  string         `json:"clerk_user_id" gorm:"not null;size:100;index"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName sets the table name for the Job model
func (Job) TableName() string {
	return "jobs"
}

// JobCreateRequest represents the request to create a job
type JobCreateRequest struct {
	Language string `json:"language" binding:"required,min=1,max=50"`
	Code     string `json:"code" binding:"required,min=1"`
}

// JobResponse represents the job response
type JobResponse struct {
	ID           uint      `json:"id"`
	JobID        string    `json:"job_id"`
	Language     string    `json:"language"`
	Code         string    `json:"code"`
	Status       JobStatus `json:"status"`
	Message      string    `json:"message,omitempty"`
	Error        string    `json:"error,omitempty"`
	StdErr       string    `json:"stderr,omitempty"`
	StdOut       string    `json:"stdout,omitempty"`
	ExecDuration int       `json:"exec_duration,omitempty"`
	MemUsage     int64     `json:"mem_usage,omitempty"`
	ClerkUserID  string    `json:"clerk_user_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BenchJob represents the job structure expected by the worker
type BenchJob struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

// JobStatusUpdate represents job status updates from the worker
type JobStatusUpdate struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Error        string `json:"error"`
	StdErr       string `json:"stderr"`
	StdOut       string `json:"stdout"`
	ExecDuration int    `json:"exec_duration"`
	MemUsage     int64  `json:"mem_usage"`
}
