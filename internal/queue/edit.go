package queue

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
	history := EditHistoryEntry{
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

	// Add to edit history
	editHistory := []EditHistoryEntry{history}
	if existingHistoryInterface, exists := job.Config["edit_history"]; exists {
		if historyBytes, err := json.Marshal(existingHistoryInterface); err == nil {
			var existingHistory []EditHistoryEntry
			if err := json.Unmarshal(historyBytes, &existingHistory); err == nil {
				editHistory = append(existingHistory, history)
			}
		}
	}
	job.Config["edit_history"] = editHistory

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

// ToJSON converts FFmpegCommand to JSON string
func (cmd *FFmpegCommand) ToJSON() string {
	data, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

// FromJSON creates FFmpegCommand from JSON string
func FFmpegCommandFromJSON(jsonStr string) (*FFmpegCommand, error) {
	var cmd FFmpegCommand
	err := json.Unmarshal([]byte(jsonStr), &cmd)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &cmd, nil
}

// ToFullCommand converts FFmpegCommand to full command string
func (cmd *FFmpegCommand) ToFullCommand() string {
	if cmd == nil {
		return ""
	}

	args := []string{cmd.Executable}
	args = append(args, cmd.Args...)

	if cmd.InputFile != "" {
		args = append(args, "-i", cmd.InputFile)
	}

	if cmd.OutputFile != "" {
		args = append(args, cmd.OutputFile)
	}

	return strings.Join(args, " ")
}

// ValidateCommandStructure performs deeper validation of command structure
func ValidateCommandStructure(cmd *FFmpegCommand) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// Check for common FFmpeg patterns
	hasInput := false
	hasOutput := false

	for _, arg := range cmd.Args {
		if arg == "-i" && cmd.InputFile != "" {
			hasInput = true
		}
	}

	if cmd.InputFile != "" {
		hasInput = true
	}

	if cmd.OutputFile != "" {
		hasOutput = true
	}

	if !hasInput {
		return fmt.Errorf("command must specify an input file")
	}

	if !hasOutput {
		return fmt.Errorf("command must specify an output file")
	}

	// Check for conflicting options
	if cmd.Options != nil {
		if overwrite, exists := cmd.Options["overwrite"]; exists && overwrite == "false" {
			if cmd.OutputFile != "" && !strings.Contains(cmd.OutputFile, "OUTPUT") {
				// Real file path with overwrite disabled
				return fmt.Errorf("cannot overwrite existing file with overwrite disabled")
			}
		}
	}

	return nil
}
