package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/we-be/tiny-ria/quotron/scheduler/internal/config"
	"github.com/we-be/tiny-ria/quotron/scheduler/internal/jobs"
)

// Scheduler manages scheduled jobs
type Scheduler struct {
	config     *config.SchedulerConfig
	cron       *cron.Cron
	jobMap     map[string]jobs.Job
	entryIDs   map[string]cron.EntryID
	mu         sync.Mutex
	isRunning  bool
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewScheduler creates a new scheduler with the given configuration
func NewScheduler(cfg *config.SchedulerConfig) *Scheduler {
	// Create scheduler with timezone
	var cronOpts []cron.Option
	if cfg.TimeZone != "" {
		loc, err := time.LoadLocation(cfg.TimeZone)
		if err == nil {
			cronOpts = append(cronOpts, cron.WithLocation(loc))
		} else {
			log.Printf("Error loading timezone %s: %v, using UTC", cfg.TimeZone, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	return &Scheduler{
		config:     cfg,
		cron:       cron.New(cronOpts...),
		jobMap:     make(map[string]jobs.Job),
		entryIDs:   make(map[string]cron.EntryID),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// RegisterJob registers a job with the scheduler
func (s *Scheduler) RegisterJob(job jobs.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobName := job.Name()
	if _, exists := s.jobMap[jobName]; exists {
		return fmt.Errorf("job with name '%s' already registered", jobName)
	}

	s.jobMap[jobName] = job
	return nil
}

// RegisterDefaultJobs registers the default set of jobs
func (s *Scheduler) RegisterDefaultJobs(cfg *config.Config) error {
	// Stock quotes job
	stockQuoteJob := jobs.NewStockQuoteJob(cfg.ApiKey, cfg.ApiScraper, true)
	
	// Configure to use API service if enabled
	if cfg.UseAPIService {
		stockQuoteJob.WithAPIService(cfg.ApiHost, cfg.ApiPort)
	}
	
	if err := s.RegisterJob(stockQuoteJob); err != nil {
		return err
	}

	// Market indices job
	marketIndexJob := jobs.NewMarketIndexJob(cfg.ApiKey, cfg.ApiScraper, true)
	
	// Configure to use API service if enabled
	if cfg.UseAPIService {
		marketIndexJob.WithAPIService(cfg.ApiHost, cfg.ApiPort)
	}
	
	if err := s.RegisterJob(marketIndexJob); err != nil {
		return err
	}
	
	// Crypto quotes job
	cryptoQuoteJob := jobs.NewCryptoQuoteJob(cfg.ApiScraper, true)
	
	// Configure to use API service if enabled
	if cfg.UseAPIService {
		cryptoQuoteJob.WithAPIService(cfg.ApiHost, cfg.ApiPort)
	}
	
	if err := s.RegisterJob(cryptoQuoteJob); err != nil {
		return err
	}

	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("scheduler is already running")
	}

	// Schedule all jobs
	for jobName, job := range s.jobMap {
		jobSchedule, ok := s.config.Schedules[jobName]
		if !ok {
			log.Printf("No schedule found for job '%s', skipping", jobName)
			continue
		}

		if !jobSchedule.Enabled {
			log.Printf("Job '%s' is disabled, skipping", jobName)
			continue
		}

		// Create a closure for the job execution
		jobFunc := func(j jobs.Job, params map[string]string) func() {
			return func() {
				jobCtx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
				defer cancel()

				log.Printf("Starting job '%s'", j.Name())
				start := time.Now()
				err := j.Execute(jobCtx, params)
				elapsed := time.Since(start)

				if err != nil {
					log.Printf("Job '%s' failed after %v: %v", j.Name(), elapsed, err)
				} else {
					log.Printf("Job '%s' completed successfully in %v", j.Name(), elapsed)
				}
			}
		}

		// Add the job to the cron scheduler
		entryID, err := s.cron.AddFunc(jobSchedule.Cron, jobFunc(job, jobSchedule.Parameters))
		if err != nil {
			return fmt.Errorf("failed to schedule job '%s': %w", jobName, err)
		}

		s.entryIDs[jobName] = entryID
		log.Printf("Scheduled job '%s' with cron expression '%s'", jobName, jobSchedule.Cron)
	}

	// Add heartbeat update job (runs every minute)
	s.cron.AddFunc("* * * * *", func() {
		updateHeartbeatFile()
	})
	
	// Start the cron scheduler
	s.cron.Start()
	s.isRunning = true
	log.Println("Scheduler started")
	
	// Create initial heartbeat file
	updateHeartbeatFile()

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	// Stop the context
	s.cancelFunc()

	// Stop the cron scheduler
	s.cron.Stop()
	s.isRunning = false
	log.Println("Scheduler stopped")
}

// ListJobs returns a list of all registered jobs
func (s *Scheduler) ListJobs() []jobs.Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobList := make([]jobs.Job, 0, len(s.jobMap))
	for _, job := range s.jobMap {
		jobList = append(jobList, job)
	}
	return jobList
}

// GetNextRun returns the next scheduled run time for a job
func (s *Scheduler) GetNextRun(jobName string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, ok := s.entryIDs[jobName]
	if !ok {
		return time.Time{}, fmt.Errorf("job '%s' not scheduled", jobName)
	}

	entry := s.cron.Entry(entryID)
	return entry.Next, nil
}

// RunJobNow runs a job immediately
func (s *Scheduler) RunJobNow(jobName string) error {
	s.mu.Lock()
	job, ok := s.jobMap[jobName]
	jobSchedule, scheduleOk := s.config.Schedules[jobName]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("job '%s' not found", jobName)
	}

	if !scheduleOk {
		return fmt.Errorf("job '%s' has no schedule", jobName)
	}

	// Run the job in a goroutine
	go func() {
		jobCtx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()

		log.Printf("Starting manual execution of job '%s'", job.Name())
		start := time.Now()
		err := job.Execute(jobCtx, jobSchedule.Parameters)
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Manual job '%s' failed after %v: %v", job.Name(), elapsed, err)
		} else {
			log.Printf("Manual job '%s' completed successfully in %v", job.Name(), elapsed)
		}
		
		// Update heartbeat after job execution
		updateHeartbeatFile()
	}()

	return nil
}

// updateHeartbeatFile creates or updates the scheduler heartbeat file
// This file is used by the CLI to detect if the scheduler is still active
func updateHeartbeatFile() {
	heartbeatFile := "scheduler_heartbeat"
	
	// Create or update the heartbeat file
	currentTime := []byte(time.Now().Format(time.RFC3339))
	err := os.WriteFile(heartbeatFile, currentTime, 0644)
	if err != nil {
		log.Printf("Warning: could not update heartbeat file: %v", err)
	}
}