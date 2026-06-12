package main

import (
	"fmt"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	mainmenumodule "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modules/mainmenu"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

// moduleLabel returns the translated label for a given module ID.
// Called on each showMainMenu() rebuild so labels reflect the active language.
func moduleLabel(id string) string {
	t := i18n.T()
	switch id {
	case "convert":
		return t.ModuleConvert
	case "merge":
		return t.ModuleMerge
	case "trim":
		return t.ModuleTrim
	case "filters":
		return t.ModuleFilters
	case "upscale":
		return t.ModuleUpscale
	case "audio":
		return t.ModuleAudio
	case "author":
		return t.ModuleAuthor
	case "rip":
		return t.ModuleRip
	case "burn":
		return t.ModuleBurn
	case "filemanager":
		return t.ModuleFileManager
	case "bluray":
		return t.ModuleBluRay
	case "subtitles":
		return t.ModuleSubtitles
	case "thumbnail":
		return t.ModuleThumbnail
	case "compare":
		return t.ModuleCompare
	case "inspect":
		return t.ModuleInspect
	case "player":
		return t.ModulePlayer
	case "settings":
		return t.ModuleSettings
	default:
		return id
	}
}

// categoryLabel returns the translated label for a given category ID.
func categoryLabel(id string) string {
	t := i18n.T()
	switch id {
	case "Convert":
		return t.CategoryConvert
	case "Inspect":
		return t.CategoryInspect
	case "Disc":
		return t.CategoryDisc
	case "Playback":
		return t.CategoryPlayback
	case "Advanced":
		return t.CategoryAdvanced
	case "Screenshots":
		return t.CategoryScreenshots
	case "Settings":
		return t.CategorySettings
	default:
		return id
	}
}

func (s *appState) showMainMenu() {
	s.stopPreview()
	s.stopPlayer()
	s.stopQueueAutoRefresh()
	s.stopQueueElapsedTicker()
	if s.queueView != nil {
		s.queueView.StopAnimations()
	}
	s.active = ""
	s.queueBackTarget = ""

	// Track navigation history
	s.pushNavigationHistory("mainmenu")

	// Convert modules to UI metadata with preference/dependency filtering.
	// Labels are resolved from i18n.T() so they update on language change.
	sourceMods := make([]mainmenumodule.SourceModule, 0, len(modulesList))
	for _, m := range modulesList {
		sourceMods = append(sourceMods, mainmenumodule.SourceModule{
			ID:            m.ID,
			Label:         moduleLabel(m.ID),
			Color:         m.Color,
			TextColor:     m.TextColor,
			Category:      categoryLabel(m.Category),
			HasHandler:    m.Handle != nil,
			DepsAvailable: isModuleAvailable(m.ID),
		})
	}
	// Native Go engine enables disc modules cross-platform.
	mods := mainmenumodule.BuildVisibleModules(sourceMods, mainmenumodule.Visibility{
		ShowUpscale: s.convert.ShowUpscale,
		ShowDisc:    s.convert.ShowDisc,
		IsDevBuild:  isDevBuild(),
	})
	ids := make([]string, len(mods))
	for i, m := range mods {
		ids[i] = m.ID
	}
	s.visibleModuleIDs = ids

	// In pipeline step2 mode: dim modules that cannot serve as step2 targets.
	// Invalid step2 = anything that doesn't accept a single video as input and
	// produce a single processed output (disc tools, comparison, playback, settings).
	if s.pipelineStep == "step2" {
		invalidStep2 := map[string]bool{
			"rip": true, "burn": true, "author": true,
			"inspect": true, "compare": true, "player": true,
			"filemanager": true, "settings": true, "merge": true,
		}
		for i := range mods {
			if invalidStep2[mods[i].ID] {
				mods[i].Enabled = false
				mods[i].MissingDependencies = false
			}
		}
	}

	titleColor := utils.MustHex("#4CE870")
	t := i18n.T()
	menuLabels := ui.MenuLabels{
		Logs:                t.MenuLogs,
		Files:               t.MenuFiles,
		FilesOpen:           t.MenuFilesOpen,
		FilesRecent:         t.MenuFilesRecent,
		FilesOpenFolder:     t.MenuFilesOpenFolder,
		FilesAddMore:        t.MenuFilesAddMore,
		FilesGoTo:           t.MenuFilesGoTo,
		Window:              s.window,
		Queue:               t.MenuQueue,
		CategoryConvert:     t.CategoryConvert,
		CategoryInspect:     t.CategoryInspect,
		CategoryDisc:        t.CategoryDisc,
		CategoryPlayback:    t.CategoryPlayback,
		CategoryAdvanced:    t.CategoryAdvanced,
		CategoryScreenshots: t.CategoryScreenshots,
		CategorySettings:    t.CategorySettings,
		HistoryTitle:        t.HistoryTitle,
		HistoryInProgress:   t.QueueInProgress,
		HistoryCompleted:    t.QueueCompleted,
		HistoryFailed:       t.QueueFailed,
		HistoryClearAll:     t.ActionClearAll,
		HistoryNoEntries:    t.HistoryNoEntries,
	}

	// PERFORMANCE: Cache queue list to avoid multiple expensive copies
	var queueList []*queue.Job
	if s.jobQueue != nil {
		queueList = s.jobQueue.List()
	}

	// Get queue stats - show completed jobs out of total
	var queueCompleted, queueTotal int
	if s.jobQueue != nil {
		_, _, completed, _, _ := s.jobQueue.Stats()
		queueCompleted = completed
		queueTotal = len(queueList)
	}

	// Build sidebar if visible
	var sidebar fyne.CanvasObject
	if s.sidebarVisible {
		activeJobs := mainmenumodule.BuildActiveJobs(queueList)

		onHistoryClick := func(entry ui.HistoryEntry) {
			if entry.Status == queue.JobStatusRunning || entry.Status == queue.JobStatusPending {
				s.showQueue()
				return
			}
			s.showHistoryDetails(entry)
		}
		sidebar = ui.BuildHistorySidebar(
			menuLabels,
			s.historyEntries,
			activeJobs,
			onHistoryClick,
			s.deleteHistoryEntry,
			s.clearHistoryEntries,
			s.historyTabIdx,
			func(idx int) {
				s.historyTabIdx = idx
			},
			func() {
				s.sidebarVisible = false
				s.refreshMainMenuThrottled()
			},
			titleColor,
			utils.MustHex("#1A1F2E"),
			textColor,
		)
	}

	recentEntries := s.recentFiles.Entries()
	recentUI := make([]ui.RecentFile, len(recentEntries))
	for i, e := range recentEntries {
		recentUI[i] = ui.RecentFile{
			Path:        e.Path,
			DisplayName: e.DisplayName,
			Module:      e.Module,
		}
	}
	filesData := &ui.FilesDropdownData{
		CurrentModule: s.active,
		RecentFiles:   recentUI,
		OnFileClick: func(path, module string) {
			switch module {
			case "convert":
				go s.loadVideo(path)
			case "author":
				go func() {
					s.loadVideoTSChapters(path)
					fyne.CurrentApp().Driver().DoFromGoroutine(s.showAuthorView, false)
				}()
			case "inspect":
				s.showInspectViewForPath(path)
			case "player":
				s.showPlayerViewForPath(path)
			default:
				go s.loadVideo(path)
			}
		},
		OnOpenFolder: func() {
			var outputDir string
			switch s.active {
			case "audio":
				outputDir = s.audioOutputDir
			default:
				outputDir = s.convert.OutputDir
			}
			if outputDir == "" {
				outputDir = s.defaultOutputDir
			}
			if outputDir != "" {
				exec.Command("explorer", outputDir).Start()
			}
		},
		OnOpenMore: func() {
			switch s.active {
			case "audio":
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
					if err != nil || r == nil {
						return
					}
					path := r.URI().Path()
					r.Close()
					go s.loadAudioFile(path)
				}, s.window)
			default:
				dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
					if err != nil || r == nil {
						return
					}
					path := r.URI().Path()
					r.Close()
					go s.loadVideo(path)
				}, s.window)
			}
		},
	}

	menu := ui.BuildMainMenu(t.AppTitle, menuLabels, mods, s.showModule, s.handleModuleDrop, s.showQueue, nil, func() {
		// Toggle sidebar - use throttled refresh to prevent lag
		s.sidebarVisible = !s.sidebarVisible
		s.refreshMainMenuThrottled()
	}, filesData, s.sidebarVisible, sidebar, titleColor, queueColor, textColor, queueCompleted, queueTotal,
		s.pipelineStep, s.togglePipeline)

	// Update stats bar
	s.updateStatsBar()

	// Footer with version info and a small About/Support button
	versionLabel := widget.NewLabel(fmt.Sprintf("VideoTools %s", versionWithPlatform()))
	versionLabel.Alignment = fyne.TextAlignLeading
	aboutBtn := ui.MakePillButton(t.MenuAbout, ui.BorderDim, func() {
		s.showAbout()
	})
	footer := container.NewBorder(nil, nil, nil, aboutBtn, versionLabel)

	// Add stats bar at the bottom of the menu
	content := container.NewBorder(
		nil,                                   // top
		container.NewVBox(s.statsBar, footer), // bottom
		nil,                                   // left
		nil,                                   // right
		container.NewPadded(menu),             // center
	)

	s.setContent(content)
}

// refreshMainMenuThrottled rebuilds main menu but throttles to prevent excessive redraws
// Windows GUI is sensitive to rapid rebuilds, so we enforce a minimum delay
func (s *appState) refreshMainMenuThrottled() {
	now := time.Now()
	if !s.mainMenuLastRefresh.IsZero() && now.Sub(s.mainMenuLastRefresh) < 300*time.Millisecond {
		// Too soon since last refresh - skip to prevent lag
		return
	}
	s.mainMenuLastRefresh = now
	s.showMainMenu()
}

// refreshMainMenuSidebar is a lightweight refresh for sidebar-only updates
// This prevents full main menu rebuilds when only history changes
func (s *appState) refreshMainMenuSidebar() {
	// For now, use throttled refresh to prevent cascading rebuilds
	// In the future, could optimize to only update sidebar component
	s.refreshMainMenuThrottled()
}

// togglePipeline cycles the pipeline state machine:
//   "" → "step1" (activate, waiting for step1 job)
//   "step1" → ""  (cancel before step1 is queued)
//   "step2" → ""  (cancel after step1 is queued — step1 job remains standalone)
func (s *appState) togglePipeline() {
	switch s.pipelineStep {
	case "":
		s.pipelineStep = "step1"
	default:
		// Cancel pipeline — step1 job (if queued) stays in queue as a normal job
		s.pipelineStep = ""
		s.pipelineStep1ID = ""
		s.pipelineStep1OutFile = ""
	}
	s.refreshMainMenuThrottled()
}

// pipelineAdd adds a job to the queue, respecting the active pipeline state.
// In step1 mode: records the job as step1 and navigates back to the main menu.
// In step2 mode: blocks the job on step1 completion and navigates to the queue.
// Otherwise: adds the job normally.
func (s *appState) pipelineAdd(job *queue.Job) {
	switch s.pipelineStep {
	case "step1":
		s.jobQueue.Add(job)
		s.pipelineStep1ID = job.ID
		s.pipelineStep1OutFile = job.OutputFile
		s.pipelineStep = "step2"
		if !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(s.showMainMenu, false)
	case "step2":
		job.PipelineAfter = s.pipelineStep1ID
		if !s.prefs.PipelineKeepIntermediate && s.pipelineStep1OutFile != "" {
			job.PipelineDeleteOnSuccess = s.pipelineStep1OutFile
		}
		s.jobQueue.Add(job)
		s.pipelineStep = ""
		s.pipelineStep1ID = ""
		s.pipelineStep1OutFile = ""
		if !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(s.showQueue, false)
	default:
		s.jobQueue.Add(job)
	}
}
