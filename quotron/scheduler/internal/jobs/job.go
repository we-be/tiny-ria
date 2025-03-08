package jobs

import (
	"context"
	"time"
)

// Job defines the interface for a schedulable job
type Job interface {
	// Name returns the unique name of the job
	Name() string

	// Description returns a human-readable description of the job
	Description() string

	// Execute runs the job with the provided context and parameters
	Execute(ctx context.Context, params map[string]string) error

	// LastRun returns the timestamp of the last successful execution
	LastRun() time.Time

	// SetLastRun updates the timestamp of the last successful execution
	SetLastRun(time.Time)
}

// BaseJob provides common functionality for all jobs
type BaseJob struct {
	name        string
	description string
	lastRun     time.Time
}

// NewBaseJob creates a new base job with the given name and description
func NewBaseJob(name, description string) BaseJob {
	return BaseJob{
		name:        name,
		description: description,
		lastRun:     time.Time{}, // Zero time (never run)
	}
}

// Name returns the job name
func (j *BaseJob) Name() string {
	return j.name
}

// Description returns the job description
func (j *BaseJob) Description() string {
	return j.description
}

// LastRun returns the timestamp of the last successful execution
func (j *BaseJob) LastRun() time.Time {
	return j.lastRun
}

// SetLastRun updates the timestamp of the last successful execution
func (j *BaseJob) SetLastRun(t time.Time) {
	j.lastRun = t
}