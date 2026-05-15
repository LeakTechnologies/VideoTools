package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
)

// CommandEditor provides UI for editing FFmpeg commands
type CommandEditor struct {
	window      fyne.Window
	editManager queue.EditJobManager
	jobID       string

	// UI components
	jsonEntry   *widget.Entry
	validateBtn *widget.Button
	applyBtn    *widget.Button
	resetBtn    *widget.Button
	cancelBtn   *widget.Button
	statusLabel *widget.Label
	historyList *widget.List

	// Data
	editableJob *queue.EditableJob
	editHistory []queue.EditHistoryEntry
}

// CommandEditorConfig holds configuration for the command editor
type CommandEditorConfig struct {
	Window      fyne.Window
	EditManager queue.EditJobManager
	JobID       string
	Title       string
}

// NewCommandEditor creates a new command editor dialog
func NewCommandEditor(config CommandEditorConfig) *CommandEditor {
	editor := &CommandEditor{
		window:      config.Window,
		editManager: config.EditManager,
		jobID:       config.JobID,
	}

	// Load editable job
	editableJob, err := editor.editManager.GetEditableJob(config.JobID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to load job: %w", err), config.Window)
		return nil
	}
	editor.editableJob = editableJob

	// Load edit history
	history, err := editor.editManager.GetEditHistory(config.JobID)
	if err == nil {
		editor.editHistory = history
	}

	editor.buildUI(config.Title)
	return editor
}

// buildUI creates the command editor interface
func (e *CommandEditor) buildUI(title string) {
	// JSON editor with syntax highlighting
	e.jsonEntry = widget.NewMultiLineEntry()
	e.jsonEntry.SetPlaceHolder("FFmpeg command JSON will appear here...")
	e.jsonEntry.TextStyle = fyne.TextStyle{Monospace: true}

	// Load current command
	if e.editableJob.CurrentCommand != nil {
		e.jsonEntry.SetText(e.editableJob.CurrentCommand.ToJSON())
	}

	// Command validation status
	e.statusLabel = widget.NewLabel("Ready")
	e.statusLabel.Importance = widget.MediumImportance

	// Action buttons
	e.validateBtn = widget.NewButtonWithIcon("Validate", theme.ConfirmIcon(), e.validateCommand)
	e.validateBtn.Importance = widget.MediumImportance

	e.applyBtn = widget.NewButtonWithIcon("Apply Changes", theme.ConfirmIcon(), e.applyChanges)
	e.applyBtn.Importance = widget.HighImportance
	e.applyBtn.Disable()

	e.resetBtn = widget.NewButtonWithIcon("Reset to Original", theme.ViewRefreshIcon(), e.resetToOriginal)
	e.resetBtn.Importance = widget.MediumImportance

	e.cancelBtn = widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		e.close()
	})

	// Edit history list
	e.historyList = widget.NewList(
		func() int { return len(e.editHistory) },
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("Timestamp"),
				widget.NewLabel("Change Reason"),
				widget.NewSeparator(),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(e.editHistory) {
				return
			}

			entry := e.editHistory[id]
			vbox := obj.(*fyne.Container)
			timestamp := vbox.Objects[0].(*widget.Label)
			reason := vbox.Objects[1].(*widget.Label)

			timestamp.SetText(entry.Timestamp.Format(time.RFC822))
			reason.SetText(entry.ChangeReason)

			if entry.Applied {
				timestamp.Importance = widget.SuccessImportance
			}
		},
	)

	// Layout
	content := container.NewHSplit(
		container.NewVBox(
			widget.NewCard("Command Editor", "",
				container.NewVBox(
					widget.NewLabel("Edit FFmpeg command in JSON format:"),
					container.NewScroll(e.jsonEntry),
					e.statusLabel,
					container.NewHBox(
						e.validateBtn,
						e.applyBtn,
						e.resetBtn,
						layout.NewSpacer(),
						e.cancelBtn,
					),
				),
			),
		),
		container.NewVBox(
			widget.NewCard("Edit History", "", e.historyList),
			e.buildCommandPreview(),
		),
	)
	content.Resize(fyne.NewSize(900, 600))

	// Dialog
	dlg := dialog.NewCustom(title, "", content, e.window)
	dlg.Resize(fyne.NewSize(950, 650))
	dlg.Show()

	// Auto-validation on text change
	e.jsonEntry.OnChanged = func(text string) {
		e.applyBtn.Disable()
		e.statusLabel.SetText("Unsaved changes")
		e.statusLabel.Importance = widget.MediumImportance
	}
}

// validateCommand validates the current command
func (e *CommandEditor) validateCommand() {
	jsonText := e.jsonEntry.Text

	cmd, err := queue.FFmpegCommandFromJSON(jsonText)
	if err != nil {
		e.statusLabel.SetText(fmt.Sprintf("Invalid JSON: %v", err))
		e.statusLabel.Importance = widget.DangerImportance
		e.applyBtn.Disable()
		return
	}

	if err := e.editManager.ValidateCommand(cmd); err != nil {
		e.statusLabel.SetText(fmt.Sprintf("Invalid command: %v", err))
		e.statusLabel.Importance = widget.DangerImportance
		e.applyBtn.Disable()
		return
	}

	if err := queue.ValidateCommandStructure(cmd); err != nil {
		e.statusLabel.SetText(fmt.Sprintf("Command structure error: %v", err))
		e.statusLabel.Importance = widget.DangerImportance
		e.applyBtn.Disable()
		return
	}

	e.statusLabel.SetText("Valid command")
	e.statusLabel.Importance = widget.SuccessImportance
	e.applyBtn.Enable()
}

// applyChanges applies the edited command
func (e *CommandEditor) applyChanges() {
	jsonText := e.jsonEntry.Text

	cmd, err := queue.FFmpegCommandFromJSON(jsonText)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Invalid JSON: %w", err), e.window)
		return
	}

	// Show reason dialog
	reasonEntry := widget.NewEntry()
	reasonEntry.SetPlaceHolder("Enter reason for change...")

	content := container.NewVBox(
		widget.NewLabel("Please enter a reason for this change:"),
		reasonEntry,
	)
	buttons := container.NewHBox(
		widget.NewButton("Cancel", func() {}),
		widget.NewButton("Apply", func() {
			reason := reasonEntry.Text
			if reason == "" {
				reason = "Manual edit via command editor"
			}

			if err := e.editManager.UpdateJobCommand(e.jobID, cmd, reason); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to update job: %w", err), e.window)
				return
			}

			if err := e.editManager.ApplyEdit(e.jobID); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to apply edit: %w", err), e.window)
				return
			}

			dialog.ShowInformation("Success", "Command updated successfully", e.window)
			e.refreshData()
			e.close()
		}),
	)

	reasonDlg := dialog.NewCustom("Apply Changes", "OK", content, e.window)
	reasonDlg.SetOnClosed(func() {
		// Handle button clicks manually
	})

	// Create a custom dialog layout
	dialogContent := container.NewVBox(content, buttons)
	customDlg := dialog.NewCustomWithoutButtons("Apply Changes", dialogContent, e.window)
	customDlg.Show()
	reasonDlg.Show()
}

// resetToOriginal resets the command to original
func (e *CommandEditor) resetToOriginal() {
	if e.editableJob.OriginalCommand == nil {
		dialog.ShowInformation("Info", "No original command available", e.window)
		return
	}

	confirmDlg := dialog.NewConfirm("Reset Command",
		"Are you sure you want to reset to the original command? This will discard all current changes.",
		func(confirmed bool) {
			if confirmed {
				e.jsonEntry.SetText(e.editableJob.OriginalCommand.ToJSON())
				e.statusLabel.SetText("Reset to original")
				e.statusLabel.Importance = widget.MediumImportance
				e.applyBtn.Disable()
			}
		}, e.window)
	confirmDlg.Show()
}

// buildCommandPreview creates a preview of the command
func (e *CommandEditor) buildCommandPreview() fyne.CanvasObject {
	previewLabel := widget.NewLabel("")
	previewLabel.TextStyle = fyne.TextStyle{Monospace: true}
	previewLabel.Wrapping = fyne.TextWrapBreak

	refreshPreview := func() {
		jsonText := e.jsonEntry.Text
		cmd, err := queue.FFmpegCommandFromJSON(jsonText)
		if err != nil {
			previewLabel.SetText("Invalid command")
			return
		}
		previewLabel.SetText(cmd.ToFullCommand())
	}

	// Initial preview
	refreshPreview()

	// Update preview on text change
	e.jsonEntry.OnChanged = func(text string) {
		refreshPreview()
		e.applyBtn.Disable()
		e.statusLabel.SetText("Unsaved changes")
		e.statusLabel.Importance = widget.MediumImportance
	}

	return widget.NewCard("Command Preview", "",
		container.NewScroll(previewLabel))
}

// refreshData refreshes the editor data
func (e *CommandEditor) refreshData() {
	// Reload editable job
	editableJob, err := e.editManager.GetEditableJob(e.jobID)
	if err == nil {
		e.editableJob = editableJob
	}

	// Reload history
	history, err := e.editManager.GetEditHistory(e.jobID)
	if err == nil {
		e.editHistory = history
		e.historyList.Refresh()
	}
}

// close closes the editor
func (e *CommandEditor) close() {
	// Close dialog by finding parent dialog
	// This is a workaround since Fyne doesn't expose direct dialog closing
	for _, win := range fyne.CurrentApp().Driver().AllWindows() {
		if win.Title() == "Command Editor" || strings.Contains(win.Title(), "Edit Job") {
			win.Close()
			break
		}
	}
}

// ShowCommandEditorDialog shows a command editor for a specific job
func ShowCommandEditorDialog(window fyne.Window, editManager queue.EditJobManager, jobID, jobTitle string) {
	config := CommandEditorConfig{
		Window:      window,
		EditManager: editManager,
		JobID:       jobID,
		Title:       fmt.Sprintf("Edit Job: %s", jobTitle),
	}

	NewCommandEditor(config)
}

// CreateCommandEditorButton creates a button that opens the command editor
func CreateCommandEditorButton(window fyne.Window, editManager queue.EditJobManager, jobID, jobTitle string) *widget.Button {
	btn := widget.NewButtonWithIcon("Edit Command", theme.DocumentCreateIcon(), func() {
		ShowCommandEditorDialog(window, editManager, jobID, jobTitle)
	})
	btn.Importance = widget.MediumImportance
	return btn
}
