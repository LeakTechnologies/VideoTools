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

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	queuepkg "git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showQueue() {
	s.stopPreview()
	s.stopPlayer()
	if s.active != "queue" && !s.navigationHistorySuppress {
		s.lastModule = s.active
		s.queueBackTarget = s.active
	}
	s.active = "queue"
	s.pushNavigationHistory("queue")
	s.refreshQueueView()
	if s.queueView != nil {
		s.setContent(s.queueView.GetRoot())
	}
	s.startQueueAutoRefresh()
	s.startQueueElapsedTicker()
}

func (s *appState) clearCompletedJobs() {
	if s.jobQueue != nil {
		s.jobQueue.Clear()
	}
}

func (s *appState) refreshQueueView() {
	if s.active == "queue" {
		now := time.Now()
		if !s.queueLastRefresh.IsZero() && now.Sub(s.queueLastRefresh) < 200*time.Millisecond {
			return
		}
		s.queueLastRefresh = now
	}

	if s.queueScroll != nil {
		s.queueOffset = s.queueScroll.Offset
	}

	jobs := s.jobQueue.List()
	if s.convertBusy {
		in := filepath.Base(s.convertActiveIn)
		if in == "" && s.source != nil {
			in = filepath.Base(s.source.Path)
		}
		out := filepath.Base(s.convertActiveOut)
		jobs = append([]*queuepkg.Job{{
			Type:        queuepkg.JobTypeConvert,
			Status:      queuepkg.JobStatusRunning,
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
		opts := queue.Options{
			Window: s.window,
			Jobs:   jobs,
			OnStopPreview: func() {
				s.stopPreview()
			},
			OnBack: func() {
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
			OnPause: func(id string) {
				if err := s.jobQueue.Pause(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to pause job: %v", err)
				}
			},
			OnResume: func(id string) {
				if err := s.jobQueue.Resume(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to resume job: %v", err)
				}
			},
			OnCancel: func(id string) {
				if err := s.jobQueue.Cancel(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to cancel job: %v", err)
				}
			},
			OnRemove: func(id string) {
				job, jobErr := s.jobQueue.Get(id)
				if err := s.jobQueue.Remove(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to remove job: %v", err)
					return
				}
				// Sync loadedVideos: remove the video whose path matches this job's input
				if jobErr == nil && job.InputFile != "" {
					for i, v := range s.loadedVideos {
						if v.Path == job.InputFile {
							s.loadedVideos = append(s.loadedVideos[:i], s.loadedVideos[i+1:]...)
							// Clamp currentIndex to valid range
							if s.currentIndex >= len(s.loadedVideos) {
								s.currentIndex = len(s.loadedVideos) - 1
							}
							if s.currentIndex < 0 {
								s.currentIndex = 0
							}
							break
						}
					}
				}
			},
			OnMoveUp: func(id string) {
				if err := s.jobQueue.MoveUp(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job up: %v", err)
				}
			},
			OnMoveDown: func(id string) {
				if err := s.jobQueue.MoveDown(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job down: %v", err)
				}
			},
			OnPauseAll: func() {
				s.jobQueue.PauseAll()
			},
			OnResumeAll: func() {
				s.jobQueue.ResumeAll()
			},
			OnStart: func() {
				s.jobQueue.ResumeAll()
			},
			OnClear: func() {
				s.jobQueue.Clear()
				if len(s.jobQueue.List()) == 0 {
					s.showMainMenu()
				} else {
					s.startQueueAutoRefresh()
					s.refreshQueueView()
				}
			},
			OnClearAll: func() {
				s.jobQueue.ClearAll()
				if s.lastModule != "" && s.lastModule != "queue" && s.lastModule != "menu" {
					s.showModule(s.lastModule)
				} else {
					s.showMainMenu()
				}
			},
			OnCancelAll: func() {
				s.jobQueue.CancelAll()
				s.refreshQueueView()
			},
			OnRetry: func(id string) {
				if err := s.jobQueue.RetryJob(id); err != nil {
					logging.Debug(logging.CatSystem, "retry job failed: %v", err)
				}
			},
			OnCopyError: func(id string) {
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
			OnViewLog: func(id string) {
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
			OnCopyCommand: func(id string) {
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
			OnOpenFolder: func(id string) {
				job, err := s.jobQueue.Get(id)
				if err != nil || job.OutputFile == "" {
					return
				}
				_ = openFolder(filepath.Dir(job.OutputFile))
			},
			OnOpenOutput: func(id string) {
				job, err := s.jobQueue.Get(id)
				if err != nil {
					logging.Debug(logging.CatSystem, "OnOpenOutput: job not found: %s", id)
					return
				}
				if job.OutputFile == "" {
					logging.Debug(logging.CatSystem, "OnOpenOutput: no output file for job: %s", id)
					return
				}
				outputFile := job.OutputFile
				if info, err := os.Stat(outputFile); err == nil && info.IsDir() {
					_ = openFolder(outputFile)
					return
				}
				ext := strings.ToLower(filepath.Ext(outputFile))
				isVideo := false
				switch ext {
				case ".mp4", ".mkv", ".avi", ".mov", ".mpg", ".mpeg", ".ts", ".m2ts", ".wmv", ".flv", ".webm", ".m4v", ".vob":
					isVideo = true
				}
				if ext == ".iso" {
					t := i18n.T()
					dialog.ShowInformation(t.DialogISOCannotOpen, t.DialogISOCannotOpenMsg, s.window)
					return
				}
				if isVideo && s.prefs.QueuePlayBehavior == "inspect" {
					s.showInspectViewForPath(outputFile)
				} else if isVideo {
					s.showPlayerViewForPath(outputFile)
				} else {
					_ = openURL(outputFile)
				}
			},
			OnBurnISO: func(id string) {
				t := i18n.T()
				dialog.ShowInformation(t.DialogBurnComingSoon, t.DialogBurnComingSoonMsg, s.window)
			},
			OnOpenInModule: func(jobID, module string) {
				job, err := s.jobQueue.Get(jobID)
				if err != nil {
					logging.Debug(logging.CatSystem, "OnOpenInModule: job not found: %s", jobID)
					return
				}
				if job.OutputFile == "" {
					logging.Debug(logging.CatSystem, "OnOpenInModule: no output file for job: %s", jobID)
					return
				}
				outputFile := job.OutputFile
				if info, err := os.Stat(outputFile); err == nil && info.IsDir() {
					_ = openFolder(outputFile)
					return
				}
				src := &videoSource{Path: outputFile, DisplayName: filepath.Base(outputFile)}
				switch module {
				case "convert":
					s.source = src
					s.showConvertView(src)
				case "inspect":
					s.showInspectViewForPath(outputFile)
				case "audio":
					s.showAudioView()
				}
			},
			OnScheduleModule: func(jobID, module string) {
				logging.Info(logging.CatSystem, "scheduled module %s for job %s on completion", module, jobID)
			},
			TitleColor: utils.MustHex("#4CE870"),
			BgColor:    gridColor,
			TextColor:  textColor,
			StatsBar:  s.statsBar,
		}

		_, view := queue.BuildView(opts)

		s.queueView = view
		s.queueScroll = view.GetScroll()
		s.setContent(view.GetRoot())

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
