package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

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
	JobTypeThumb     JobType = "thumb"
	JobTypeInspect   JobType = "inspect"
	JobTypeCompare   JobType = "compare"
	JobTypePlayer    JobType = "player"
	JobTypeBenchmark JobType = "benchmark"
	JobTypeSnippet   JobType = "snippet"
	JobTypeEditJob   JobType = "editjob" // NEW: editable jobs
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
	LogPath     string                 `json:"log_path,omitempty"`
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

// Stop stops processing jobs
func (q *Queue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
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
		nextJob.cancel = nil
		q.mu.Unlock()
		q.notifyChange()
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

// generateID generates a unique ID for a job
func generateID() string {
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
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

// EditJobStatus represents the edit state of a job
type EditJobStatus string

const (
	EditJobStatusOriginal  EditJobStatus = "original"  // Original job state
	EditJobStatusModified  EditJobStatus = "modified"  // Job has been modified
	EditJobStatusValidated EditJobStatus = "validated" // Job has been validated
	EditJobStatusApplied   EditJobStatus = "applied"   // Changes have been applied
)

// EditHistoryEntry tracks changes made to a job
type EditHistoryEntry struct {
	Timestamp    time.Time      `json:"timestamp"`
	OldCommand   *FFmpegCommand `json:"old_command,omitempty"`
	NewCommand   *FFmpegCommand `json:"new_command"`
	ChangeReason string         `json:"change_reason"`
	Applied      bool           `json:"applied"`
}

// FFmpegCommand represents a structured FFmpeg command
type FFmpegCommand struct {
	Executable string            `json:"executable"`
	Args       []string          `json:"args"`
	InputFile  string            `json:"input_file"`
	OutputFile string            `json:"output_file"`
	Options    map[string]string `json:"options,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EditableJob extends Job with editing capabilities
type EditableJob struct {
	*Job
	EditStatus      EditJobStatus      `json:"edit_status"`
	EditHistory     []EditHistoryEntry `json:"edit_history"`
	OriginalCommand *FFmpegCommand     `json:"original_command"`
	CurrentCommand  *FFmpegCommand     `json:"current_command"`
}

// EditJobManager manages job editing operations
type EditJobManager interface {
	// GetEditableJob returns an editable version of a job
	GetEditableJob(id string) (*EditableJob, error)

	// UpdateJobCommand updates a job's FFmpeg command
	UpdateJobCommand(id string, newCommand *FFmpegCommand, reason string) error

	// ValidateCommand validates an FFmpeg command
	ValidateCommand(cmd *FFmpegCommand) error

	// GetEditHistory returns the edit history for a job
	GetEditHistory(id string) ([]EditHistoryEntry, error)

	// ApplyEdit applies pending edits to a job
	ApplyEdit(id string) error

	// ResetToOriginal resets a job to its original command
	ResetToOriginal(id string) error

	// CreateEditableJob creates a new editable job
	CreateEditableJob(job *Job, cmd *FFmpegCommand) (*EditableJob, error)
}

// editJobManager implements EditJobManager
type editJobManager struct {
	queue *Queue
}

// NewEditJobManager creates a new edit job manager
func NewEditJobManager(queue *Queue) EditJobManager {
	return &editJobManager{queue: queue}
}

// GetEditableJob returns an editable version of a job
func (e *editJobManager) GetEditableJob(id string) (*EditableJob, error) {
	job, err := e.queue.Get(id)
	if err != nil {
		return nil, err
	}

	editable := &EditableJob{
		Job:         job,
		EditStatus:  EditJobStatusOriginal,
		EditHistory: make([]EditHistoryEntry, 0),
	}

	// Extract current command from job config if available
	if cmd, err := e.extractCommandFromJob(job); err == nil {
		editable.OriginalCommand = cmd
		editable.CurrentCommand = cmd
	}

	return editable, nil
}

// UpdateJobCommand updates a job's FFmpeg command
func (e *editJobManager) UpdateJobCommand(id string, newCommand *FFmpegCommand, reason string) error {
	job, err := e.queue.Get(id)
	if err != nil {
		return err
	}

	// Validate the new command
	if err := e.ValidateCommand(newCommand); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Create history entry
	oldCmd, _ := e.extractCommandFromJob(job)
	_ = EditHistoryEntry{
		Timestamp:    time.Now(),
		OldCommand:   oldCmd,
		NewCommand:   newCommand,
		ChangeReason: reason,
		Applied:      false,
	}

	// Update job config with new command
	if job.Config == nil {
		job.Config = make(map[string]interface{})
	}
	job.Config["ffmpeg_command"] = newCommand

	// Update job metadata
	job.Config["last_edited"] = time.Now().Format(time.RFC3339)
	job.Config["edit_reason"] = reason

	return nil
}

// ValidateCommand validates an FFmpeg command
func (e *editJobManager) ValidateCommand(cmd *FFmpegCommand) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if cmd.Executable == "" {
		return fmt.Errorf("executable cannot be empty")
	}

	if len(cmd.Args) == 0 {
		return fmt.Errorf("command arguments cannot be empty")
	}

	// Basic validation for input/output files
	if cmd.InputFile != "" && !strings.Contains(cmd.InputFile, "INPUT") {
		// Check if input file path is valid (basic check)
		if strings.HasPrefix(cmd.InputFile, "-") {
			return fmt.Errorf("input file cannot start with '-'")
		}
	}

	if cmd.OutputFile != "" && !strings.Contains(cmd.OutputFile, "OUTPUT") {
		// Check if output file path is valid (basic check)
		if strings.HasPrefix(cmd.OutputFile, "-") {
			return fmt.Errorf("output file cannot start with '-'")
		}
	}

	return nil
}

// GetEditHistory returns the edit history for a job
func (e *editJobManager) GetEditHistory(id string) ([]EditHistoryEntry, error) {
	job, err := e.queue.Get(id)
	if err != nil {
		return nil, err
	}

	// Extract history from job config
	if historyInterface, exists := job.Config["edit_history"]; exists {
		if historyBytes, err := json.Marshal(historyInterface); err == nil {
			var history []EditHistoryEntry
			if err := json.Unmarshal(historyBytes, &history); err == nil {
				return history, nil
			}
		}
	}

	return make([]EditHistoryEntry, 0), nil
}

// ApplyEdit applies pending edits to a job
func (e *editJobManager) ApplyEdit(id string) error {
	job, err := e.queue.Get(id)
	if err != nil {
		return err
	}

	// Mark edit as applied
	if job.Config == nil {
		job.Config = make(map[string]interface{})
	}
	job.Config["edit_applied"] = time.Now().Format(time.RFC3339)

	return nil
}

// ResetToOriginal resets a job to its original command
func (e *editJobManager) ResetToOriginal(id string) error {
	job, err := e.queue.Get(id)
	if err != nil {
		return err
	}

	// Get original command from job config
	if originalInterface, exists := job.Config["original_command"]; exists {
		if job.Config == nil {
			job.Config = make(map[string]interface{})
		}
		job.Config["ffmpeg_command"] = originalInterface
		job.Config["reset_to_original"] = time.Now().Format(time.RFC3339)
	}

	return nil
}

// CreateEditableJob creates a new editable job
func (e *editJobManager) CreateEditableJob(job *Job, cmd *FFmpegCommand) (*EditableJob, error) {
	if err := e.ValidateCommand(cmd); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	editable := &EditableJob{
		Job:             job,
		EditStatus:      EditJobStatusOriginal,
		EditHistory:     make([]EditHistoryEntry, 0),
		OriginalCommand: cmd,
		CurrentCommand:  cmd,
	}

	// Store command in job config
	if job.Config == nil {
		job.Config = make(map[string]interface{})
	}
	job.Config["ffmpeg_command"] = cmd
	job.Config["original_command"] = cmd

	return editable, nil
}

// extractCommandFromJob extracts FFmpeg command from job config
func (e *editJobManager) extractCommandFromJob(job *Job) (*FFmpegCommand, error) {
	if job.Config == nil {
		return nil, fmt.Errorf("job has no config")
	}

	if cmdInterface, exists := job.Config["ffmpeg_command"]; exists {
		if cmdBytes, err := json.Marshal(cmdInterface); err == nil {
			var cmd FFmpegCommand
			if err := json.Unmarshal(cmdBytes, &cmd); err == nil {
				return &cmd, nil
			}
		}
	}

	return nil, fmt.Errorf("no ffmpeg command found in job config")
}
