package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	mainmenumodule "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/mainmenu"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
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
	case "enhancement":
		return t.ModuleEnhancement
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
	})

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
			titleColor,
			utils.MustHex("#1A1F2E"),
			textColor,
		)
	}

	menu := ui.BuildMainMenu(t.AppTitle, menuLabels, mods, s.showModule, s.handleModuleDrop, s.showQueue, nil, func() {
		// Toggle sidebar - use throttled refresh to prevent lag
		s.sidebarVisible = !s.sidebarVisible
		s.refreshMainMenuThrottled()
	}, nil, s.sidebarVisible, sidebar, titleColor, queueColor, textColor, queueCompleted, queueTotal)

	// Update stats bar
	s.updateStatsBar()

	// Footer with version info and a small About/Support button
	versionLabel := widget.NewLabel(fmt.Sprintf("VideoTools %s", versionWithPlatform()))
	versionLabel.Alignment = fyne.TextAlignLeading
	aboutBtn := widget.NewButton(t.MenuAbout, func() {
		s.showAbout()
	})
	aboutBtn.Importance = widget.LowImportance
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
