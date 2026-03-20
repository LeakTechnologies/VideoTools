package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showQueue() {
	s.stopPreview()
	s.stopPlayer()
	// Only update the back target on a real forward navigation, not during back/forward jumps.
	if s.active != "queue" && !s.navigationHistorySuppress {
		s.lastModule = s.active
		s.queueBackTarget = s.active
	}
	s.active = "queue"
	s.pushNavigationHistory("queue")
	s.refreshQueueView()
	if s.queueView != nil {
		s.setContent(s.queueView.Root)
	}
	s.startQueueAutoRefresh()
	s.startQueueElapsedTicker()
}

// clearCompletedJobs removes all completed and failed jobs from the queue
func (s *appState) clearCompletedJobs() {
	if s.jobQueue != nil {
		s.jobQueue.Clear()
	}
}

// refreshQueueView rebuilds the queue UI while preserving scroll position and inline active conversion.
func (s *appState) refreshQueueView() {
	if s.active == "queue" {
		now := time.Now()
		if !s.queueLastRefresh.IsZero() && now.Sub(s.queueLastRefresh) < 200*time.Millisecond {
			return
		}
		s.queueLastRefresh = now
	}

	// Preserve current scroll offset if we already have a view
	if s.queueScroll != nil {
		s.queueOffset = s.queueScroll.Offset
	}

	jobs := s.jobQueue.List()
	// If a direct conversion is running but not represented in the queue, surface it as a pseudo job.
	if s.convertBusy {
		in := filepath.Base(s.convertActiveIn)
		if in == "" && s.source != nil {
			in = filepath.Base(s.source.Path)
		}
		out := filepath.Base(s.convertActiveOut)
		jobs = append([]*queue.Job{{
			ID:          "active-convert",
			Type:        queue.JobTypeConvert,
			Status:      queue.JobStatusRunning,
			Title:       fmt.Sprintf("Direct convert: %s", in),
			Description: fmt.Sprintf("Output: %s", out),
			Progress:    s.convertProgress,
			Config: map[string]interface{}{
				"fps":   s.convertFPS,
				"speed": s.convertSpeed,
				"eta":   s.convertETA,
			},
		}}, jobs...)
	}

	if s.queueView == nil {
		view := ui.BuildQueueView(
			jobs,
			func() { // onBack
				// Stop auto-refresh before navigating away for snappy response
				s.stopQueueAutoRefresh()
				target := s.queueBackTarget
				if target == "" {
					target = s.lastModule
				}
				if target != "" && target != "queue" && target != "menu" {
					s.showModule(target)
				} else {
					s.showMainMenu()
				}
			},
			func(id string) { // onPause
				if err := s.jobQueue.Pause(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to pause job: %v", err)
				}
			},
			func(id string) { // onResume
				if err := s.jobQueue.Resume(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to resume job: %v", err)
				}
			},
			func(id string) { // onCancel
				if err := s.jobQueue.Cancel(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to cancel job: %v", err)
				}
			},
			func(id string) { // onRemove
				if err := s.jobQueue.Remove(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to remove job: %v", err)
				}
			},
			func(id string) { // onMoveUp
				if err := s.jobQueue.MoveUp(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job up: %v", err)
				}
			},
			func(id string) { // onMoveDown
				if err := s.jobQueue.MoveDown(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job down: %v", err)
				}
			},
			func() { // onPauseAll
				s.jobQueue.PauseAll()
			},
			func() { // onResumeAll
				s.jobQueue.ResumeAll()
			},
			func() { // onStart
				s.jobQueue.ResumeAll()
			},
			func() { // onClear
				// Stop auto-refresh to prevent double UI updates
				s.stopQueueAutoRefresh()
				s.jobQueue.Clear()

				// Always return to main menu after clearing
				if len(s.jobQueue.List()) == 0 {
					s.showMainMenu()
				} else {
					// Restart auto-refresh and do single refresh
					s.startQueueAutoRefresh()
					s.refreshQueueView()
				}
			},
			func() { // onClearAll
				// Stop auto-refresh to prevent double UI updates during navigation
				s.stopQueueAutoRefresh()
				s.jobQueue.ClearAll()
				// Return to the module we were working on if possible
				if s.lastModule != "" && s.lastModule != "queue" && s.lastModule != "menu" {
					s.showModule(s.lastModule)
				} else {
					s.showMainMenu()
				}
			},
			func(id string) { // onCopyError
				job, err := s.jobQueue.Get(id)
				if err != nil {
					logging.Debug(logging.CatSystem, "copy error text failed: %v", err)
					return
				}
				var b strings.Builder
				b.WriteString("VideoTools Job Error\n")
				b.WriteString(fmt.Sprintf("Title: %s\n", job.Title))
				b.WriteString(fmt.Sprintf("Module: %s\n", string(job.Type)))
				if job.InputFile != "" {
					b.WriteString(fmt.Sprintf("Input: %s\n", job.InputFile))
				}
				if job.OutputFile != "" {
					b.WriteString(fmt.Sprintf("Output: %s\n", job.OutputFile))
				}
				errText := strings.TrimSpace(job.Error)
				if errText == "" {
					errText = "No error message recorded."
				}
				b.WriteString(fmt.Sprintf("Error: %s\n", errText))
				if job.LogPath != "" {
					b.WriteString(fmt.Sprintf("Log Path: %s\n", job.LogPath))
					const maxLines = 30
					if data, readErr := os.ReadFile(job.LogPath); readErr == nil {
						lines := strings.Split(string(data), "\n")
						if len(lines) > maxLines {
							lines = lines[len(lines)-maxLines:]
						}
						b.WriteString("Log Tail:\n")
						for _, line := range lines {
							if strings.TrimSpace(line) != "" {
								b.WriteString("  " + line + "\n")
							}
						}
					} else {
						b.WriteString(fmt.Sprintf("Log Tail: failed to read log (%v)\n", readErr))
					}
				}
				s.window.Clipboard().SetContent(strings.TrimSpace(b.String()))
			},
			func(id string) { // onViewLog
				job, err := s.jobQueue.Get(id)
				if err != nil {
					logging.Debug(logging.CatSystem, "view log failed: %v", err)
					return
				}
				path := strings.TrimSpace(job.LogPath)
				if path == "" {
					dialog.ShowInformation("No Log", "No log path recorded for this job.", s.window)
					return
				}
				data, err := os.ReadFile(path)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to read log: %w", err), s.window)
					return
				}
				text := widget.NewMultiLineEntry()
				text.SetText(string(data))
				text.Wrapping = fyne.TextWrapWord
				text.Disable()
				dialog.ShowCustom("Conversion Log", "Close", container.NewVScroll(text), s.window)
			},
			func(id string) { // onCopyCommand
				job, err := s.jobQueue.Get(id)
				if err != nil {
					logging.Debug(logging.CatSystem, "copy command failed: %v", err)
					return
				}
				cmdStr := buildFFmpegCommandFromJob(job)
				if cmdStr == "" {
					dialog.ShowInformation("No Command", "Unable to generate FFmpeg command for this job.", s.window)
					return
				}
				s.window.Clipboard().SetContent(cmdStr)
				dialog.ShowInformation("Copied", "FFmpeg command copied to clipboard", s.window)
			},
			func(id string) { // onOpenFolder
				job, err := s.jobQueue.Get(id)
				if err != nil || job.OutputFile == "" {
					return
				}
				_ = openFolder(filepath.Dir(job.OutputFile))
			},
			func(id string) { // onOpenOutput
				job, err := s.jobQueue.Get(id)
				if err != nil || job.OutputFile == "" {
					return
				}
				_ = openURL(job.OutputFile)
			},
			utils.MustHex("#4CE870"), // titleColor
			gridColor,                // bgColor
			textColor,                // textColor
		)

		s.queueView = view
		s.queueScroll = view.Scroll
		s.setContent(view.Root)

		// Restore scroll offset
		if s.queueScroll != nil && s.active == "queue" {
			savedOffset := s.queueOffset
			go func() {
				time.Sleep(10 * time.Millisecond)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if s.queueScroll != nil {
						s.queueScroll.Offset = savedOffset
						s.queueScroll.Refresh()
					}
				}, false)
			}()
		}
	} else {
		s.queueView.UpdateJobs(jobs)
	}
}

func (s *appState) startQueueAutoRefresh() {
	if s.queueAutoRefreshRunning {
		return
	}
	stop := make(chan struct{})
	s.queueAutoRefreshStop = stop
	s.queueAutoRefreshRunning = true
	go func() {
		// Short interval keeps elapsed/progress responsive with incremental UI updates.
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if s.active != "queue" {
					return
				}
				app := fyne.CurrentApp()
				if app == nil || app.Driver() == nil {
					continue
				}
				app.Driver().DoFromGoroutine(func() {
					if s.active == "queue" {
						s.refreshQueueView()
					}
				}, false)
			}
		}
	}()
}

func (s *appState) stopQueueAutoRefresh() {
	if !s.queueAutoRefreshRunning {
		return
	}
	if s.queueAutoRefreshStop != nil {
		close(s.queueAutoRefreshStop)
	}
	s.queueAutoRefreshStop = nil
	s.queueAutoRefreshRunning = false

	if s.queueView != nil {
		s.queueView.StopAnimations()
	}
}

func (s *appState) startQueueElapsedTicker() {
	if s.queueElapsedRunning {
		return
	}
	stop := make(chan struct{})
	s.queueElapsedStop = stop
	s.queueElapsedRunning = true
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if s.active != "queue" || s.queueView == nil {
					return
				}
				app := fyne.CurrentApp()
				if app == nil || app.Driver() == nil {
					continue
				}
				app.Driver().DoFromGoroutine(func() {
					if s.active == "queue" && s.queueView != nil {
						s.queueView.UpdateRunningStatus(s.jobQueue.List())
					}
				}, false)
			}
		}
	}()
}

func (s *appState) stopQueueElapsedTicker() {
	if !s.queueElapsedRunning {
		return
	}
	if s.queueElapsedStop != nil {
		close(s.queueElapsedStop)
	}
	s.queueElapsedStop = nil
	s.queueElapsedRunning = false
}
