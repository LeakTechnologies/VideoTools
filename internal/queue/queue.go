package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobType represents the type of job to execute
type JobType string

const (
	JobTypeConvert JobType = "convert"
	JobTypeMerge   JobType = "merge"
	JobTypeTrim    JobType = "trim"
	JobTypeFilter  JobType = "filter"
	JobTypeUpscale JobType = "upscale"
	JobTypeAudio   JobType = "audio"
	JobTypeThumb   JobType = "thumb"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a single job in the queue
type Job struct {
	ID          string                 `json:"id"`
	Type        JobType                `json:"type"`
	Status      JobStatus              `json:"status"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	InputFile   string                 `json:"input_file"`
	OutputFile  string                 `json:"output_file"`
	Config      map[string]interface{} `json:"config"`
	Progress    float64                `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Priority    int                    `json:"priority"` // Higher priority = runs first
	cancel      context.CancelFunc     `json:"-"`
}

// JobExecutor is a function that executes a job
type JobExecutor func(ctx context.Context, job *Job, progressCallback func(float64)) error

// Queue manages a queue of jobs
type Queue struct {
	jobs     []*Job
	executor JobExecutor
	running  bool
	mu       sync.RWMutex
	onChange func() // Callback when queue state changes
}

// New creates a new queue with the given executor
func New(executor JobExecutor) *Queue {
	return &Queue{
		jobs:     make([]*Job, 0),
		executor: executor,
		running:  false,
	}
}

// SetChangeCallback sets a callback to be called when the queue state changes
func (q *Queue) SetChangeCallback(callback func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onChange = callback
}

// notifyChange triggers the onChange callback if set
// Must be called without holding the mutex lock
func (q *Queue) notifyChange() {
	if q.onChange != nil {
		// Call in goroutine to avoid blocking and potential deadlocks
		go q.onChange()
	}
}

// Add adds a job to the queue
func (q *Queue) Add(job *Job) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job.ID == "" {
		job.ID = generateID()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	if job.Status == "" {
		job.Status = JobStatusPending
	}

	q.jobs = append(q.jobs, job)
	q.notifyChange()
}

// Remove removes a job from the queue by ID
func (q *Queue) Remove(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, job := range q.jobs {
		if job.ID == id {
			// Cancel if running
			if job.Status == JobStatusRunning && job.cancel != nil {
				job.cancel()
			}
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			q.notifyChange()
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

// Get retrieves a job by ID
func (q *Queue) Get(id string) (*Job, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, job := range q.jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

// List returns all jobs in the queue
func (q *Queue) List() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Job, len(q.jobs))
	copy(result, q.jobs)
	return result
}

// Stats returns queue statistics
func (q *Queue) Stats() (pending, running, completed, failed int) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, job := range q.jobs {
		switch job.Status {
		case JobStatusPending, JobStatusPaused:
			pending++
		case JobStatusRunning:
			running++
		case JobStatusCompleted:
			completed++
		case JobStatusFailed, JobStatusCancelled:
			failed++
		}
	}
	return
}

// Pause pauses a running job
func (q *Queue) Pause(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status != JobStatusRunning {
				return fmt.Errorf("job is not running")
			}
			if job.cancel != nil {
				job.cancel()
			}
			job.Status = JobStatusPaused
			q.notifyChange()
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

// Resume resumes a paused job
func (q *Queue) Resume(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status != JobStatusPaused {
				return fmt.Errorf("job is not paused")
			}
			job.Status = JobStatusPending
			q.notifyChange()
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

// Cancel cancels a job
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status == JobStatusRunning && job.cancel != nil {
				job.cancel()
			}
			job.Status = JobStatusCancelled
			q.notifyChange()
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

// Start starts processing jobs in the queue
func (q *Queue) Start() {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.mu.Unlock()

	go q.processJobs()
}

// Stop stops processing jobs
func (q *Queue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.running = false
}

// processJobs continuously processes pending jobs
func (q *Queue) processJobs() {
	for {
		q.mu.Lock()
		if !q.running {
			q.mu.Unlock()
			return
		}

		// Find highest priority pending job
		var nextJob *Job
		highestPriority := -1
		for _, job := range q.jobs {
			if job.Status == JobStatusPending && job.Priority > highestPriority {
				nextJob = job
				highestPriority = job.Priority
			}
		}

		if nextJob == nil {
			q.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Mark as running
		nextJob.Status = JobStatusRunning
		now := time.Now()
		nextJob.StartedAt = &now
		ctx, cancel := context.WithCancel(context.Background())
		nextJob.cancel = cancel

		q.mu.Unlock()
		q.notifyChange()

		// Execute job
		err := q.executor(ctx, nextJob, func(progress float64) {
			q.mu.Lock()
			nextJob.Progress = progress
			q.mu.Unlock()
			q.notifyChange()
		})

		// Update job status
		q.mu.Lock()
		now = time.Now()
		nextJob.CompletedAt = &now
		if err != nil {
			nextJob.Status = JobStatusFailed
			nextJob.Error = err.Error()
		} else {
			nextJob.Status = JobStatusCompleted
			nextJob.Progress = 100.0
		}
		nextJob.cancel = nil
		q.mu.Unlock()
		q.notifyChange()
	}
}

// Save saves the queue to a JSON file
func (q *Queue) Save(path string) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(q.jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	return nil
}

// Load loads the queue from a JSON file
func (q *Queue) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved queue, that's OK
		}
		return fmt.Errorf("failed to read queue file: %w", err)
	}

	var jobs []*Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return fmt.Errorf("failed to unmarshal queue: %w", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Reset running jobs to pending
	for _, job := range jobs {
		if job.Status == JobStatusRunning {
			job.Status = JobStatusPending
			job.Progress = 0
		}
	}

	q.jobs = jobs
	q.notifyChange()
	return nil
}

// Clear removes all completed, failed, and cancelled jobs
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	filtered := make([]*Job, 0)
	for _, job := range q.jobs {
		if job.Status == JobStatusPending || job.Status == JobStatusRunning || job.Status == JobStatusPaused {
			filtered = append(filtered, job)
		}
	}
	q.jobs = filtered
	q.notifyChange()
}

// generateID generates a unique ID for a job
func generateID() string {
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}
