package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

var jobSeq uint64 // monotonic counter; avoids timestamp collisions on Windows

// JobType represents the type of job to execute
type JobType string

const (
	JobTypeConvert   JobType = "convert"
	JobTypeMerge     JobType = "merge"
	JobTypeTrim      JobType = "trim"
	JobTypeFilter    JobType = "filters"
	JobTypeUpscale   JobType = "upscale"
	JobTypeAudio     JobType = "audio"
	JobTypeAuthor    JobType = "author"
	JobTypeRip       JobType = "rip"
	JobTypeBluray    JobType = "bluray"
	JobTypeSubtitles JobType = "subtitles"
	JobTypeThumbnail JobType = "thumbnail"
	JobTypeInspect   JobType = "inspect"
	JobTypeCompare   JobType = "compare"
	JobTypePlayer    JobType = "player"
	JobTypeBenchmark JobType = "benchmark"
	JobTypeSnippet   JobType = "snippet"
	JobTypeEditJob   JobType = "editjob" // NEW: editable jobs
	JobTypeBurn      JobType = "burn"
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
	ThumbnailPath string               `json:"thumbnail_path,omitempty"`           // Midpoint thumbnail for queue display
	LogPath     string                 `json:"log_path,omitempty"`
	Config      map[string]interface{} `json:"config"`
	Progress    float64                `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Priority               int                    `json:"priority"`                          // Higher priority = runs first
	GroupID                string                 `json:"group_id,omitempty"`                // Batch group ID for related jobs
	PipelineAfter          string                 `json:"pipeline_after,omitempty"`          // Job ID that must complete before this runs
	PipelineDeleteOnSuccess string                `json:"pipeline_delete_on_success,omitempty"` // Intermediate file to delete after success
	cancel                 context.CancelFunc     `json:"-"`
}

// JobExecutor is a function that executes a job
type JobExecutor func(ctx context.Context, job *Job, progressCallback func(float64)) error

// Queue manages a queue of jobs
type Queue struct {
	jobs     []*Job
	executor JobExecutor
	running  bool
	mu       sync.RWMutex
	onChange func()       // Callback when queue state changes
	notifyCh chan struct{} // capacity-1 coalescing channel; see notifyChange
}

// New creates a new queue with the given executor
func New(executor JobExecutor) *Queue {
	q := &Queue{
		jobs:     make([]*Job, 0),
		executor: executor,
		running:  false,
		notifyCh: make(chan struct{}, 1),
	}
	go q.notifyWorker()
	return q
}

// SetChangeCallback sets a callback to be called when the queue state changes
func (q *Queue) SetChangeCallback(callback func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onChange = callback
}

// notifyChange signals the worker that queue state has changed.
// Must be called without holding the mutex. Rapid successive calls are
// coalesced: if a notification is already pending the send is skipped and
// the in-flight worker pass will observe the latest state when it fires.
func (q *Queue) notifyChange() {
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

// notifyWorker is a single long-lived goroutine that serialises onChange
// callbacks. It replaces the previous pattern of spawning a new goroutine
// on every notifyChange call, which caused unbounded goroutine fan-out
// during high-frequency progress updates on long encode jobs.
func (q *Queue) notifyWorker() {
	for range q.notifyCh {
		q.mu.RLock()
		fn := q.onChange
		q.mu.RUnlock()
		if fn != nil {
			fn()
		}
	}
}

// Add adds a job to the queue (at the end)
func (q *Queue) Add(job *Job) {
	q.mu.Lock()

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
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
}

// AddNext adds a job to the front of the pending queue (right after any running job)
func (q *Queue) AddNext(job *Job) {
	q.mu.Lock()

	if job.ID == "" {
		job.ID = generateID()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	if job.Status == "" {
		job.Status = JobStatusPending
	}

	// Find the position after any running jobs
	insertPos := 0
	for i, j := range q.jobs {
		if j.Status == JobStatusRunning {
			insertPos = i + 1
		} else {
			break
		}
	}

	// Insert at the calculated position
	q.jobs = append(q.jobs[:insertPos], append([]*Job{job}, q.jobs[insertPos:]...)...)
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
}

// Remove removes a job from the queue by ID
func (q *Queue) Remove(id string) error {
	q.mu.Lock()

	var removed bool

	for i, job := range q.jobs {
		if job.ID == id {
			// Cancel if running
			if job.Status == JobStatusRunning && job.cancel != nil {
				job.cancel()
			}
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			q.rebalancePrioritiesLocked()
			removed = true
			break
		}
	}
	q.mu.Unlock()
	if removed {
		q.notifyChange()
		return nil
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

// RetryJob creates a fresh copy of a failed or cancelled job and adds it to the queue.
func (q *Queue) RetryJob(id string) error {
	q.mu.RLock()
	var original *Job
	for _, j := range q.jobs {
		if j.ID == id {
			original = j
			break
		}
	}
	q.mu.RUnlock()

	if original == nil {
		return fmt.Errorf("job not found: %s", id)
	}
	if original.Status != JobStatusFailed && original.Status != JobStatusCancelled {
		return fmt.Errorf("job %s is not in a retryable state (%s)", id, original.Status)
	}

	// Deep-copy config map so the retry is independent of the original.
	configCopy := make(map[string]interface{}, len(original.Config))
	for k, v := range original.Config {
		configCopy[k] = v
	}

	retry := &Job{
		Type:        original.Type,
		Status:      JobStatusPending,
		Title:       original.Title,
		Description: original.Description,
		InputFile:   original.InputFile,
		OutputFile:  original.OutputFile,
		Config:      configCopy,
		Priority:    original.Priority,
		CreatedAt:   time.Now(),
	}
	q.Add(retry)
	return nil
}

// List returns all jobs in the queue
func (q *Queue) List() []*Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Return a copy of the jobs to avoid races on the live queue state
	result := make([]*Job, len(q.jobs))
	for i, job := range q.jobs {
		clone := *job
		result[i] = &clone
	}
	return result
}

// Stats returns queue statistics
func (q *Queue) Stats() (pending, running, completed, failed, cancelled int) {
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
		case JobStatusFailed:
			failed++
		case JobStatusCancelled:
			cancelled++
		}
	}
	return
}

// CurrentRunning returns the currently running job, if any.
func (q *Queue) CurrentRunning() *Job {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, job := range q.jobs {
		if job.Status == JobStatusRunning {
			clone := *job
			return &clone
		}
	}
	return nil
}

// Pause pauses a running job
func (q *Queue) Pause(id string) error {
	q.mu.Lock()

	result := fmt.Errorf("job not found: %s", id)

	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status != JobStatusRunning {
				result = fmt.Errorf("job is not running")
				break
			}
			if job.cancel != nil {
				job.cancel()
			}
			job.Status = JobStatusPaused
			// Keep position; just stop current run
			result = nil
			break
		}
	}
	q.mu.Unlock()
	if result == nil {
		q.notifyChange()
	}
	return result
}

// Resume resumes a paused job
func (q *Queue) Resume(id string) error {
	q.mu.Lock()

	result := fmt.Errorf("job not found: %s", id)

	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status != JobStatusPaused {
				result = fmt.Errorf("job is not paused")
				break
			}
			job.Status = JobStatusPending
			// Keep position; move selection via priorities
			result = nil
			break
		}
	}
	q.mu.Unlock()
	if result == nil {
		q.notifyChange()
	}
	return result
}

// Cancel cancels a job
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()

	var cancelled bool
	now := time.Now()
	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status == JobStatusRunning && job.cancel != nil {
				job.cancel()
			}
			job.Status = JobStatusCancelled
			job.CompletedAt = &now
			q.rebalancePrioritiesLocked()
			cancelled = true
			break
		}
	}
	q.mu.Unlock()
	if cancelled {
		q.notifyChange()
		return nil
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

// Stop stops processing jobs and cancels any currently running job.
func (q *Queue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cancelRunningLocked()
	q.running = false
}

// IsRunning returns true if the queue is currently processing jobs
func (q *Queue) IsRunning() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.running
}

// PauseAll pauses any running job and stops processing
func (q *Queue) PauseAll() {
	q.mu.Lock()
	for _, job := range q.jobs {
		if job.Status == JobStatusRunning && job.cancel != nil {
			job.cancel()
			job.Status = JobStatusPaused
			job.cancel = nil
			job.StartedAt = nil
			job.CompletedAt = nil
			job.Error = ""
		}
	}
	q.running = false
	q.mu.Unlock()
	q.notifyChange()
}

// ResumeAll restarts processing the queue
func (q *Queue) ResumeAll() {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.mu.Unlock()
	q.notifyChange()
	go q.processJobs()
}

// findJobByIDLocked returns the job with the given ID; caller must hold mu.
func (q *Queue) findJobByIDLocked(id string) *Job {
	for _, j := range q.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// processJobs continuously processes pending jobs
func (q *Queue) processJobs() {
	defer logging.RecoverPanic() // Catch and log any panics in job processing
	for {
		q.mu.Lock()
		if !q.running {
			q.mu.Unlock()
			return
		}

		// Check if there's already a running job (only process one at a time)
		hasRunningJob := false
		for _, job := range q.jobs {
			if job.Status == JobStatusRunning {
				hasRunningJob = true
				break
			}
		}

		// If a job is already running, wait and check again later
		if hasRunningJob {
			q.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Find highest priority pending job that is not blocked by a pipeline dependency
		var nextJob *Job
		highestPriority := -1
		for _, job := range q.jobs {
			if job.Status != JobStatusPending {
				continue
			}
			if job.PipelineAfter != "" {
				upstream := q.findJobByIDLocked(job.PipelineAfter)
				if upstream == nil || upstream.Status != JobStatusCompleted {
					continue // blocked — upstream not done yet
				}
			}
			if job.Priority > highestPriority {
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
		if err != nil {
			if ctx.Err() == context.Canceled {
				if nextJob.Status == JobStatusPaused {
					// Leave as paused without timestamps/error
					nextJob.StartedAt = nil
					nextJob.CompletedAt = nil
					nextJob.Error = ""
				} else {
					// Cancelled
					nextJob.Status = JobStatusCancelled
					nextJob.CompletedAt = &now
					nextJob.Error = ""
				}
			} else {
				nextJob.Status = JobStatusFailed
				nextJob.CompletedAt = &now
				nextJob.Error = err.Error()
			}
		} else {
			nextJob.Status = JobStatusCompleted
			nextJob.Progress = 100.0
			nextJob.CompletedAt = &now
		}
		deleteIntermediate := nextJob.PipelineDeleteOnSuccess
		nextJob.cancel = nil
		q.mu.Unlock()
		q.notifyChange()

		if deleteIntermediate != "" {
			if err := os.Remove(deleteIntermediate); err != nil && !os.IsNotExist(err) {
				logging.Warning(logging.CatQueue, "pipeline cleanup: failed to delete intermediate %s: %v", deleteIntermediate, err)
			} else {
				logging.Debug(logging.CatQueue, "pipeline cleanup: deleted intermediate %s", deleteIntermediate)
			}
		}
	}
}

// MoveUp moves a pending or paused job one position up in the queue
func (q *Queue) MoveUp(id string) error {
	return q.move(id, -1)
}

// MoveDown moves a pending or paused job one position down in the queue
func (q *Queue) MoveDown(id string) error {
	return q.move(id, 1)
}

func (q *Queue) move(id string, delta int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	var idx int = -1
	for i, job := range q.jobs {
		if job.ID == id {
			idx = i
			if job.Status != JobStatusPending && job.Status != JobStatusPaused {
				return fmt.Errorf("job must be pending or paused to reorder")
			}
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("job not found: %s", id)
	}

	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(q.jobs) {
		return nil // already at boundary; no-op
	}

	q.jobs[idx], q.jobs[newIdx] = q.jobs[newIdx], q.jobs[idx]
	q.rebalancePrioritiesLocked()
	return nil
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

	// Reset running jobs to pending
	for _, job := range jobs {
		if job.Status == JobStatusRunning {
			job.Status = JobStatusPending
			job.Progress = 0
		}
	}

	q.jobs = jobs
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
	return nil
}

// Clear removes all completed, failed, and cancelled jobs
func (q *Queue) Clear() {
	q.mu.Lock()

	// Keep only pending, running, and paused jobs
	filtered := make([]*Job, 0)
	for _, job := range q.jobs {
		if job.Status == JobStatusPending || job.Status == JobStatusRunning || job.Status == JobStatusPaused {
			filtered = append(filtered, job)
		}
	}
	q.jobs = filtered
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
}

// CancelAll cancels the running job and marks all pending/paused jobs as cancelled.
// Jobs remain visible in the queue (not removed). The processor keeps running.
func (q *Queue) CancelAll() {
	q.mu.Lock()
	now := time.Now()
	for _, job := range q.jobs {
		switch job.Status {
		case JobStatusRunning:
			if job.cancel != nil {
				job.cancel()
				job.cancel = nil
			}
			job.Status = JobStatusCancelled
			job.CompletedAt = &now
		case JobStatusPending, JobStatusPaused:
			job.Status = JobStatusCancelled
			job.CompletedAt = &now
		}
	}
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
}

// ClearAll removes all jobs from the queue
func (q *Queue) ClearAll() {
	q.mu.Lock()

	// Cancel any running work and stop the processor
	q.cancelRunningLocked()
	q.running = false

	q.jobs = make([]*Job, 0)
	q.rebalancePrioritiesLocked()
	q.mu.Unlock()
	q.notifyChange()
}

// generateID generates a unique ID for a job.
// Uses a monotonic counter so that batch-adds within the same nanosecond
// (common on Windows where the system timer resolution can be ≥100 ns) still
// produce distinct IDs.
func generateID() string {
	seq := atomic.AddUint64(&jobSeq, 1)
	return fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), seq)
}

// rebalancePrioritiesLocked assigns descending priorities so earlier items are selected first
func (q *Queue) rebalancePrioritiesLocked() {
	for i := range q.jobs {
		q.jobs[i].Priority = len(q.jobs) - i
	}
}

// cancelRunningLocked cancels any currently running job and marks it cancelled.
func (q *Queue) cancelRunningLocked() {
	now := time.Now()
	for _, job := range q.jobs {
		if job.Status == JobStatusRunning {
			if job.cancel != nil {
				job.cancel()
			}
			job.Status = JobStatusCancelled
			job.CompletedAt = &now
		}
	}
}
