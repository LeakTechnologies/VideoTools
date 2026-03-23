package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/udf"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/vob"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type authorConfig = modulecfg.AuthorConfig

func defaultAuthorConfig() authorConfig {
	return modulecfg.DefaultAuthorConfig()
}

func loadPersistedAuthorConfig() (authorConfig, error) {
	return modulecfg.LoadAuthorConfig()
}

func savePersistedAuthorConfig(cfg authorConfig) error {
	return modulecfg.SaveAuthorConfig(cfg)
}

func (s *appState) applyAuthorConfig(cfg authorConfig) {
	s.authorOutputType = cfg.OutputType
	s.authorRegion = cfg.Region
	s.authorAspectRatio = cfg.AspectRatio
	s.authorDiscSize = cfg.DiscSize
	s.authorTitle = cfg.Title
	s.authorCreateMenu = cfg.CreateMenu
	s.authorMenuTemplate = cfg.MenuTemplate
	s.authorMenuTheme = cfg.MenuTheme
	s.authorMenuBackgroundImage = cfg.MenuBackgroundImage
	s.authorMenuMotionBackground = cfg.MenuMotionBackground
	s.authorMenuCustomBgColor = cfg.MenuCustomBgColor
	s.authorMenuCustomTextColor = cfg.MenuCustomTextColor
	s.authorMenuCustomAccentColor = cfg.MenuCustomAccentColor
	s.authorMenuTitleLogoEnabled = cfg.MenuTitleLogoEnabled
	s.authorMenuTitleLogoPath = cfg.MenuTitleLogoPath
	s.authorMenuTitleLogoPosition = cfg.MenuTitleLogoPosition
	s.authorMenuTitleLogoScale = cfg.MenuTitleLogoScale
	s.authorMenuTitleLogoMargin = cfg.MenuTitleLogoMargin
	s.authorMenuStudioLogoEnabled = cfg.MenuStudioLogoEnabled
	s.authorMenuStudioLogoPath = cfg.MenuStudioLogoPath
	s.authorMenuStudioLogoPosition = cfg.MenuStudioLogoPosition
	s.authorMenuStudioLogoScale = cfg.MenuStudioLogoScale
	s.authorMenuStudioLogoMargin = cfg.MenuStudioLogoMargin
	s.authorMenuStructure = cfg.MenuStructure
	s.authorMenuExtrasEnabled = cfg.MenuExtrasEnabled
	s.authorMenuChapterThumbnailSrc = cfg.MenuChapterThumbSrc
	s.authorTreatAsChapters = cfg.TreatAsChapters
	s.authorSceneThreshold = cfg.SceneThreshold
}

func (s *appState) persistAuthorConfig() {
	cfg := authorConfig{
		OutputType:             s.authorOutputType,
		Region:                 s.authorRegion,
		AspectRatio:            s.authorAspectRatio,
		DiscSize:               s.authorDiscSize,
		Title:                  s.authorTitle,
		CreateMenu:             s.authorCreateMenu,
		MenuTemplate:           s.authorMenuTemplate,
		MenuTheme:              s.authorMenuTheme,
		MenuBackgroundImage:    s.authorMenuBackgroundImage,
		MenuMotionBackground:   s.authorMenuMotionBackground,
		MenuCustomBgColor:      s.authorMenuCustomBgColor,
		MenuCustomTextColor:    s.authorMenuCustomTextColor,
		MenuCustomAccentColor:  s.authorMenuCustomAccentColor,
		MenuTitleLogoEnabled:   s.authorMenuTitleLogoEnabled,
		MenuTitleLogoPath:      s.authorMenuTitleLogoPath,
		MenuTitleLogoPosition:  s.authorMenuTitleLogoPosition,
		MenuTitleLogoScale:     s.authorMenuTitleLogoScale,
		MenuTitleLogoMargin:    s.authorMenuTitleLogoMargin,
		MenuStudioLogoEnabled:  s.authorMenuStudioLogoEnabled,
		MenuStudioLogoPath:     s.authorMenuStudioLogoPath,
		MenuStudioLogoPosition: s.authorMenuStudioLogoPosition,
		MenuStudioLogoScale:    s.authorMenuStudioLogoScale,
		MenuStudioLogoMargin:   s.authorMenuStudioLogoMargin,
		MenuStructure:          s.authorMenuStructure,
		MenuExtrasEnabled:      s.authorMenuExtrasEnabled,
		MenuChapterThumbSrc:    s.authorMenuChapterThumbnailSrc,
		TreatAsChapters:        s.authorTreatAsChapters,
		SceneThreshold:         s.authorSceneThreshold,
	}
	if err := savePersistedAuthorConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist author config: %v", err)
	}
}

func buildAuthorView(state *appState) fyne.CanvasObject {
	state.stopPreview()
	state.lastModule = state.active
	state.active = "author"

	if cfg, err := loadPersistedAuthorConfig(); err == nil {
		state.applyAuthorConfig(cfg)
	}

	if state.authorOutputType == "" {
		state.authorOutputType = "dvd"
	}
	if state.authorRegion == "" {
		state.authorRegion = "AUTO"
	}
	if state.authorAspectRatio == "" {
		state.authorAspectRatio = "AUTO"
	}
	if state.authorDiscSize == "" {
		state.authorDiscSize = "DVD5"
	}
	if state.authorMenuTemplate == "" {
		state.authorMenuTemplate = "Minimal"
	}
	if state.authorMenuTheme == "" {
		state.authorMenuTheme = "VideoTools"
	}
	if state.authorMenuTitleLogoPosition == "" {
		state.authorMenuTitleLogoPosition = "Center"
	}
	if state.authorMenuTitleLogoScale == 0 {
		state.authorMenuTitleLogoScale = 1.0
	}
	if state.authorMenuTitleLogoMargin == 0 {
		state.authorMenuTitleLogoMargin = 24
	}
	if state.authorMenuStudioLogoPosition == "" {
		state.authorMenuStudioLogoPosition = "Top Right"
	}
	if state.authorMenuStudioLogoScale == 0 {
		state.authorMenuStudioLogoScale = 1.0
	}
	if state.authorMenuStudioLogoMargin == 0 {
		state.authorMenuStudioLogoMargin = 24
	}
	if state.authorMenuStructure == "" {
		state.authorMenuStructure = "Feature + Chapters"
	}
	if state.authorMenuChapterThumbnailSrc == "" {
		state.authorMenuChapterThumbnailSrc = "Auto"
	}

	authorColor := moduleColor("author")
	t := i18n.T()

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleAuthor), func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	cancelBtn := widget.NewButton(t.ActionCancel, func() {
		if state.jobQueue != nil {
			if job := state.jobQueue.CurrentRunning(); job != nil && job.Type == queue.JobTypeAuthor {
				state.jobQueue.Cancel(job.ID)
			}
		}
	})
	cancelBtn.Importance = widget.DangerImportance
	state.authorCancelBtn = cancelBtn
	state.updateAuthorCancelButton()

	topBar := ui.TintedBar(authorColor, container.NewHBox(backBtn, layout.NewSpacer(), cancelBtn, clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(authorColor, layout.NewSpacer(), state.statsBar)

	tabs := container.NewAppTabs(
		container.NewTabItem(t.AuthorVideos, buildVideoClipsTab(state)),
		container.NewTabItem(t.AuthorChapters, buildChaptersTab(state)),
		container.NewTabItem(t.ModuleSubtitles, buildSubtitlesTab(state)),
		container.NewTabItem(t.AuthorMenuTab, buildAuthorMenuTab(state)),
		container.NewTabItem(t.ModuleSettings, buildAuthorSettingsTab(state)),
		container.NewTabItem(t.AuthorGenerateTab, buildAuthorDiscTab(state)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(topBar, bottomBar, nil, nil, tabs)
}

func buildVideoClipsTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	state.authorVideoTSPath = strings.TrimSpace(state.authorVideoTSPath)
	list := container.NewVBox()
	listScroll := ui.NewFastVScroll(list)

	var rebuildList func()
	var emptyOverlay *fyne.Container
	rebuildList = func() {
		list.Objects = nil

		// Show VIDEO_TS folder if loaded
		if state.authorVideoTSPath != "" {
			if emptyOverlay != nil {
				emptyOverlay.Hide()
			}

			videoTSLabel := widget.NewLabel(t.AuthorVideoTSSource)
			videoTSLabel.TextStyle = fyne.TextStyle{Bold: true}
			pathLabel := widget.NewLabel(state.authorVideoTSPath)
			pathLabel.Wrapping = fyne.TextWrapBreak

			removeBtn := widget.NewButton(t.ActionRemove, func() {
				state.authorVideoTSPath = ""
				state.authorOutputType = "dvd"
				rebuildList()
				state.updateAuthorSummary()
			})
			removeBtn.Importance = widget.MediumImportance

			infoLabel := widget.NewLabel(t.AuthorBurnInfo)
			infoLabel.TextStyle = fyne.TextStyle{Italic: true}
			infoLabel.Wrapping = fyne.TextWrapWord

			row := container.NewBorder(
				nil,
				nil,
				nil,
				removeBtn,
				container.NewVBox(videoTSLabel, pathLabel, infoLabel),
			)
			cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
			cardBg.CornerRadius = 6
			cardBg.SetMinSize(fyne.NewSize(0, videoTSLabel.MinSize().Height+pathLabel.MinSize().Height+infoLabel.MinSize().Height+20))
			list.Add(container.NewPadded(container.NewMax(cardBg, row)))
			list.Refresh()
			return
		}

		if len(state.authorClips) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			list.Refresh()
			return
		}

		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, clip := range state.authorClips {
			idx := i
			nameLabel := widget.NewLabel(clip.DisplayName)
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			durationLabel := widget.NewLabel(fmt.Sprintf("%.2fs", clip.Duration))
			durationLabel.TextStyle = fyne.TextStyle{Italic: true}
			durationLabel.Alignment = fyne.TextAlignTrailing

			titleEntry := widget.NewEntry()
			titleEntry.SetPlaceHolder(fmt.Sprintf("Chapter %d", idx+1))
			titleEntry.SetText(clip.ChapterTitle)
			titleEntry.OnChanged = func(val string) {
				state.authorClips[idx].ChapterTitle = val
				if state.authorTreatAsChapters {
					state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
					state.authorChapterSource = "clips"
					state.updateAuthorSummary()
				}
			}

			extraCheck := widget.NewCheck(t.AuthorMarkAsExtra, func(checked bool) {
				state.authorClips[idx].IsExtra = checked
				// Refresh chapters to exclude/include this clip
				if state.authorTreatAsChapters {
					state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
					state.authorChapterSource = "clips"
					if state.authorChaptersRefresh != nil {
						state.authorChaptersRefresh()
					}
				}
				state.updateAuthorSummary()
				state.persistAuthorConfig()
			})
			extraCheck.SetChecked(clip.IsExtra)

			moveUpBtn := widget.NewButton("▲", func() {
				if idx == 0 {
					return
				}
				state.authorClips[idx], state.authorClips[idx-1] = state.authorClips[idx-1], state.authorClips[idx]
				if state.authorChapterSource == "clips" {
					state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
					if state.authorChaptersRefresh != nil {
						state.authorChaptersRefresh()
					}
				}
				rebuildList()
				state.updateAuthorSummary()
			})
			moveUpBtn.Importance = widget.LowImportance

			moveDownBtn := widget.NewButton("▼", func() {
				if idx >= len(state.authorClips)-1 {
					return
				}
				state.authorClips[idx], state.authorClips[idx+1] = state.authorClips[idx+1], state.authorClips[idx]
				if state.authorChapterSource == "clips" {
					state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
					if state.authorChaptersRefresh != nil {
						state.authorChaptersRefresh()
					}
				}
				rebuildList()
				state.updateAuthorSummary()
			})
			moveDownBtn.Importance = widget.LowImportance

			removeBtn := widget.NewButton(t.ActionRemove, func() {
				state.authorClips = append(state.authorClips[:idx], state.authorClips[idx+1:]...)
				if state.authorChapterSource == "clips" {
					if len(state.authorClips) == 0 {
						state.authorChapters = nil
						state.authorChapterSource = ""
					} else {
						state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
					}
					if state.authorChaptersRefresh != nil {
						state.authorChaptersRefresh()
					}
				}
				rebuildList()
				state.updateAuthorSummary()
			})
			removeBtn.Importance = widget.MediumImportance

			tracksBtn := widget.NewButton(t.AuthorTracks, func() {
				state.showTrackSelectionDialog(idx, rebuildList)
			})
			tracksBtn.Importance = widget.LowImportance

			row := container.NewBorder(
				nil,
				nil,
				nil,
				container.NewVBox(durationLabel, container.NewHBox(moveUpBtn, moveDownBtn), tracksBtn, removeBtn),
				container.NewVBox(nameLabel, titleEntry, extraCheck),
			)
			cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
			cardBg.CornerRadius = 6
			cardBg.SetMinSize(fyne.NewSize(0, nameLabel.MinSize().Height+durationLabel.MinSize().Height+12))
			list.Add(container.NewPadded(container.NewMax(cardBg, row)))
		}
		list.Refresh()
	}

	addBtn := widget.NewButton(t.ActionAdd, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.addAuthorFiles([]string{reader.URI().Path()})
			rebuildList()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton(t.ActionClearAll, func() {
		state.authorClips = []authorClip{}
		state.authorChapters = nil
		state.authorChapterSource = ""
		state.authorVideoTSPath = ""
		state.authorTitle = ""
		state.authorFile = nil
		rebuildList()
		state.updateAuthorSummary()
		if state.authorChaptersRefresh != nil {
			state.authorChaptersRefresh()
		}
	})
	clearBtn.Importance = widget.MediumImportance

	addQueueBtn := widget.NewButton(t.ActionAddToQueue, func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation(t.AuthorNoClips, t.AuthorAddClipsFirst, state.window)
			return
		}
		state.startAuthorGeneration(false)
	})
	addQueueBtn.Importance = widget.MediumImportance

	compileBtn := widget.NewButton(t.AuthorCompileDVD, func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation(t.AuthorNoClips, t.AuthorAddClipsFirst, state.window)
			return
		}
		state.startAuthorGeneration(true)
	})
	compileBtn.Importance = widget.HighImportance

	chapterToggle := widget.NewCheck(t.AuthorTreatAsChapters, func(checked bool) {
		state.authorTreatAsChapters = checked
		if checked {
			state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
			state.authorChapterSource = "clips"
		} else if state.authorChapterSource == "clips" {
			state.authorChapterSource = ""
			state.authorChapters = nil
		}
		state.updateAuthorSummary()
		state.persistAuthorConfig()
		if state.authorChaptersRefresh != nil {
			state.authorChaptersRefresh()
		}
	})
	chapterToggle.SetChecked(state.authorTreatAsChapters)

	dropTarget := ui.NewDroppable(listScroll, func(items []fyne.URI) {
		var paths []string
		for _, uri := range items {
			if uri.Scheme() == "file" {
				paths = append(paths, uri.Path())
			}
		}
		if len(paths) > 0 {
			state.addAuthorFiles(paths)
			rebuildList()
		}
	})

	emptyLabel := widget.NewLabel(t.AuthorDragDropHint)
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(dropTarget, emptyOverlay)

	// DVD Title entry (synced with Settings tab)
	dvdTitleEntry := widget.NewEntry()
	dvdTitleEntry.SetPlaceHolder(t.AuthorDVDTitle)
	dvdTitleEntry.SetText(state.authorTitle)
	dvdTitleEntry.OnChanged = func(value string) {
		state.authorTitle = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	}

	controls := container.NewBorder(
		container.NewVBox(
			widget.NewLabel(t.AuthorDVDTitle),
			dvdTitleEntry,
			widget.NewSeparator(),
			widget.NewLabel(t.AuthorVideosCount),
		),
		container.NewVBox(chapterToggle, container.NewHBox(addBtn, clearBtn, addQueueBtn, compileBtn)),
		nil,
		nil,
		listArea,
	)

	rebuildList()
	return container.NewPadded(controls)
}

func buildChaptersTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	var fileLabel *widget.Label
	if state.authorFile != nil {
		fileLabel = widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(state.authorFile.Path)))
		fileLabel.TextStyle = fyne.TextStyle{Bold: true}
	} else {
		fileLabel = widget.NewLabel(t.AuthorSelectSingle)
	}

	chapterList := container.NewVBox()
	sourceLabel := widget.NewLabel("")
	refreshChapters := func() {
		chapterList.Objects = nil
		sourceLabel.SetText("")
		if len(state.authorChapters) == 0 {
			if state.authorTreatAsChapters && len(state.authorClips) > 1 {
				state.authorChapters = chaptersFromClips(featureClipsOnly(state.authorClips))
				state.authorChapterSource = "clips"
			}
		}
		if len(state.authorChapters) == 0 {
			chapterList.Add(widget.NewLabel(t.AuthorNoChapters))
			return
		}
		switch state.authorChapterSource {
		case "clips":
			sourceLabel.SetText("Source: Video clips (treat as chapters)")
		case "embedded":
			sourceLabel.SetText("Source: Embedded chapters")
		case "scenes":
			sourceLabel.SetText("Source: Scene detection")
		case "videots":
			sourceLabel.SetText("Source: VIDEO_TS chapters")
		default:
			sourceLabel.SetText("Source: Chapters")
		}
		for i, ch := range state.authorChapters {
			title := ch.Title
			if title == "" {
				title = fmt.Sprintf("Chapter %d", i+1)
			}
			chapterList.Add(widget.NewLabel(fmt.Sprintf("%02d. %s (%s)", i+1, title, formatChapterTime(ch.Timestamp))))
		}
	}
	state.authorChaptersRefresh = refreshChapters

	selectBtn := widget.NewButton(t.AuthorSelectVideo, func() {
		dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			path := uc.URI().Path()
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}
			state.authorFile = src
			fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(src.Path)))
			// Clear the custom title so it can be re-derived from the new content.
			// This addresses the user's request for the title to "reset".
			state.authorTitle = ""
			state.updateAuthorSummary()
			// Update the UI for the title entry if the settings tab is currently visible.
			if state.active == "author" && state.window.Canvas() != nil {
				app := fyne.CurrentApp()
				if app != nil && app.Driver() != nil {
					app.Driver().DoFromGoroutine(func() {
						state.showAuthorView() // Rebuild the module to refresh titleEntry
					}, false)
				}
			}
			state.loadEmbeddedChapters(path)
			refreshChapters()
		}, state.window)
	})

	thresholdLabel := widget.NewLabel(fmt.Sprintf("Detection Sensitivity: %.2f", state.authorSceneThreshold))
	thresholdSlider := widget.NewSlider(0.1, 0.9)
	thresholdSlider.Value = state.authorSceneThreshold
	thresholdSlider.Step = 0.05
	thresholdSlider.OnChanged = func(v float64) {
		state.authorSceneThreshold = v
		thresholdLabel.SetText(fmt.Sprintf("Detection Sensitivity: %.2f", v))
		state.persistAuthorConfig()
	}

	detectBtn := widget.NewButton(t.AuthorDetectScenes, func() {
		targetPath := ""
		if state.authorFile != nil {
			targetPath = state.authorFile.Path
		} else if len(state.authorClips) > 0 {
			targetPath = state.authorClips[0].Path
		}
		if targetPath == "" {
			dialog.ShowInformation(t.AuthorNoFile, t.AuthorSelectVideoFirst, state.window)
			return
		}

		progress := dialog.NewProgressInfinite(t.AuthorSceneDetection, "Analyzing scene changes with FFmpeg...", state.window)
		progress.Show()
		state.authorDetecting = true

		go func() {
			chapters, err := detectSceneChapters(targetPath, state.authorSceneThreshold)
			runOnUI(func() {
				progress.Hide()
				state.authorDetecting = false
				if err != nil {
					dialog.ShowError(err, state.window)
					return
				}
				if len(chapters) == 0 {
					dialog.ShowInformation(t.AuthorSceneDetection, t.AuthorNoScenesDetected, state.window)
					return
				}
				// Show chapter preview dialog for visual verification and manual tweaking
				state.showChapterPreview(targetPath, chapters, func(accepted bool, result []authorChapter) {
					if accepted {
						state.authorChapters = result
						state.authorChapterSource = "scenes"
						state.updateAuthorSummary()
						refreshChapters()
					}
				})
			})
		}()
	})
	detectBtn.Importance = widget.HighImportance

	addChapterBtn := widget.NewButton(t.AuthorAddChapter, func() {
		dialog.ShowInformation(t.AuthorAddChapter, t.AuthorManualChapterSoon, state.window)
	})

	exportBtn := widget.NewButton(t.AuthorExportChapters, func() {
		dialog.ShowInformation(t.AuthorExport, t.AuthorChapterExportSoon, state.window)
	})

	controlsTop := container.NewVBox(
		fileLabel,
		selectBtn,
		widget.NewSeparator(),
		widget.NewLabel(t.AuthorSceneDetection+":"),
		thresholdLabel,
		thresholdSlider,
		detectBtn,
		widget.NewSeparator(),
		widget.NewLabel(t.AuthorChapters+":"),
		sourceLabel,
	)

	listScroll := ui.NewFastVScroll(chapterList)
	bottomRow := container.NewHBox(addChapterBtn, exportBtn)

	controls := container.NewBorder(
		controlsTop,
		bottomRow,
		nil,
		nil,
		listScroll,
	)

	refreshChapters()
	return container.NewPadded(controls)
}

func buildSubtitlesTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	list := container.NewVBox()
	listScroll := ui.NewFastVScroll(list)

	var buildSubList func()
	var emptyOverlay *fyne.Container
	buildSubList = func() {
		list.Objects = nil

		if len(state.authorSubtitles) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			list.Refresh()
			return
		}

		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, path := range state.authorSubtitles {
			idx := i
			card := widget.NewCard(filepath.Base(path), "", nil)

			removeBtn := widget.NewButton(t.ActionRemove, func() {
				state.authorSubtitles = append(state.authorSubtitles[:idx], state.authorSubtitles[idx+1:]...)
				buildSubList()
				state.updateAuthorSummary()
			})
			removeBtn.Importance = widget.MediumImportance

			cardContent := container.NewVBox(removeBtn)
			card.SetContent(cardContent)
			list.Add(card)
		}
		list.Refresh()
	}

	addBtn := widget.NewButton(t.AuthorAddSubtitles, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorSubtitles = append(state.authorSubtitles, reader.URI().Path())
			buildSubList()
			state.updateAuthorSummary()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance

	openSubtitlesBtn := widget.NewButton(t.AuthorOpenSubtitlesTool, func() {
		if state.authorFile != nil {
			state.subtitleVideoPath = state.authorFile.Path
		} else if len(state.authorClips) > 0 {
			state.subtitleVideoPath = state.authorClips[0].Path
		}
		if len(state.authorSubtitles) > 0 {
			state.subtitleFilePath = state.authorSubtitles[0]
		}
		state.showSubtitlesView()
	})
	openSubtitlesBtn.Importance = widget.MediumImportance

	clearBtn := widget.NewButton(t.ActionClearAll, func() {
		state.authorSubtitles = []string{}
		buildSubList()
		state.updateAuthorSummary()
	})
	clearBtn.Importance = widget.MediumImportance

	dropTarget := ui.NewDroppable(listScroll, func(items []fyne.URI) {
		var paths []string
		for _, uri := range items {
			if uri.Scheme() == "file" {
				paths = append(paths, uri.Path())
			}
		}
		if len(paths) > 0 {
			state.authorSubtitles = append(state.authorSubtitles, paths...)
			buildSubList()
			state.updateAuthorSummary()
		}
	})

	emptyLabel := widget.NewLabel(t.AuthorDragDropSubtitles)
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(dropTarget, emptyOverlay)

	controls := container.NewBorder(
		widget.NewLabel(t.AuthorSubtitleTracks),
		container.NewHBox(addBtn, openSubtitlesBtn, clearBtn),
		nil,
		nil,
		listArea,
	)

	buildSubList()
	return container.NewPadded(controls)
}

func buildAuthorSettingsTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	regionSelect := widget.NewSelect([]string{"AUTO", "NTSC", "PAL"}, func(value string) {
		state.authorRegion = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	discSizeSelect := widget.NewSelect([]string{"DVD5", "DVD9"}, func(value string) {
		state.authorDiscSize = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	updateDynamicSettings := func(target string) {
		if target == "bluray" {
			regionSelect.Options = []string{"AUTO", "1080p", "4K UHD"}
			discSizeSelect.Options = []string{"BD25", "BD50", "BD66", "BD100"}
		} else {
			regionSelect.Options = []string{"AUTO", "NTSC", "PAL"}
			discSizeSelect.Options = []string{"DVD5", "DVD9"}
		}
		regionSelect.Refresh()
		discSizeSelect.Refresh()
	}

	outputType := widget.NewSelect([]string{"ISO Image", "DVD (VIDEO_TS)"}, func(value string) {
		if value == "ISO Image" {
			state.authorOutputType = "iso"
		} else {
			state.authorOutputType = "dvd"
		}
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	aspectSelect := widget.NewSelect([]string{"AUTO", "4:3", "16:9"}, func(value string) {
		state.authorAspectRatio = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Disc title...")
	titleEntry.OnChanged = func(value string) {
		state.authorTitle = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	}

	targetType := widget.NewSelect([]string{"DVD-Video", "Blu-ray Disc"}, func(value string) {
		if value == "DVD-Video" {
			state.authorOutputType = "dvd"
			updateDynamicSettings("dvd")
		} else {
			state.authorOutputType = "bluray"
			updateDynamicSettings("bluray")
		}
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorOutputType == "bluray" {
		targetType.SetSelected("Blu-ray Disc")
		updateDynamicSettings("bluray")
	} else {
		targetType.SetSelected("DVD-Video")
		updateDynamicSettings("dvd")
	}

	applyControls := func() {
		if state.authorOutputType == "iso" {
			outputType.SetSelected("ISO Image")
		} else {
			outputType.SetSelected("DVD (VIDEO_TS)")
		}
		if state.authorRegion == "" {
			regionSelect.SetSelected("AUTO")
		} else {
			regionSelect.SetSelected(state.authorRegion)
		}
		if state.authorAspectRatio == "" {
			aspectSelect.SetSelected("AUTO")
		} else {
			aspectSelect.SetSelected(state.authorAspectRatio)
		}
		if state.authorDiscSize == "" {
			discSizeSelect.SetSelected("DVD5")
		} else {
			discSizeSelect.SetSelected(state.authorDiscSize)
		}
		titleEntry.SetText(state.authorTitle)
	}

	loadCfgBtn := widget.NewButton(t.AuthorLoadConfig, func() {
		cfg, err := loadPersistedAuthorConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.applyAuthorConfig(cfg)
		applyControls()
		state.updateAuthorSummary()
	})

	saveCfgBtn := widget.NewButton(t.AuthorSaveConfig, func() {
		cfg := authorConfig{
			OutputType:             state.authorOutputType,
			Region:                 state.authorRegion,
			AspectRatio:            state.authorAspectRatio,
			DiscSize:               state.authorDiscSize,
			Title:                  state.authorTitle,
			CreateMenu:             state.authorCreateMenu,
			MenuTemplate:           state.authorMenuTemplate,
			MenuTheme:              state.authorMenuTheme,
			MenuBackgroundImage:    state.authorMenuBackgroundImage,
			MenuTitleLogoEnabled:   state.authorMenuTitleLogoEnabled,
			MenuTitleLogoPath:      state.authorMenuTitleLogoPath,
			MenuTitleLogoPosition:  state.authorMenuTitleLogoPosition,
			MenuTitleLogoScale:     state.authorMenuTitleLogoScale,
			MenuTitleLogoMargin:    state.authorMenuTitleLogoMargin,
			MenuStudioLogoEnabled:  state.authorMenuStudioLogoEnabled,
			MenuStudioLogoPath:     state.authorMenuStudioLogoPath,
			MenuStudioLogoPosition: state.authorMenuStudioLogoPosition,
			MenuStudioLogoScale:    state.authorMenuStudioLogoScale,
			MenuStudioLogoMargin:   state.authorMenuStudioLogoMargin,
			MenuStructure:          state.authorMenuStructure,
			MenuExtrasEnabled:      state.authorMenuExtrasEnabled,
			MenuChapterThumbSrc:    state.authorMenuChapterThumbnailSrc,
			TreatAsChapters:        state.authorTreatAsChapters,
			SceneThreshold:         state.authorSceneThreshold,
		}
		if err := savePersistedAuthorConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", configpath.ModuleConfigPath("author")), state.window)
	})

	resetBtn := widget.NewButton(t.AuthorReset, func() {
		cfg := defaultAuthorConfig()
		state.applyAuthorConfig(cfg)
		applyControls()
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	info := widget.NewLabel(t.AuthorRequiresFFmpeg + " " + t.AuthorRequiresSpumux)
	info.Wrapping = fyne.TextWrapWord

	controls := container.NewVBox(
		widget.NewLabelWithStyle("Target Disc Type:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetType,
		widget.NewSeparator(),
		widget.NewLabel("Output Format:"),
		outputType,
		widget.NewLabel("Region:"),
		regionSelect,
		widget.NewLabel("Aspect Ratio:"),
		aspectSelect,
		widget.NewLabel("Disc Size:"),
		discSizeSelect,
		widget.NewLabel(t.AuthorDVDTitle),
		titleEntry,
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
	)

	return ui.NewFastVScroll(container.NewPadded(controls))
}

func buildAuthorMenuTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	navyBlue := utils.MustHex("#191F35")
	boxAccent := gridColor
	var updateMenuControls func(bool)
	var schedulePreviewRefresh func()

	buildMenuBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = boxAccent
		bg.StrokeWidth = 1
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		return container.NewMax(bg, container.NewPadded(body))
	}

	sectionGap := func() fyne.CanvasObject {
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 10))
		return gap
	}

	createMenuCheck := widget.NewCheck(t.AuthorEnableMenus, func(checked bool) {
		state.authorCreateMenu = checked
		state.updateAuthorSummary()
		state.persistAuthorConfig()
		if updateMenuControls != nil {
			updateMenuControls(checked)
		}
	})
	createMenuCheck.SetChecked(state.authorCreateMenu)
	menuDisabledNote := widget.NewLabel(t.AuthorMenusDisabledNote)
	menuDisabledNote.TextStyle = fyne.TextStyle{Italic: true}
	menuDisabledNote.Wrapping = fyne.TextWrapWord

	// Theme dropdown - Visual aesthetic (colors/feel)
	themeOptions := []string{
		"VideoTools",
		"Minimal",
		"Western",
		"Film Noir",
		"Classic Hollywood",
		"Warm Cinema",
		"Ocean",
		"Nature",
		"Custom",
	}
	menuThemeSelect := widget.NewSelect(themeOptions, func(value string) {
		state.authorMenuTheme = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorMenuTheme == "" {
		state.authorMenuTheme = "VideoTools"
	}
	menuThemeSelect.SetSelected(state.authorMenuTheme)

	var updateCustomColors func()
	menuThemeSelect.OnChanged = func(value string) {
		state.authorMenuTheme = value
		updateCustomColors()
		state.updateAuthorSummary()
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	}

	// Custom theme color pickers
	customBgColorEntry := widget.NewEntry()
	customBgColorEntry.SetPlaceHolder("#000000")
	if state.authorMenuCustomBgColor != "" {
		customBgColorEntry.SetText(state.authorMenuCustomBgColor)
	}
	customBgColorEntry.OnChanged = func(value string) {
		state.authorMenuCustomBgColor = value
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	}

	customTextColorEntry := widget.NewEntry()
	customTextColorEntry.SetPlaceHolder("#FFFFFF")
	if state.authorMenuCustomTextColor != "" {
		customTextColorEntry.SetText(state.authorMenuCustomTextColor)
	}
	customTextColorEntry.OnChanged = func(value string) {
		state.authorMenuCustomTextColor = value
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	}

	customAccentColorEntry := widget.NewEntry()
	customAccentColorEntry.SetPlaceHolder("#FFFFFF")
	if state.authorMenuCustomAccentColor != "" {
		customAccentColorEntry.SetText(state.authorMenuCustomAccentColor)
	}
	customAccentColorEntry.OnChanged = func(value string) {
		state.authorMenuCustomAccentColor = value
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	}

	// Show/hide custom color pickers based on theme selection
	updateCustomColors = func() {
		isCustom := state.authorMenuTheme == "Custom"
		customBgColorEntry.Show()
		customTextColorEntry.Show()
		customAccentColorEntry.Show()
		if !isCustom {
			customBgColorEntry.Hide()
			customTextColorEntry.Hide()
			customAccentColorEntry.Hide()
		}
	}
	updateCustomColors()

	// Template dropdown - Layout/structure
	templateOptions := []string{
		"Minimal (Clean & Simple)",
		"Simple (Title + Buttons)",
		"Classic (Centered Title + Buttons)",
		"Grid (2x2 Buttons)",
		"Filmstrip (Wide Buttons)",
		"Poster (Grid Thumbnails)",
		"Scriptable (Custom JSON Theme)",
	}
	templateValueByLabel := map[string]string{
		"Minimal (Clean & Simple)":           "Minimal",
		"Simple (Title + Buttons)":           "Simple",
		"Classic (Centered Title + Buttons)": "Classic",
		"Grid (2x2 Buttons)":                 "Grid",
		"Filmstrip (Wide Buttons)":           "Filmstrip",
		"Poster (Grid Thumbnails)":           "Poster",
		"Scriptable (Custom JSON Theme)":     "Scriptable",
	}
	templateLabelByValue := map[string]string{
		"Minimal":   "Minimal (Clean & Simple)",
		"Simple":    "Simple (Title + Buttons)",
		"Classic":   "Classic (Centered Title + Buttons)",
		"Grid":      "Grid (2x2 Buttons)",
		"Filmstrip": "Filmstrip (Wide Buttons)",
		"Poster":    "Poster (Grid Thumbnails)",
	}

	menuTemplateSelect := widget.NewSelect(templateOptions, func(value string) {
		state.authorMenuTemplate = templateValueByLabel[value]
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorMenuTemplate == "" {
		state.authorMenuTemplate = "Minimal"
	}
	templateLabel := templateLabelByValue[state.authorMenuTemplate]
	if templateLabel == "" {
		templateLabel = templateOptions[0]
		state.authorMenuTemplate = templateValueByLabel[templateLabel]
	}
	menuTemplateSelect.SetSelected(templateLabel)

	bgImageLabel := widget.NewLabel(state.authorMenuBackgroundImage)
	bgImageLabel.Wrapping = fyne.TextWrapWord
	bgImageButton := widget.NewButton(t.AuthorSelectBGImage, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorMenuBackgroundImage = reader.URI().Path()
			bgImageLabel.SetText(state.authorMenuBackgroundImage)
			state.updateAuthorSummary()
			state.persistAuthorConfig()
			if schedulePreviewRefresh != nil {
				schedulePreviewRefresh()
			}
		}, state.window)
	})
	bgImageButton.Importance = widget.HighImportance

	// Motion background (video loop) controls
	motionBgLabel := widget.NewLabel(state.authorMenuMotionBackground)
	motionBgLabel.Wrapping = fyne.TextWrapWord
	motionBgNote := widget.NewLabel(t.AuthorMotionBackground)
	motionBgNote.TextStyle = fyne.TextStyle{Italic: true}
	motionBgNote.Wrapping = fyne.TextWrapWord
	motionBgButton := widget.NewButton(t.AuthorSelectMotionBG, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorMenuMotionBackground = reader.URI().Path()
			motionBgLabel.SetText(state.authorMenuMotionBackground)
			state.updateAuthorSummary()
			state.persistAuthorConfig()
		}, state.window)
	})
	motionBgButton.Importance = widget.MediumImportance

	clearMotionBgButton := widget.NewButton(t.ActionClear, func() {
		state.authorMenuMotionBackground = ""
		motionBgLabel.SetText("")
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	clearMotionBgButton.Importance = widget.LowImportance

	menuTemplateSelect.OnChanged = func(value string) {
		state.authorMenuTemplate = templateValueByLabel[value]
		state.updateAuthorSummary()
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	}

	logoEnableCheck := widget.NewCheck(t.AuthorEmbedLogo, nil)
	logoEnableCheck.SetChecked(state.authorMenuStudioLogoEnabled)

	logoFileEntry := widget.NewEntry()
	logoFileEntry.Disable()
	logoFileEntry.SetPlaceHolder(t.AuthorNoLogoSelected)
	logoPreview := canvas.NewImageFromFile("")
	logoPreview.FillMode = canvas.ImageFillContain
	logoPreview.SetMinSize(fyne.NewSize(96, 96))
	logoPreviewLabel := widget.NewLabel(t.AuthorNoLogoSelected)
	logoPreviewLabel.Wrapping = fyne.TextWrapWord
	logoPreviewSize := widget.NewLabel("")
	logoPreviewSize.Wrapping = fyne.TextWrapWord
	logoPreviewBorder := canvas.NewRectangle(color.NRGBA{R: 80, G: 86, B: 100, A: 120})
	logoPreviewBorder.SetMinSize(fyne.NewSize(120, 96))
	logoPreviewBox := container.NewMax(
		logoPreviewBorder,
		container.NewPadded(container.NewCenter(logoPreview)),
	)
	previewMin := canvas.NewRectangle(color.NRGBA{A: 0})
	previewMin.SetMinSize(fyne.NewSize(220, 0))
	updateBrandingTitle := func() {}

	menuPreviewSize := func() (int, int) {
		width := 720
		height := 480
		switch strings.ToUpper(strings.TrimSpace(state.authorRegion)) {
		case "PAL":
			height = 576
		}
		return width, height
	}

	logoDisplayName := func() string {
		if strings.TrimSpace(state.authorMenuStudioLogoPath) == "" {
			return "VT_Logo.png (default)"
		}
		return filepath.Base(state.authorMenuStudioLogoPath)
	}

	updateLogoPreview := func() {
		logoFileEntry.SetText(logoDisplayName())
		if !state.authorMenuStudioLogoEnabled {
			logoPreviewBox.Hide()
			logoPreviewLabel.SetText("Logo disabled")
			logoPreviewSize.SetText("")
			return
		}

		path := state.authorMenuStudioLogoPath
		if strings.TrimSpace(path) == "" {
			path = filepath.Join("assets", "logo", "VT_Logo.png")
		}

		if _, err := os.Stat(path); err != nil {
			logoPreviewBox.Hide()
			logoPreviewLabel.SetText("Logo file not found")
			logoPreviewSize.SetText("")
			return
		}

		logoPreviewBox.Show()
		logoPreviewLabel.SetText(filepath.Base(path))
		logoPreview.File = path
		logoPreview.Refresh()

		file, err := os.Open(path)
		if err != nil {
			logoPreviewSize.SetText("")
			return
		}
		defer file.Close()

		cfg, _, err := image.DecodeConfig(file)
		if err != nil {
			logoPreviewSize.SetText("")
			return
		}

		menuW, menuH := menuPreviewSize()
		maxW := int(float64(menuW) * 0.25)
		maxH := int(float64(menuH) * 0.25)
		scale := state.authorMenuStudioLogoScale
		targetW := int(math.Round(float64(cfg.Width) * scale))
		targetH := int(math.Round(float64(cfg.Height) * scale))
		if targetW > maxW || targetH > maxH {
			ratioW := float64(maxW) / float64(targetW)
			ratioH := float64(maxH) / float64(targetH)
			ratio := math.Min(ratioW, ratioH)
			targetW = int(math.Round(float64(targetW) * ratio))
			targetH = int(math.Round(float64(targetH) * ratio))
		}

		logoPreviewSize.SetText(fmt.Sprintf("Logo size: %dx%d (max %dx%d)", targetW, targetH, maxW, maxH))
	}
	logoPickButton := widget.NewButton(t.AuthorSelectLogo, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorMenuStudioLogoPath = reader.URI().Path()
			logoFileEntry.SetText(logoDisplayName())
			updateLogoPreview()
			updateBrandingTitle()
			state.updateAuthorSummary()
			state.persistAuthorConfig()
		}, state.window)
	})
	logoPickButton.Importance = widget.MediumImportance
	logoClearButton := widget.NewButton(t.ActionClear, func() {
		state.authorMenuStudioLogoPath = ""
		logoFileEntry.SetText(logoDisplayName())
		logoEnableCheck.SetChecked(false)
		updateLogoPreview()
		updateBrandingTitle()
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	logoPositionSelect := widget.NewSelect([]string{
		"Top Left",
		"Top Right",
		"Bottom Left",
		"Bottom Right",
		"Center",
	}, func(value string) {
		state.authorMenuStudioLogoPosition = value
		updateBrandingTitle()
		state.persistAuthorConfig()
	})
	if state.authorMenuStudioLogoPosition == "" {
		state.authorMenuStudioLogoPosition = "Top Right"
	}
	logoPositionSelect.SetSelected(state.authorMenuStudioLogoPosition)

	scaleOptions := []string{"50%", "75%", "100%", "125%", "150%", "200%"}
	scaleValueByLabel := map[string]float64{
		"50%":  0.5,
		"75%":  0.75,
		"100%": 1.0,
		"125%": 1.25,
		"150%": 1.5,
		"200%": 2.0,
	}
	scaleLabelByValue := map[float64]string{
		0.5:  "50%",
		0.75: "75%",
		1.0:  "100%",
		1.25: "125%",
		1.5:  "150%",
		2.0:  "200%",
	}
	if state.authorMenuStudioLogoScale == 0 {
		state.authorMenuStudioLogoScale = 1.0
	}
	logoScaleSelect := widget.NewSelect(scaleOptions, func(value string) {
		if scale, ok := scaleValueByLabel[value]; ok {
			state.authorMenuStudioLogoScale = scale
			updateLogoPreview()
			updateBrandingTitle()
			state.persistAuthorConfig()
		}
	})
	scaleLabel := scaleLabelByValue[state.authorMenuStudioLogoScale]
	if scaleLabel == "" {
		scaleLabel = "100%"
		state.authorMenuStudioLogoScale = 1.0
	}
	logoScaleSelect.SetSelected(scaleLabel)

	if state.authorMenuStudioLogoMargin == 0 {
		state.authorMenuStudioLogoMargin = 24
	}
	marginEntry := widget.NewEntry()
	marginEntry.SetText(strconv.Itoa(state.authorMenuStudioLogoMargin))
	updatingMargin := false
	updateMargin := func(value int, updateEntry bool) {
		if value < 0 {
			value = 0
		}
		if value > 60 {
			value = 60
		}
		state.authorMenuStudioLogoMargin = value
		if updateEntry {
			updatingMargin = true
			marginEntry.SetText(strconv.Itoa(value))
			updatingMargin = false
		}
		state.persistAuthorConfig()
	}
	marginEntry.OnChanged = func(value string) {
		if updatingMargin {
			return
		}
		if strings.TrimSpace(value) == "" {
			return
		}
		if v, err := strconv.Atoi(value); err == nil {
			if v == state.authorMenuStudioLogoMargin {
				return
			}
			updateMargin(v, true)
		}
	}
	marginMinus := widget.NewButton("-", func() {
		updateMargin(state.authorMenuStudioLogoMargin-2, true)
	})
	marginPlus := widget.NewButton("+", func() {
		updateMargin(state.authorMenuStudioLogoMargin+2, true)
	})

	safeAreaNote := widget.NewLabel("Logos are constrained to DVD safe areas.")
	safeAreaNote.TextStyle = fyne.TextStyle{Italic: true}
	safeAreaNote.Wrapping = fyne.TextWrapWord

	menuStructureSelect := widget.NewSelect([]string{
		"Feature Only",
		"Feature + Chapters",
		"Feature + Extras",
		"Feature + Chapters + Extras",
	}, func(value string) {
		state.authorMenuStructure = value
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	})
	if state.authorMenuStructure == "" {
		state.authorMenuStructure = "Feature + Chapters"
	}
	menuStructureSelect.SetSelected(state.authorMenuStructure)

	extrasMenuCheck := widget.NewCheck(t.AuthorEnableExtrasMenu, func(checked bool) {
		state.authorMenuExtrasEnabled = checked
		state.persistAuthorConfig()
		if schedulePreviewRefresh != nil {
			schedulePreviewRefresh()
		}
	})
	extrasMenuCheck.SetChecked(state.authorMenuExtrasEnabled)
	extrasNote := widget.NewLabel(t.AuthorExtrasNote)
	extrasNote.Wrapping = fyne.TextWrapWord

	thumbSourceSelect := widget.NewSelect([]string{
		"Auto",
		"First Frame",
		"Midpoint",
		"Custom (Advanced)",
	}, func(value string) {
		state.authorMenuChapterThumbnailSrc = value
		state.persistAuthorConfig()
	})
	if state.authorMenuChapterThumbnailSrc == "" {
		state.authorMenuChapterThumbnailSrc = "Auto"
	}
	thumbSourceSelect.SetSelected(state.authorMenuChapterThumbnailSrc)

	info := widget.NewLabel("DVD menus are generated using the VideoTools theme and IBM Plex Mono. Menu settings apply only to disc authoring.")
	info.Wrapping = fyne.TextWrapWord

	logoPreviewGroup := container.NewMax(
		previewMin,
		container.NewVBox(
			widget.NewLabel("Preview:"),
			logoPreviewLabel,
			logoPreviewBox,
			logoPreviewSize,
		),
	)

	logoButtonRow := container.NewHBox(
		logoPickButton,
		logoClearButton,
	)
	logoFileRow := container.NewBorder(nil, nil, nil, logoButtonRow, logoFileEntry)

	logoPositionRow := container.NewHBox(
		widget.NewLabel("Position:"),
		layout.NewSpacer(),
		logoPositionSelect,
	)
	logoScaleRow := container.NewHBox(
		widget.NewLabel("Scale:"),
		layout.NewSpacer(),
		logoScaleSelect,
	)
	logoMarginRow := container.NewHBox(
		widget.NewLabel("Margin:"),
		marginMinus,
		marginEntry,
		marginPlus,
		widget.NewLabel("px"),
	)

	menuCore := buildMenuBox("Menu Core", container.NewVBox(
		createMenuCheck,
		menuDisabledNote,
		widget.NewLabel("Theme:"),
		menuThemeSelect,
		container.NewHBox(widget.NewLabel("Bg:"), customBgColorEntry),
		container.NewHBox(widget.NewLabel("Text:"), customTextColorEntry),
		container.NewHBox(widget.NewLabel("Accent:"), customAccentColorEntry),
		widget.NewLabel("Template:"),
		menuTemplateSelect,
		bgImageLabel,
		bgImageButton,
		layout.NewSpacer(),
		motionBgNote,
		motionBgLabel,
		container.NewHBox(motionBgButton, clearMotionBgButton),
		widget.NewLabel("Menu Structure:"),
		menuStructureSelect,
	))

	brandingLeft := container.NewVBox(
		logoEnableCheck,
		widget.NewLabel("Logo File:"),
		logoFileRow,
		logoPositionRow,
		logoScaleRow,
		logoMarginRow,
		safeAreaNote,
	)
	brandingContent := container.NewBorder(nil, nil, nil, logoPreviewGroup, brandingLeft)
	brandingItem := widget.NewAccordionItem(t.AuthorBranding, brandingContent)
	brandingItem.Open = false
	brandingAccordion := widget.NewAccordion(brandingItem)
	branding := container.NewMax(
		canvas.NewRectangle(navyBlue),
		container.NewPadded(brandingAccordion),
	)
	if bg, ok := branding.Objects[0].(*canvas.Rectangle); ok {
		bg.CornerRadius = 10
		bg.StrokeColor = boxAccent
		bg.StrokeWidth = 1
	}

	navigation := buildMenuBox(t.AuthorNavigation, container.NewVBox(
		extrasMenuCheck,
		extrasNote,
		widget.NewLabel(t.AuthorChapterThumbSource),
		thumbSourceSelect,
	))

	var previewPanelContent fyne.CanvasObject
	previewPanelContent, schedulePreviewRefresh = buildMenuPreviewPanel(state)
	previewBox := buildMenuBox("DVD Menu Preview", previewPanelContent)

	controls := container.NewVBox(
		menuCore,
		sectionGap(),
		branding,
		sectionGap(),
		navigation,
		sectionGap(),
		previewBox,
		sectionGap(),
		info,
	)

	updateMenuControls = func(enabled bool) {
		menuDisabledNote.Hidden = enabled
		setEnabled := func(on bool, items ...fyne.Disableable) {
			for _, item := range items {
				if on {
					item.Enable()
				} else {
					item.Disable()
				}
			}
		}

		setEnabled(enabled,
			menuThemeSelect,
			menuTemplateSelect,
			bgImageButton,
			menuStructureSelect,
			logoEnableCheck,
			logoPickButton,
			logoClearButton,
			logoPositionSelect,
			logoScaleSelect,
			marginEntry,
			marginMinus,
			marginPlus,
			extrasMenuCheck,
			thumbSourceSelect,
		)

		logoControlsEnabled := enabled && state.authorMenuStudioLogoEnabled
		setEnabled(logoControlsEnabled,
			logoPickButton,
			logoClearButton,
			logoPositionSelect,
			logoScaleSelect,
			marginEntry,
			marginMinus,
			marginPlus,
		)
	}

	updateBrandingTitle = func() {
		if !state.authorMenuStudioLogoEnabled {
			brandingItem.Title = t.AuthorBrandingDisabled
			brandingAccordion.Refresh()
			return
		}
		scaleText := scaleLabelByValue[state.authorMenuStudioLogoScale]
		if scaleText == "" {
			scaleText = "100%"
		}
		name := logoDisplayName()
		brandingItem.Title = fmt.Sprintf("Branding: %s (%s, %s)", name, state.authorMenuStudioLogoPosition, scaleText)
		brandingAccordion.Refresh()
	}

	logoEnableCheck.OnChanged = func(checked bool) {
		state.authorMenuStudioLogoEnabled = checked
		updateLogoPreview()
		updateBrandingTitle()
		updateMenuControls(state.authorCreateMenu)
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	}

	updateLogoPreview()
	updateBrandingTitle()
	updateMenuControls(state.authorCreateMenu)

	scroll := ui.NewFastVScroll(container.NewPadded(controls))
	scroll.SetMinSize(fyne.NewSize(0, 420))
	return scroll
}

func buildAuthorDiscTab(state *appState) fyne.CanvasObject {
	t := i18n.T()
	generateBtn := widget.NewButton(t.AuthorGenerateDVD, func() {
		if len(state.authorClips) == 0 && state.authorFile == nil {
			dialog.ShowInformation(t.AuthorNoContent, t.AuthorAddVideosForDVD, state.window)
			return
		}
		state.startAuthorGeneration(true)
	})
	generateBtn.Importance = widget.HighImportance

	// Keyboard shortcut: Ctrl+Enter -> Generate DVD
	if c := state.window.Canvas(); c != nil {
		triggerGenerate := func() {
			if !generateBtn.Disabled() && generateBtn.OnTapped != nil {
				generateBtn.OnTapped()
			}
		}
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { triggerGenerate() })
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { triggerGenerate() })
	}

	summaryLabel := widget.NewLabel(authorSummary(state))
	summaryLabel.Wrapping = fyne.TextWrapWord
	state.authorSummaryLabel = summaryLabel

	statusLabel := widget.NewLabel(t.AuthorReady)
	statusLabel.Wrapping = fyne.TextWrapWord
	state.authorStatusLabel = statusLabel

	progressBar := widget.NewProgressBar()
	progressBar.SetValue(state.authorProgress / 100.0)
	state.authorProgressBar = progressBar

	logEntry := widget.NewMultiLineEntry()
	logEntry.Wrapping = fyne.TextWrapOff
	logEntry.Disable()
	logEntry.SetText(state.authorLogText)
	state.authorLogEntry = logEntry
	logScroll := ui.NewFastVScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 200))
	state.authorLogScroll = logScroll

	// Log control buttons
	copyLogBtn := widget.NewButton(t.AuthorCopyLog, func() {
		if state.authorLogFilePath != "" {
			// Copy from file for accuracy
			if data, err := os.ReadFile(state.authorLogFilePath); err == nil {
				state.window.Clipboard().SetContent(string(data))
				dialog.ShowInformation(t.AuthorCopied, t.AuthorFullLogCopied, state.window)
				return
			}
		}
		// Fallback to in-memory log
		state.window.Clipboard().SetContent(state.authorLogText)
		dialog.ShowInformation(t.AuthorCopied, t.AuthorCopyLog, state.window)
	})
	copyLogBtn.Importance = widget.LowImportance

	viewFullLogBtn := widget.NewButton(t.AuthorViewFullLog, func() {
		if state.authorLogFilePath == "" || state.authorLogFilePath == "-" {
			dialog.ShowInformation(t.AuthorNoLogFile, "No log file available to view", state.window)
			return
		}
		if _, err := os.Stat(state.authorLogFilePath); err != nil {
			dialog.ShowError(fmt.Errorf("log file not found: %w", err), state.window)
			return
		}
		state.openLogViewer("Authoring Log", state.authorLogFilePath, false)
	})
	viewFullLogBtn.Importance = widget.LowImportance

	logControls := container.NewHBox(
		widget.NewLabel(t.AuthorAuthoringLogLabel),
		layout.NewSpacer(),
		copyLogBtn,
		viewFullLogBtn,
	)

	controls := container.NewVBox(
		widget.NewLabel("Generate DVD/ISO:"), // TODO: i18n
		widget.NewSeparator(),
		summaryLabel,
		widget.NewSeparator(),
		widget.NewLabel(t.AuthorStatus),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		logControls,
		logScroll,
		widget.NewSeparator(),
		generateBtn,
	)

	return ui.NewFastVScroll(container.NewPadded(controls))
}

func authorSummary(state *appState) string {
	t := i18n.T()
	summary := t.AuthorReadyToGenerate + "\n\n"
	if state.authorVideoTSPath != "" {
		summary += fmt.Sprintf("VIDEO_TS: %s\n", filepath.Base(filepath.Dir(state.authorVideoTSPath)))
	} else if len(state.authorClips) > 0 {
		summary += fmt.Sprintf("%s: %d\n", t.AuthorVideos, len(state.authorClips))
		for i, clip := range state.authorClips {
			summary += fmt.Sprintf("  %d. %s (%.2fs)\n", i+1, clip.DisplayName, clip.Duration)
		}
	} else if state.authorFile != nil {
		summary += fmt.Sprintf("Video File: %s\n", filepath.Base(state.authorFile.Path))
	}

	if len(state.authorSubtitles) > 0 {
		summary += fmt.Sprintf("Subtitle Tracks: %d\n", len(state.authorSubtitles))
		for i, path := range state.authorSubtitles {
			summary += fmt.Sprintf("  %d. %s\n", i+1, filepath.Base(path))
		}
	}

	if count, label := state.authorChapterSummary(); count > 0 {
		summary += fmt.Sprintf("%s: %d\n", label, count)
	}

	summary += fmt.Sprintf("Output Type: %s\n", state.authorOutputType)
	summary += fmt.Sprintf("Disc Size: %s\n", state.authorDiscSize)
	summary += fmt.Sprintf("Region: %s\n", state.authorRegion)
	summary += fmt.Sprintf("Aspect Ratio: %s\n", state.authorAspectRatio)
	if outPath := authorDefaultOutputPath(state.authorOutputType, authorOutputTitle(state), authorSummaryPaths(state)); outPath != "" {
		summary += fmt.Sprintf("Output Path: %s\n", outPath)
	}
	if state.authorTitle != "" {
		summary += fmt.Sprintf("DVD Title: %s\n", state.authorTitle)
	}
	if totalDur := authorTotalDuration(state); totalDur > 0 {
		bitrate := authorTargetBitrateKbps(state.authorDiscSize, totalDur)
		summary += fmt.Sprintf("Estimated Target Bitrate: %dkbps\n", bitrate)
	}
	return summary
}

func (s *appState) showTrackSelectionDialog(idx int, refresh func()) {
	t := i18n.T()
	if idx < 0 || idx >= len(s.authorClips) {
		return
	}
	clip := &s.authorClips[idx]

	src, err := probeVideo(clip.Path)
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	content := container.NewVBox()

	// Audio Tracks
	content.Add(widget.NewLabelWithStyle(t.AuthorAudioTracks, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, track := range src.Audio {
		trackIdx := track.Index
		stream := track // capture
		check := widget.NewCheck(fmt.Sprintf("Stream #%d: %s (%s, %dch)", stream.Index, stream.Codec, stream.Language, stream.Channels), nil)

		// Initial state
		isSelected := false
		for _, at := range clip.AudioTracks {
			if at.Index == trackIdx {
				isSelected = true
				break
			}
		}
		check.SetChecked(isSelected)

		check.OnChanged = func(checked bool) {
			if checked {
				clip.AudioTracks = append(clip.AudioTracks, authorAudioTrack{
					Index: trackIdx, Language: stream.Language, Codec: stream.Codec, Channels: stream.Channels,
				})
			} else {
				for i, at := range clip.AudioTracks {
					if at.Index == trackIdx {
						clip.AudioTracks = append(clip.AudioTracks[:i], clip.AudioTracks[i+1:]...)
						break
					}
				}
			}
		}
		content.Add(check)
	}

	content.Add(widget.NewSeparator())

	// Subtitle Tracks
	content.Add(widget.NewLabelWithStyle(t.AuthorSubtitleTracks, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for _, track := range src.Subtitles {
		trackIdx := track.Index
		stream := track
		check := widget.NewCheck(fmt.Sprintf("Stream #%d: %s (%s)", stream.Index, stream.Codec, stream.Language), nil)

		isSelected := false
		for _, st := range clip.SubtitleTracks {
			if st.Index == trackIdx {
				isSelected = true
				break
			}
		}
		check.SetChecked(isSelected)

		check.OnChanged = func(checked bool) {
			if checked {
				clip.SubtitleTracks = append(clip.SubtitleTracks, authorSubtitleTrack{
					Index: trackIdx, Language: stream.Language,
				})
			} else {
				for i, st := range clip.SubtitleTracks {
					if st.Index == trackIdx {
						clip.SubtitleTracks = append(clip.SubtitleTracks[:i], clip.SubtitleTracks[i+1:]...)
						break
					}
				}
			}
		}
		content.Add(check)
	}

	d := dialog.NewCustom(t.AuthorTrackSelection+": "+clip.DisplayName, t.ActionClose, ui.NewFastVScroll(content), s.window)
	d.Resize(fyne.NewSize(500, 400))
	d.Show()
}

func (s *appState) addAuthorFiles(paths []string) {
	wasEmpty := len(s.authorClips) == 0
	for _, path := range paths {
		// Check if this is a directory containing a project file
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			projPath := filepath.Join(path, "author_project.json")
			if _, err := os.Stat(projPath); err == nil {
				logging.Info(logging.CatDVD, "Loading Archivist project: %s", projPath)

				// Scan for assets
				entries, _ := os.ReadDir(path)
				clip := authorClip{
					Path:        filepath.Join(path, "video.m2v"), // Standard name for extracted video
					DisplayName: filepath.Base(path),
					Duration:    0, // Will be probed
				}

				for _, entry := range entries {
					fname := entry.Name()
					fullPath := filepath.Join(path, fname)
					if strings.HasSuffix(fname, ".ac3") {
						clip.AudioTracks = append(clip.AudioTracks, authorAudioTrack{
							Label: fname, Index: 0, ExternalPath: fullPath,
						})
					} else if strings.HasSuffix(fname, ".sup") {
						clip.SubtitleTracks = append(clip.SubtitleTracks, authorSubtitleTrack{
							Label: fname, Index: 0, ExternalPath: fullPath,
						})
					}
				}

				src, _ := probeVideo(clip.Path)
				if src != nil {
					clip.Duration = src.Duration
				}

				s.authorClips = append(s.authorClips, clip)
				continue
			}
		}

		src, err := probeVideo(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load video %s: %w", filepath.Base(path), err), s.window)
			continue
		}

		clip := authorClip{
			Path:         path,
			DisplayName:  filepath.Base(path),
			Duration:     src.Duration,
			Chapters:     []authorChapter{},
			ChapterTitle: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}

		// Auto-populate all found tracks as default
		for _, at := range src.Audio {
			clip.AudioTracks = append(clip.AudioTracks, authorAudioTrack{
				Index: at.Index, Language: at.Language, Codec: at.Codec, Channels: at.Channels,
			})
		}
		for _, st := range src.Subtitles {
			clip.SubtitleTracks = append(clip.SubtitleTracks, authorSubtitleTrack{
				Index: st.Index, Language: st.Language,
			})
		}

		s.authorClips = append(s.authorClips, clip)
	}

	if wasEmpty && len(s.authorClips) == 1 {
		s.loadEmbeddedChapters(s.authorClips[0].Path)
	} else if len(s.authorClips) > 1 && s.authorChapterSource == "embedded" {
		s.authorChapters = nil
		s.authorChapterSource = ""
	}
	s.authorTitle = ""
	s.updateAuthorSummary()
	// Update the UI for the title entry if the settings tab is currently visible.
	// This ensures the title entry visually resets as well.
	if s.active == "author" && s.window.Canvas() != nil {
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				// Rebuild the settings tab to refresh its controls.
				// This is a bit heavy, but ensures the titleEntry reflects the change.
				s.showAuthorView()
			}, false)
		}
	}
}

func (s *appState) updateAuthorSummary() {
	if s.authorSummaryLabel == nil {
		return
	}
	s.authorSummaryLabel.SetText(authorSummary(s))
}

func (s *appState) authorChapterSummary() (int, string) {
	if len(s.authorChapters) > 0 {
		switch s.authorChapterSource {
		case "embedded":
			return len(s.authorChapters), "Embedded Chapters"
		case "scenes":
			return len(s.authorChapters), "Scene Chapters"
		case "videots":
			return len(s.authorChapters), "VIDEO_TS Chapters"
		default:
			return len(s.authorChapters), "Chapters"
		}
	}
	if s.authorTreatAsChapters && len(s.authorClips) > 1 {
		return len(s.authorClips), "Clip Chapters"
	}
	return 0, ""
}

func authorTotalDuration(state *appState) float64 {
	if len(state.authorClips) > 0 {
		var total float64
		for _, clip := range state.authorClips {
			total += clip.Duration
		}
		return total
	}
	if state.authorFile != nil {
		return state.authorFile.Duration
	}
	return 0
}

func authorSummaryPaths(state *appState) []string {
	if state.authorVideoTSPath != "" {
		return []string{state.authorVideoTSPath}
	}
	if len(state.authorClips) > 0 {
		paths := make([]string, 0, len(state.authorClips))
		for _, clip := range state.authorClips {
			paths = append(paths, clip.Path)
		}
		return paths
	}
	if state.authorFile != nil {
		return []string{state.authorFile.Path}
	}
	return nil
}

func authorOutputTitle(state *appState) string {
	title := strings.TrimSpace(state.authorTitle)
	if title != "" {
		return title
	}
	if state.authorVideoTSPath != "" {
		return filepath.Base(filepath.Dir(state.authorVideoTSPath))
	}
	return defaultAuthorTitle(authorSummaryPaths(state))
}

func authorTargetBitrateKbps(discSize string, totalSeconds float64) int {
	if totalSeconds <= 0 {
		return 0
	}
	var targetBytes float64
	switch strings.ToUpper(strings.TrimSpace(discSize)) {
	case "DVD9":
		targetBytes = 7.3 * 1024 * 1024 * 1024
	default:
		targetBytes = 4.1 * 1024 * 1024 * 1024
	}
	totalBits := targetBytes * 8
	kbps := int(totalBits / totalSeconds / 1000)
	if kbps > 9500 {
		kbps = 9500
	}
	if kbps < 1500 {
		kbps = 1500
	}
	return kbps
}

func (s *appState) loadEmbeddedChapters(path string) {
	chapters, err := extractChaptersFromFile(path)
	if err != nil || len(chapters) == 0 {
		if s.authorChapterSource == "embedded" {
			s.authorChapters = nil
			s.authorChapterSource = ""
			s.updateAuthorSummary()
			if s.authorChaptersRefresh != nil {
				s.authorChaptersRefresh()
			}
		}
		return
	}
	s.authorChapters = chapters
	s.authorChapterSource = "embedded"
	s.updateAuthorSummary()
	if s.authorChaptersRefresh != nil {
		s.authorChaptersRefresh()
	}
}

func (s *appState) loadVideoTSChapters(videoTSPath string) {
	chapters, err := extractChaptersFromVideoTS(videoTSPath)
	if err != nil || len(chapters) == 0 {
		// No chapters found, clear if previously set
		if s.authorChapterSource == "videots" {
			s.authorChapters = nil
			s.authorChapterSource = ""
			s.updateAuthorSummary()
			if s.authorChaptersRefresh != nil {
				s.authorChaptersRefresh()
			}
		}
		return
	}
	s.authorChapters = chapters
	s.authorChapterSource = "videots"
	s.updateAuthorSummary()
	if s.authorChaptersRefresh != nil {
		s.authorChaptersRefresh()
	}
}

func featureClipsOnly(clips []authorClip) []authorClip {
	var features []authorClip
	for _, clip := range clips {
		if !clip.IsExtra {
			features = append(features, clip)
		}
	}
	return features
}

func chaptersFromClips(clips []authorClip) []authorChapter {
	if len(clips) == 0 {
		return nil
	}
	var chapters []authorChapter
	var t float64
	firstTitle := strings.TrimSpace(clips[0].ChapterTitle)
	if firstTitle == "" {
		firstTitle = "Chapter 1"
	}
	chapters = append(chapters, authorChapter{Timestamp: 0, Title: firstTitle, Auto: true})
	for i := 1; i < len(clips); i++ {
		t += clips[i-1].Duration
		title := strings.TrimSpace(clips[i].ChapterTitle)
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapters = append(chapters, authorChapter{
			Timestamp: t,
			Title:     title,
			Auto:      true,
		})
	}
	return chapters
}

func detectSceneChapters(path string, threshold float64) ([]authorChapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	filter := fmt.Sprintf("select='gt(scene,%.2f)',showinfo", threshold)
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(),
		"-hide_banner",
		"-loglevel", "info",
		"-i", path,
		"-vf", filter,
		"-an",
		"-f", "null",
		"-",
	)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	times := map[float64]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "pts_time:")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("pts_time:"):]
		end := strings.IndexAny(rest, " ")
		if end == -1 {
			end = len(rest)
		}
		valStr := strings.TrimSpace(rest[:end])
		if valStr == "" {
			continue
		}
		if val, err := utils.ParseFloat(valStr); err == nil {
			times[val] = struct{}{}
		}
	}

	var vals []float64
	for v := range times {
		if v < 0.01 {
			continue
		}
		vals = append(vals, v)
	}
	sort.Float64s(vals)

	if len(vals) == 0 {
		if err != nil {
			return nil, fmt.Errorf("scene detection failed: %s", strings.TrimSpace(string(out)))
		}
		return nil, nil
	}

	chapters := []authorChapter{{Timestamp: 0, Title: "Chapter 1", Auto: true}}
	for i, v := range vals {
		chapters = append(chapters, authorChapter{
			Timestamp: v,
			Title:     fmt.Sprintf("Chapter %d", i+2),
			Auto:      true,
		})
	}
	return chapters, nil
}

func extractChaptersFromFile(path string) ([]authorChapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := utils.CreateCommand(ctx, utils.GetFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_chapters",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Chapters []struct {
			StartTime string                 `json:"start_time"`
			Tags      map[string]interface{} `json:"tags"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	var chapters []authorChapter
	for i, ch := range result.Chapters {
		t, err := utils.ParseFloat(ch.StartTime)
		if err != nil {
			continue
		}
		title := ""
		if ch.Tags != nil {
			if v, ok := ch.Tags["title"]; ok {
				title = fmt.Sprintf("%v", v)
			}
		}
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapters = append(chapters, authorChapter{
			Timestamp: t,
			Title:     title,
			Auto:      true,
		})
	}

	return chapters, nil
}

func extractChaptersFromVideoTS(videoTSPath string) ([]authorChapter, error) {
	logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: searching for VOB files in: %s", videoTSPath)

	// Try to find the main title VOB files
	// Usually VTS_01_1.VOB contains the main content
	vobFiles, err := filepath.Glob(filepath.Join(videoTSPath, "VTS_*_1.VOB"))
	if err != nil {
		logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: glob error: %v", err)
		return nil, fmt.Errorf("error searching for VOB files: %w", err)
	}
	if len(vobFiles) == 0 {
		logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: no VTS_*_1.VOB files found")
		return nil, fmt.Errorf("no VOB files found in VIDEO_TS")
	}

	logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: found %d VOB files: %v", len(vobFiles), vobFiles)

	// Sort to get the first title set (usually the main feature)
	sort.Strings(vobFiles)
	mainVOB := vobFiles[0]

	logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: using main VOB: %s", mainVOB)

	// Try to extract chapters from the main VOB using ffprobe
	chapters, err := extractChaptersFromFile(mainVOB)
	if err != nil {
		logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: ffprobe error: %v", err)
		return nil, err
	}

	logging.Debug(logging.CatModule, "extractChaptersFromVideoTS: extracted %d chapters", len(chapters))
	return chapters, nil
}

func formatChapterTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func concatDVDMpg(inputs []string, output string) error {
	listPath := filepath.Join(filepath.Dir(output), "concat_list.txt")
	listFile, err := os.Create(listPath)
	if err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}
	for _, path := range inputs {
		fmt.Fprintf(listFile, "file '%s'\n", strings.ReplaceAll(path, "'", "'\\''"))
	}
	if err := listFile.Close(); err != nil {
		return fmt.Errorf("failed to write concat list: %w", err)
	}
	defer os.Remove(listPath)

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-f", "dvd", // Maintain DVD format
		"-muxrate", "10080000", // DVD mux rate
		"-packetsize", "2048", // DVD packet size
		output,
	}
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), args...)
	return cmd.Run()
}

func (s *appState) resetAuthorLog() {
	s.authorLogText = ""
	s.authorLogLines = nil
	s.authorLogFilePath = ""
	if s.authorLogEntry != nil {
		s.authorLogEntry.SetText("")
	}
	if s.authorLogScroll != nil {
		s.authorLogScroll.ScrollToTop()
	}
}

func (s *appState) appendAuthorLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}

	// Keep only last 100 lines for UI display (tail behavior)
	const maxLines = 100
	s.authorLogLines = append(s.authorLogLines, line)
	if len(s.authorLogLines) > maxLines {
		s.authorLogLines = s.authorLogLines[len(s.authorLogLines)-maxLines:]
	}

	// Rebuild text from buffer
	s.authorLogText = strings.Join(s.authorLogLines, "\n")

	if s.authorLogEntry != nil {
		s.authorLogEntry.SetText(s.authorLogText)
	}
	if s.authorLogScroll != nil {
		s.authorLogScroll.ScrollToBottom()
	}
}

func (s *appState) setAuthorStatus(text string) {
	if text == "" {
		text = "Ready"
	}
	if s.authorStatusLabel != nil {
		s.authorStatusLabel.SetText(text)
	}
}

func (s *appState) setAuthorProgress(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.authorProgress = percent
	if s.authorProgressBar != nil {
		s.authorProgressBar.SetValue(percent / 100.0)
	}
}

func (s *appState) updateAuthorCancelButton() {
	if s.authorCancelBtn == nil {
		return
	}
	if s.jobQueue == nil {
		s.authorCancelBtn.Hide()
		return
	}
	job := s.jobQueue.CurrentRunning()
	if job != nil && job.Type == queue.JobTypeAuthor {
		s.authorCancelBtn.Show()
	} else {
		s.authorCancelBtn.Hide()
	}
}

func (s *appState) startAuthorGeneration(startNow bool) {
	t := i18n.T()
	if s.authorVideoTSPath != "" {
		title := authorOutputTitle(s)
		outputPath := authorDefaultOutputPath("iso", title, []string{s.authorVideoTSPath})
		if outputPath == "" {
			dialog.ShowError(fmt.Errorf("failed to resolve output path"), s.window)
			return
		}
		if err := s.addAuthorVideoTSToQueue(s.authorVideoTSPath, title, outputPath, startNow); err != nil {
			dialog.ShowError(err, s.window)
		}
		return
	}

	paths, primary, err := s.authorSourcePaths()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	region := resolveAuthorRegion(s.authorRegion, primary)
	aspect := resolveAuthorAspect(s.authorAspectRatio, primary)
	title := strings.TrimSpace(s.authorTitle)
	if title == "" {
		title = defaultAuthorTitle(paths)
	}

	warnings := authorWarnings(s)
	uiCall := func(fn func()) {
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(fn, false)
			return
		}
		fn()
	}
	continuePrompt := func() {
		uiCall(func() {
			s.promptAuthorOutput(paths, region, aspect, title, startNow)
		})
	}
	if len(warnings) > 0 {
		uiCall(func() {
			dialog.ShowConfirm(t.AuthorAuthoringNotes, strings.Join(warnings, "\n")+"\n\nContinue?", func(ok bool) {
				if ok {
					continuePrompt()
				}
			}, s.window)
		})
		return
	}

	continuePrompt()
}

func (s *appState) promptAuthorOutput(paths []string, region, aspect, title string, startNow bool) {
	outputType := strings.ToLower(strings.TrimSpace(s.authorOutputType))
	if outputType == "" {
		outputType = "dvd"
	}

	outputPath := authorDefaultOutputPath(outputType, title, paths)
	if outputType == "iso" {
		s.generateAuthoring(paths, region, aspect, title, outputPath, true, startNow)
		return
	}
	s.generateAuthoring(paths, region, aspect, title, outputPath, false, startNow)
}

func authorWarnings(state *appState) []string {
	var warnings []string
	if len(state.authorSubtitles) > 0 {
		warnings = append(warnings, "Subtitle tracks are not authored yet; they will be ignored.")
	}
	if len(state.authorAudioTracks) > 0 {
		warnings = append(warnings, "Additional audio tracks are not authored yet; they will be ignored.")
	}
	if totalDur := authorTotalDuration(state); totalDur > 0 {
		bitrate := authorTargetBitrateKbps(state.authorDiscSize, totalDur)
		if bitrate < 3000 {
			warnings = append(warnings, fmt.Sprintf("Long runtime detected; target bitrate ~%dkbps may reduce quality.", bitrate))
		}
	}
	return warnings
}

func (s *appState) authorSourcePaths() ([]string, *videoSource, error) {
	if len(s.authorClips) > 0 {
		paths := make([]string, 0, len(s.authorClips))
		for _, clip := range s.authorClips {
			paths = append(paths, clip.Path)
		}
		primary, err := probeVideo(paths[0])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to probe source: %w", err)
		}
		return paths, primary, nil
	}

	if s.authorFile != nil {
		return []string{s.authorFile.Path}, s.authorFile, nil
	}

	return nil, nil, fmt.Errorf("no authoring content selected")
}

func resolveAuthorRegion(pref string, src *videoSource) string {
	pref = strings.ToUpper(strings.TrimSpace(pref))
	if pref == "NTSC" || pref == "PAL" {
		return pref
	}
	if src != nil {
		if src.FrameRate > 0 {
			if src.FrameRate <= 26 {
				return "PAL"
			}
			return "NTSC"
		}
		if src.Height == 576 {
			return "PAL"
		}
		if src.Height == 480 {
			return "NTSC"
		}
	}
	return "NTSC"
}

func resolveAuthorAspect(pref string, src *videoSource) string {
	pref = strings.TrimSpace(pref)
	if pref == "4:3" || pref == "16:9" {
		return pref
	}
	if src != nil && src.Width > 0 && src.Height > 0 {
		ratio := float64(src.Width) / float64(src.Height)
		if ratio >= 1.55 {
			return "16:9"
		}
		return "4:3"
	}
	return "16:9"
}

func defaultAuthorTitle(paths []string) string {
	if len(paths) == 0 {
		return "DVD"
	}
	base := filepath.Base(paths[0])
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func authorOutputFolderName(title string, paths []string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = defaultAuthorTitle(paths)
	}
	name = sanitizeForPath(name)
	if name == "" {
		name = "dvd_output"
	}
	return name
}

func authorDefaultOutputDir(outputType string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	dir := filepath.Join(home, "Videos", "VideoTools")
	if strings.EqualFold(outputType, "iso") {
		return filepath.Join(dir, "ISO_Convert")
	}
	return filepath.Join(dir, "DVD_Convert")
}

func authorDefaultOutputPath(outputType, title string, paths []string) string {
	outputType = strings.ToLower(strings.TrimSpace(outputType))
	if outputType == "" {
		outputType = "dvd"
	}
	baseDir := authorDefaultOutputDir(outputType)
	name := strings.TrimSpace(title)
	if name == "" {
		name = defaultAuthorTitle(paths)
	}
	name = sanitizeForPath(name)
	if name == "" {
		name = "dvd_output"
	}
	if outputType == "iso" {
		return uniqueFilePath(filepath.Join(baseDir, name+".iso"))
	}
	return uniqueFolderPath(filepath.Join(baseDir, name))
}

func authorTempRoot(outputPath string) string {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return utils.TempDir()
	}
	lower := strings.ToLower(trimmed)
	root := trimmed
	if strings.HasSuffix(lower, ".iso") {
		root = filepath.Dir(trimmed)
	} else if ext := filepath.Ext(trimmed); ext != "" {
		root = filepath.Dir(trimmed)
	}
	if root == "" || root == "." {
		return utils.TempDir()
	}
	return root
}

func uniqueFolderPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	for i := 1; i < 1000; i++ {
		tryPath := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			return tryPath
		}
	}
	return fmt.Sprintf("%s-%d", path, time.Now().Unix())
}

func uniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 1000; i++ {
		tryPath := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			return tryPath
		}
	}
	return fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
}

func (s *appState) generateAuthoring(paths []string, region, aspect, title, outputPath string, makeISO, startNow bool) {
	if err := s.addAuthorToQueue(paths, region, aspect, title, outputPath, makeISO, startNow); err != nil {
		dialog.ShowError(err, s.window)
	}
}

func (s *appState) addAuthorToQueue(paths []string, region, aspect, title, outputPath string, makeISO bool, startNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}

	clips := make([]map[string]interface{}, 0, len(s.authorClips))
	for _, clip := range s.authorClips {
		clips = append(clips, map[string]interface{}{
			"path":         clip.Path,
			"displayName":  clip.DisplayName,
			"duration":     clip.Duration,
			"chapterTitle": clip.ChapterTitle,
			"isExtra":      clip.IsExtra,
		})
	}
	chapters := make([]map[string]interface{}, 0, len(s.authorChapters))
	for _, ch := range s.authorChapters {
		chapters = append(chapters, map[string]interface{}{
			"timestamp": ch.Timestamp,
			"title":     ch.Title,
			"auto":      ch.Auto,
		})
	}

	config := map[string]interface{}{
		"paths":                  paths,
		"region":                 region,
		"aspect":                 aspect,
		"title":                  title,
		"outputPath":             outputPath,
		"makeISO":                makeISO,
		"treatAsChapters":        s.authorTreatAsChapters,
		"clips":                  clips,
		"chapters":               chapters,
		"discSize":               s.authorDiscSize,
		"outputType":             s.authorOutputType,
		"authorTitle":            s.authorTitle,
		"authorRegion":           s.authorRegion,
		"authorAspect":           s.authorAspectRatio,
		"createMenu":             s.authorCreateMenu,
		"chapterSource":          s.authorChapterSource,
		"subtitleTracks":         append([]string{}, s.authorSubtitles...),
		"additionalAudios":       append([]string{}, s.authorAudioTracks...),
		"menuTemplate":           s.authorMenuTemplate,
		"menuBackgroundImage":    s.authorMenuBackgroundImage,
		"menuMotionBackground":   s.authorMenuMotionBackground,
		"menuTheme":              s.authorMenuTheme,
		"menuCustomBgColor":      s.authorMenuCustomBgColor,
		"menuCustomTextColor":    s.authorMenuCustomTextColor,
		"menuCustomAccentColor":  s.authorMenuCustomAccentColor,
		"menuTitleLogoEnabled":   s.authorMenuTitleLogoEnabled,
		"menuTitleLogoPath":      s.authorMenuTitleLogoPath,
		"menuTitleLogoPosition":  s.authorMenuTitleLogoPosition,
		"menuTitleLogoScale":     s.authorMenuTitleLogoScale,
		"menuTitleLogoMargin":    s.authorMenuTitleLogoMargin,
		"menuStudioLogoEnabled":  s.authorMenuStudioLogoEnabled,
		"menuStudioLogoPath":     s.authorMenuStudioLogoPath,
		"menuStudioLogoPosition": s.authorMenuStudioLogoPosition,
		"menuStudioLogoScale":    s.authorMenuStudioLogoScale,
		"menuStudioLogoMargin":   s.authorMenuStudioLogoMargin,
		"menuStructure":          s.authorMenuStructure,
		"menuExtrasEnabled":      s.authorMenuExtrasEnabled,
		"menuChapterThumbSrc":    s.authorMenuChapterThumbnailSrc,
	}

	titleLabel := title
	if strings.TrimSpace(titleLabel) == "" {
		titleLabel = "DVD"
	}

	// Sanitize output path to ensure no special characters in filesystem operations
	sanitizedOutputPath := outputPath
	dir := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	sanitizedName := sanitizeForPath(nameWithoutExt)
	if sanitizedName == "" {
		sanitizedName = "dvd_output"
	}
	sanitizedOutputPath = filepath.Join(dir, sanitizedName+ext)

	job := &queue.Job{
		Type:        queue.JobTypeAuthor,
		Title:       fmt.Sprintf("Author DVD: %s", titleLabel),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(sanitizedOutputPath), 40)),
		InputFile:   paths[0],
		OutputFile:  sanitizedOutputPath,
		Config:      config,
	}

	s.resetAuthorLog()
	s.setAuthorStatus("Queued authoring job...")
	s.setAuthorProgress(0)
	s.jobQueue.Add(job)
	if startNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	// Navigate to queue view when starting a job immediately
	if startNow {
		s.showQueue()
	}
	return nil
}

func (s *appState) addAuthorVideoTSToQueue(videoTSPath, title, outputPath string, startNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}

	// Sanitize output path to ensure no special characters in filesystem operations
	sanitizedOutputPath := outputPath
	dir := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	sanitizedName := sanitizeForPath(nameWithoutExt)
	if sanitizedName == "" {
		sanitizedName = "dvd_output"
	}
	sanitizedOutputPath = filepath.Join(dir, sanitizedName+ext)

	job := &queue.Job{
		Type:        queue.JobTypeAuthor,
		Title:       fmt.Sprintf("Author ISO: %s", title),
		Description: fmt.Sprintf("VIDEO_TS -> %s", utils.ShortenMiddle(filepath.Base(sanitizedOutputPath), 40)),
		InputFile:   videoTSPath,
		OutputFile:  sanitizedOutputPath,
		Config: map[string]interface{}{
			"videoTSPath": videoTSPath,
			"outputPath":  sanitizedOutputPath,
			"makeISO":     true,
			"title":       title,
		},
	}

	s.resetAuthorLog()
	s.setAuthorStatus("Queued authoring job...")
	s.setAuthorProgress(0)
	s.jobQueue.Add(job)
	if startNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	// Navigate to queue view when starting a job immediately
	if startNow {
		s.showQueue()
	}
	return nil
}

func (s *appState) runAuthoringPipeline(ctx context.Context, paths []string, region, aspect, title, outputPath string, makeISO bool, clips []authorClip, chapters []authorChapter, treatAsChapters bool, createMenu bool, menuTemplate string, menuBackgroundImage string, menuMotionBackground string, menuTheme string, menuCustomBgColor, menuCustomTextColor, menuCustomAccentColor string, logos menuLogoOptions, logFn func(string), progressFn func(float64)) error {
	tempRoot := authorTempRoot(outputPath)
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		return fmt.Errorf("failed to create temp root: %w", err)
	}

	// Validate and prepare disc root BEFORE creating any workspace inside it.
	// For VIDEO_TS mode discRoot == outputPath == tempRoot, so the workDir
	// (created next) would otherwise be seen by the empty-folder guard.
	discRoot := outputPath
	var cleanup func()
	if makeISO {
		dvdTemp, err := os.MkdirTemp(tempRoot, "videotools-dvd-")
		if err != nil {
			return fmt.Errorf("failed to create DVD output directory: %w", err)
		}
		discRoot = dvdTemp
		cleanup = func() { _ = os.RemoveAll(dvdTemp) }
	} else {
		if err := prepareDiscRoot(discRoot); err != nil {
			return err
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	workDir, err := os.MkdirTemp(tempRoot, "videotools-author-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	if logFn != nil {
		logFn(fmt.Sprintf("Temp workspace: %s", workDir))
	}

	// For ISO mode the disc root is a fresh temp dir — always empty, but
	// still call prepareDiscRoot to create the VIDEO_TS sub-directory.
	if makeISO {
		if err := prepareDiscRoot(discRoot); err != nil {
			return err
		}
	}

	// Separate clips into features and extras
	var featureClips []authorClip
	var extraClips []authorClip

	if len(clips) == 0 {
		// Fallback: create default clips from paths
		for _, path := range paths {
			src, _ := probeVideo(path)
			duration := 0.0
			if src != nil {
				duration = src.Duration
			}
			c := authorClip{
				Path:        path,
				DisplayName: filepath.Base(path),
				Duration:    duration,
			}
			// Default tracks
			if src != nil {
				for _, at := range src.Audio {
					c.AudioTracks = append(c.AudioTracks, authorAudioTrack{Index: at.Index, Language: at.Language})
				}
				for _, st := range src.Subtitles {
					c.SubtitleTracks = append(c.SubtitleTracks, authorSubtitleTrack{Index: st.Index, Language: st.Language})
				}
			}
			featureClips = append(featureClips, c)
		}
	} else {
		for _, clip := range clips {
			if clip.IsExtra {
				extraClips = append(extraClips, clip)
			} else {
				featureClips = append(featureClips, clip)
			}
		}
	}

	featurePaths := make([]string, len(featureClips))
	for i, c := range featureClips {
		featurePaths[i] = c.Path
	}

	var totalDuration float64
	for _, c := range featureClips {
		totalDuration += c.Duration
	}
	for _, c := range extraClips {
		totalDuration += c.Duration
	}

	encodingProgressShare := 80.0
	otherStepsProgressShare := 20.0
	otherStepsCount := 2.0
	if makeISO {
		otherStepsCount++
	}
	progressForOtherStep := otherStepsProgressShare / otherStepsCount
	var accumulatedProgress float64

	// Encode features first
	var featureMpgPaths []string
	for i, clip := range featureClips {
		if logFn != nil {
			logFn(fmt.Sprintf("Encoding Feature %d/%d: %s", i+1, len(featureClips), clip.DisplayName))
		}
		outPath := filepath.Join(workDir, fmt.Sprintf("title_%02d.mpg", i+1))
		src, err := probeVideo(clip.Path)
		if err != nil {
			return fmt.Errorf("failed to probe %s: %w", clip.DisplayName, err)
		}

		clipProgressShare := 0.0
		if totalDuration > 0 {
			clipProgressShare = (clip.Duration / totalDuration) * encodingProgressShare
		}

		ffmpegProgressFn := func(stepPct float64) {
			overallPct := accumulatedProgress + (stepPct / 100.0 * clipProgressShare)
			if progressFn != nil {
				progressFn(overallPct)
			}
		}

		args := buildAuthorFFmpegArgs(clip, outPath, region, aspect, src.IsProgressive())
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
		}

		if err := runAuthorFFmpeg(ctx, args, src.Duration, logFn, ffmpegProgressFn); err != nil {
			return err
		}

		accumulatedProgress += clipProgressShare
		if progressFn != nil {
			progressFn(accumulatedProgress)
		}

		remuxPath := filepath.Join(workDir, fmt.Sprintf("title_%02d_remux.mpg", i+1))
		remuxArgs := []string{
			"-fflags", "+genpts",
			"-i", outPath,
			"-map", "0",
			"-map", "-0:d", // exclude data streams (dvd_nav_packet) unsupported by dvd muxer
			"-c", "copy",
			"-f", "dvd",
			"-muxrate", "10080000",
			"-packetsize", "2048",
			"-y", remuxPath,
		}
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s (remuxing for DVD compliance)", strings.Join(remuxArgs, " ")))
		}
		if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), remuxArgs, logFn); err != nil {
			return fmt.Errorf("remux failed: %w", err)
		}
		os.Remove(outPath)
		featureMpgPaths = append(featureMpgPaths, remuxPath)
	}

	// Encode extras
	var extraMpgPaths []string
	for i, clip := range extraClips {
		if logFn != nil {
			logFn(fmt.Sprintf("Encoding Extra %d/%d: %s", i+1, len(extraClips), clip.DisplayName))
		}
		outPath := filepath.Join(workDir, fmt.Sprintf("extra_%02d.mpg", i+1))
		src, err := probeVideo(clip.Path)
		if err != nil {
			return fmt.Errorf("failed to probe extra %s: %w", clip.DisplayName, err)
		}

		clipProgressShare := 0.0
		if totalDuration > 0 {
			clipProgressShare = (clip.Duration / totalDuration) * encodingProgressShare
		}

		ffmpegProgressFn := func(stepPct float64) {
			overallPct := accumulatedProgress + (stepPct / 100.0 * clipProgressShare)
			if progressFn != nil {
				progressFn(overallPct)
			}
		}

		args := buildAuthorFFmpegArgs(clip, outPath, region, aspect, src.IsProgressive())
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
		}

		if err := runAuthorFFmpeg(ctx, args, src.Duration, logFn, ffmpegProgressFn); err != nil {
			return err
		}

		accumulatedProgress += clipProgressShare
		if progressFn != nil {
			progressFn(accumulatedProgress)
		}

		remuxPath := filepath.Join(workDir, fmt.Sprintf("extra_%02d_remux.mpg", i+1))
		remuxArgs := []string{
			"-fflags", "+genpts",
			"-i", outPath,
			"-map", "0",
			"-map", "-0:d", // exclude data streams (dvd_nav_packet) unsupported by dvd muxer
			"-c", "copy",
			"-f", "dvd",
			"-muxrate", "10080000",
			"-packetsize", "2048",
			"-y", remuxPath,
		}
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s (remuxing for DVD compliance)", strings.Join(remuxArgs, " ")))
		}
		if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), remuxArgs, logFn); err != nil {
			return fmt.Errorf("remux failed: %w", err)
		}
		os.Remove(outPath)
		extraMpgPaths = append(extraMpgPaths, remuxPath)
	}

	// Filter chapters to remove any that correspond to extra clips
	// This handles the case where chapters were generated from all clips before separation
	if len(chapters) > 0 && len(extraClips) > 0 {
		filteredChapters := []authorChapter{}
		for _, ch := range chapters {
			// Check if this chapter title matches any extra clip
			isExtra := false
			for _, extra := range extraClips {
				if ch.Title == extra.ChapterTitle {
					isExtra = true
					break
				}
			}
			if !isExtra {
				filteredChapters = append(filteredChapters, ch)
			}
		}
		chapters = filteredChapters
		if logFn != nil && len(filteredChapters) < len(chapters) {
			logFn(fmt.Sprintf("Filtered out %d extra chapters, keeping %d feature chapters", len(chapters)-len(filteredChapters), len(filteredChapters)))
		}
	}

	// Generate chapters from clips if available (for professional DVD navigation)
	// Only use non-extra clips for chapters
	if len(chapters) == 0 && len(featureClips) > 1 {
		chapters = chaptersFromClips(featureClips)
		if logFn != nil {
			logFn(fmt.Sprintf("Generated %d chapter markers from video clips", len(chapters)))
		}
	}

	// Try to extract embedded chapters from single file
	if len(chapters) == 0 && len(featureMpgPaths) == 1 {
		if embed, err := extractChaptersFromFile(featurePaths[0]); err == nil && len(embed) > 0 {
			chapters = embed
			if logFn != nil {
				logFn(fmt.Sprintf("Extracted %d embedded chapters from source", len(chapters)))
			}
		}
	}

	// For professional DVD: always concatenate multiple feature files into one title with chapters
	if len(featureMpgPaths) > 1 {
		concatPath := filepath.Join(workDir, "titles_joined.mpg")
		if logFn != nil {
			logFn(fmt.Sprintf("Combining %d videos into single title with chapter markers...", len(featureMpgPaths)))
		}
		if err := concatDVDMpg(featureMpgPaths, concatPath); err != nil {
			return fmt.Errorf("failed to concatenate videos: %w", err)
		}
		featureMpgPaths = []string{concatPath}
	}

	// Log details about encoded MPG files
	if logFn != nil {
		totalMpgs := len(featureMpgPaths) + len(extraMpgPaths)
		logFn(fmt.Sprintf("Created %d MPEG file(s):", totalMpgs))
		for i, mpg := range featureMpgPaths {
			if info, err := os.Stat(mpg); err == nil {
				logFn(fmt.Sprintf("  %d. %s (%d bytes)", i+1, filepath.Base(mpg), info.Size()))
			} else {
				logFn(fmt.Sprintf("  %d. %s (stat failed: %v)", i+1, filepath.Base(mpg), err))
			}
		}
		for i, mpg := range extraMpgPaths {
			if info, err := os.Stat(mpg); err == nil {
				logFn(fmt.Sprintf("  %d. %s (EXTRA) (%d bytes)", len(featureMpgPaths)+i+1, filepath.Base(mpg), info.Size()))
			} else {
				logFn(fmt.Sprintf("  %d. %s (EXTRA) (stat failed: %v)", len(featureMpgPaths)+i+1, filepath.Base(mpg), err))
			}
		}
	}

	// Build extras list for menu (title numbers start after main feature)
	// Extras menu appears automatically when clips are marked as extras
	var extras []extraItem
	if len(extraClips) > 0 {
		for i, clip := range extraClips {
			extras = append(extras, extraItem{
				Title:    clip.ChapterTitle,
				TitleNum: i + 2, // Title 1 is main feature, extras start at title 2
			})
		}
	}

	var menuSet dvdMenuSet
	if createMenu {
		template, ok := menuTemplates[menuTemplate]
		if !ok {
			template = &SimpleMenu{}
		}
		menuSet, err = buildDVDMenuAssets(
			ctx,
			workDir,
			title,
			region,
			aspect,
			chapters,
			extras,
			logFn,
			template,
			menuBackgroundImage,
			menuMotionBackground,
			&MenuTheme{Name: menuTheme, BackgroundColor: menuCustomBgColor, TextColor: menuCustomTextColor, AccentColor: menuCustomAccentColor, IsCustom: menuTheme == "Custom"},
			logos,
			featurePaths[0], // Use first video for chapter thumbnails
			2.0,             // Default 2 second offset for chapter thumbnails
		)
		if err != nil {
			return err
		}
	}

	// Log chapter information
	if len(chapters) > 0 {
		if logFn != nil {
			logFn(fmt.Sprintf("Final DVD structure: 1 title with %d chapters", len(chapters)))
			for i, ch := range chapters {
				logFn(fmt.Sprintf("  Chapter %d: %s at %s", i+1, ch.Title, formatChapterTime(ch.Timestamp)))
			}
		}
	}

	logFn("Authoring DVD structure (Native Go Engine)...")

	// Create the DVD directory structure inside discRoot
	videoTSPath := filepath.Join(discRoot, "VIDEO_TS")
	audioTSPath := filepath.Join(discRoot, "AUDIO_TS")
	if err := os.MkdirAll(videoTSPath, 0755); err != nil {
		return fmt.Errorf("failed to create VIDEO_TS directory: %w", err)
	}
	if err := os.MkdirAll(audioTSPath, 0755); err != nil {
		return fmt.Errorf("failed to create AUDIO_TS directory: %w", err)
	}

	// Move remuxed MPG files into VIDEO_TS as VOB files.
	// DVD players expect VTS_01_1.VOB, VTS_01_2.VOB, etc. for the main feature.
	logFn(fmt.Sprintf("Placing %d encoded file(s) into VIDEO_TS as VOB...", len(featureMpgPaths)))
	for i, mpgPath := range featureMpgPaths {
		vobName := fmt.Sprintf("VTS_01_%d.VOB", i+1)
		vobPath := filepath.Join(videoTSPath, vobName)
		if err := os.Rename(mpgPath, vobPath); err != nil {
			// Cross-device rename may fail; fall back to copy+delete
			if err2 := authorCopyFile(mpgPath, vobPath); err2 != nil {
				return fmt.Errorf("failed to place %s: %w", vobName, err2)
			}
			os.Remove(mpgPath)
		}
		logFn(fmt.Sprintf("  %s placed", vobName))
	}

	// Place extras as their own title sets (VTS_02_1.VOB, etc.)
	for i, mpgPath := range extraMpgPaths {
		vtsNum := i + 2 // title sets 2, 3, ...
		vobName := fmt.Sprintf("VTS_%02d_1.VOB", vtsNum)
		vobPath := filepath.Join(videoTSPath, vobName)
		if err := os.Rename(mpgPath, vobPath); err != nil {
			if err2 := authorCopyFile(mpgPath, vobPath); err2 != nil {
				return fmt.Errorf("failed to place extra %s: %w", vobName, err2)
			}
			os.Remove(mpgPath)
		}
		logFn(fmt.Sprintf("  %s (extra) placed", vobName))
	}

	// Probe primary feature for attributes and duration
	primarySrc, _ := probeVideo(featurePaths[0])
	isNTSC := region != "PAL"

	// IFO builder targets VIDEO_TS (not discRoot)
	ifoBuilder := ifo.NewBuilder(videoTSPath)

	// Generate VTS IFO/BUP for title set 1
	vtsMat := ifo.NewVTSMAT()
	vtsMat.VTS_Attributes.CompressionMode = 1 // MPEG-2
	if region == "PAL" {
		vtsMat.VTS_Attributes.TVSystem = 1
	} else {
		vtsMat.VTS_Attributes.TVSystem = 0
	}
	if aspect == "16:9" {
		vtsMat.VTS_Attributes.AspectRatio = 3
	} else {
		vtsMat.VTS_Attributes.AspectRatio = 0
	}
	if primarySrc != nil {
		if primarySrc.Width == 720 {
			vtsMat.VTS_Attributes.Resolution = 0
		} else if primarySrc.Width == 704 {
			vtsMat.VTS_Attributes.Resolution = 1
		}
	}

	// Determine duration and audio stream count before building any IFO structures,
	// so that every GenerateVTS_IFO call (pass 1 and pass 2) has correct attrs.
	var mainDuration float64
	if primarySrc != nil {
		mainDuration = primarySrc.Duration
	}
	nAudio := uint16(len(featureClips[0].AudioTracks))
	if nAudio == 0 {
		nAudio = 1 // always at least one audio stream
	}
	vtsMat.VTS_Audio_Streams_Count = nAudio
	for i := uint16(0); i < nAudio && i < 8; i++ {
		vtsMat.VTS_Audio_Attributes[i] = ifo.AudioAttributes{
			AudioCodingMode: 0, // AC-3
			Multichannel:    0,
			SampleRate:      0, // 48 kHz
			NumChannels:     1, // 2ch stereo (value = channels - 1)
		}
	}

	// Extract chapter timestamps for PTT_SRPT / multi-cell PGC.
	// chapters[0].Timestamp is always 0 (start of title).
	var chapterTimestamps []float64
	for _, ch := range chapters {
		chapterTimestamps = append(chapterTimestamps, ch.Timestamp)
	}
	hasChapters := len(chapterTimestamps) >= 2

	// Scan VOBs for NAV_PCK positions and PTMs.
	// ScanVOBNAVPCKs reads both sector numbers and presentation timestamps so
	// we can build the VOBU_ADMAP for absolute seek and patch VOBU_SRI for
	// relative trick-play seek in one pass.
	logFn("Scanning VOBs for NAV_PCK positions...")
	mainVOBPath := filepath.Join(videoTSPath, "VTS_01_1.VOB")
	var mainNavSectors []uint32
	var mainAdmap *ifo.VOBU_ADMAP
	if navs, err2 := vob.ScanVOBNAVPCKs(mainVOBPath); err2 == nil {
		for _, n := range navs {
			mainNavSectors = append(mainNavSectors, n.Sector)
		}
		mainAdmap = ifo.BuildVOBU_ADMAP(mainNavSectors)
		logFn(fmt.Sprintf("  VTS_01_1.VOB: %d VOBUs indexed", len(navs)))
		if err2 := vob.PatchVOBUSRI(mainVOBPath, navs); err2 != nil {
			logging.Info(logging.CatDVD, "VOBU_SRI patch failed for main VOB: %v", err2)
		}
	} else {
		logging.Info(logging.CatDVD, "NAV_PCK scan failed for main VOB: %v", err2)
	}

	// Build TMAPT from VOB file size — linear approximation for seek bar.
	var mainTMAPT *ifo.VTS_TMAPT
	if info, err2 := os.Stat(mainVOBPath); err2 == nil && mainDuration > 0 {
		totalSectors := uint32(info.Size() / 2048)
		mainTMAPT = ifo.BuildLinearTMAPT(totalSectors, mainDuration, 1)
	}

	// Build PTT_SRPT and PGC for main title.
	// If chapters are available, create a multi-program PGC with one cell per
	// chapter. Otherwise fall back to a single-cell PGC.
	var mainPTTSRPT *ifo.VTS_PTT_SRPT
	var mainPGC *ifo.ProgramChain
	nChapters := uint16(1)
	if hasChapters {
		nChapters = uint16(len(chapterTimestamps))
		mainPTTSRPT = &ifo.VTS_PTT_SRPT{NrOfChapters: nChapters}
		// Placeholder sectors (0) — corrected in pass 2 for ISO builds.
		cells := ifo.ChapterCellsFromNAV(mainNavSectors, chapterTimestamps, mainDuration, 0)
		if cells != nil {
			mainPGC = ifo.BuildChapterPGC(cells, mainDuration, isNTSC)
		} else {
			mainPGC = ifo.BuildSingleCellPGC(0, 0, mainDuration, isNTSC)
		}
	} else {
		mainPGC = ifo.BuildSingleCellPGC(0, 0, mainDuration, isNTSC)
	}
	for i := uint16(0); i < nAudio && i < 8; i++ {
		mainPGC.AudioControl[i] = 0x8000 | uint16(i<<8) // active=1, stream_nr=i
	}

	if err := ifoBuilder.GenerateVTS_IFO(1, vtsMat, mainPGC, mainTMAPT, mainAdmap, mainPTTSRPT); err != nil {
		return fmt.Errorf("native ifo generation failed: %w", err)
	}

	// Generate IFOs for any extra title sets
	type extraIFOState struct {
		mat     *ifo.VTS_MAT
		pgc     *ifo.ProgramChain
		tmapt   *ifo.VTS_TMAPT
		admap   *ifo.VOBU_ADMAP
		pttsrpt *ifo.VTS_PTT_SRPT
	}
	extraStates := make([]extraIFOState, len(extraClips))
	for i, clip := range extraClips {
		vtsNum := i + 2
		extraMat := ifo.NewVTSMAT()
		extraMat.VTS_Attributes = vtsMat.VTS_Attributes
		extraMat.VTS_Audio_Streams_Count = vtsMat.VTS_Audio_Streams_Count
		extraMat.VTS_Audio_Attributes = vtsMat.VTS_Audio_Attributes
		extraPGC := ifo.BuildSingleCellPGC(0, 0, clip.Duration, isNTSC)
		for j := uint16(0); j < nAudio && j < 8; j++ {
			extraPGC.AudioControl[j] = 0x8000 | uint16(j<<8)
		}
		var extraTMAPT *ifo.VTS_TMAPT
		extraVOBPath := filepath.Join(videoTSPath, fmt.Sprintf("VTS_%02d_1.VOB", vtsNum))
		if info, err2 := os.Stat(extraVOBPath); err2 == nil && clip.Duration > 0 {
			extraTMAPT = ifo.BuildLinearTMAPT(uint32(info.Size()/2048), clip.Duration, 1)
		}
		var extraAdmap *ifo.VOBU_ADMAP
		if navs, err2 := vob.ScanVOBNAVPCKs(extraVOBPath); err2 == nil {
			extraSectors := make([]uint32, len(navs))
			for j, n := range navs {
				extraSectors[j] = n.Sector
			}
			extraAdmap = ifo.BuildVOBU_ADMAP(extraSectors)
			logFn(fmt.Sprintf("  VTS_%02d_1.VOB: %d VOBUs indexed", vtsNum, len(navs)))
			if err2 := vob.PatchVOBUSRI(extraVOBPath, navs); err2 != nil {
				logging.Info(logging.CatDVD, "VOBU_SRI patch failed for extra %d: %v", vtsNum, err2)
			}
		}
		extraStates[i] = extraIFOState{extraMat, extraPGC, extraTMAPT, extraAdmap, nil}
		if err := ifoBuilder.GenerateVTS_IFO(vtsNum, extraMat, extraPGC, extraTMAPT, extraAdmap, nil); err != nil {
			return fmt.Errorf("native ifo generation failed for extra %d: %w", vtsNum, err)
		}
	}

	// Build TT_SRPT: one entry per title set
	totalTitles := 1 + len(extraClips)
	srpt := &ifo.TT_SRPT{
		NumTitles: uint16(totalTitles),
	}
	srpt.Titles = append(srpt.Titles, ifo.TitleSearchPointer{
		TitleType:       0x01, // one sequential PGC
		NumAngles:       0x01,
		NumChapters:     nChapters,
		VTSNumber:       1,
		VTS_TitleNumber: 1,
		StartSector:     0, // updated during ISO layout
	})
	for i := range extraClips {
		srpt.Titles = append(srpt.Titles, ifo.TitleSearchPointer{
			TitleType:       0x01,
			NumAngles:       0x01,
			NumChapters:     1,
			VTSNumber:       uint8(i + 2),
			VTS_TitleNumber: 1,
			StartSector:     0,
		})
	}

	// Build optional menu PGC and place menu VOB
	menuVOBPath := filepath.Join(videoTSPath, "VIDEO_TS.VOB")
	var menuPGC *ifo.ProgramChain
	if createMenu && menuSet.MainMpg != "" && len(menuSet.MainButtons) > 0 {
		// Copy the spumux-processed menu VOB into VIDEO_TS/
		if err := authorCopyFile(menuSet.MainMpg, menuVOBPath); err != nil {
			logging.Info(logging.CatDVD, "Failed to copy menu VOB, using minimal placeholder: %v", err)
			if err2 := vob.CreateMinimalMenuVOB(menuVOBPath); err2 != nil {
				logging.Info(logging.CatDVD, "Failed to create minimal menu VOB: %v", err2)
			}
		} else {
			// Build a menu PGC with one cell command per button
			cmdTable := &ifo.DVDCommandTable{}
			cmdTable.Pre = []ifo.DVDCommand{ifo.SetHL_BTNNCommand(1)} // highlight first button on entry
			for _, btn := range menuSet.MainButtons {
				cmdTable.Cell = append(cmdTable.Cell, ifo.ParseButtonCommand(btn.Command))
			}
			menuDuration := 10.0 // default menu loop duration in seconds
			if src, err2 := probeVideo(menuSet.MainMpg); err2 == nil && src.Duration > 0 {
				menuDuration = src.Duration
			}
			menuPGC = ifo.BuildMenuPGC(cmdTable, menuDuration, isNTSC)
			logFn(fmt.Sprintf("Menu VOB placed with %d button(s)", len(menuSet.MainButtons)))
		}
	} else {
		// No menu: write a minimal single-NAV_PCK placeholder
		if err := vob.CreateMinimalMenuVOB(menuVOBPath); err != nil {
			logging.Info(logging.CatDVD, "Failed to create menu VOB: %v", err)
		}
	}

	// Generate VMG IFO/BUP (disc root metadata)
	vmgMat := ifo.NewVMGMAT()
	vmgMat.NrOfTitleSets = uint16(totalTitles)
	allMats := []*ifo.VTS_MAT{vtsMat}
	for _, st := range extraStates {
		allMats = append(allMats, st.mat)
	}
	vtsAtrt := ifo.BuildVTS_ATRT(allMats)
	if err := ifoBuilder.GenerateVMG_IFO(vmgMat, srpt, menuPGC, vtsAtrt); err != nil {
		return fmt.Errorf("native vmg generation failed: %w", err)
	}

	logFn("IFO/BUP files generated")

	accumulatedProgress += progressForOtherStep
	progressFn(accumulatedProgress)

	if makeISO {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create ISO output directory: %w", err)
		}

		// --- Two-pass disc layout ---
		// Pass 1: Compute where every file will sit on the ISO so we can write
		// correct sector addresses into the IFO files before building the image.
		// Use a throw-away writer (writes to io.Discard) for the layout scan.
		logFn("Computing disc layout...")
		layoutWriter := udf.NewWriter(io.Discard, title)
		if err := layoutWriter.AddDirFS(discRoot); err != nil {
			return fmt.Errorf("disc layout scan failed: %w", err)
		}
		layout := layoutWriter.PreAssignSectors()

		// Helper: look up disc sector for a given VIDEO_TS file.
		vtsSector := func(filename string) (uint32, uint32, bool) {
			key := "/VIDEO_TS/" + filename
			if info, ok := layout[key]; ok && info.SectorCount > 0 {
				return info.DataSector, info.SectorCount, true
			}
			return 0, 0, false
		}

		// Re-build the main title PGC with the actual disc sector range.
		if first, count, ok := vtsSector("VTS_01_1.VOB"); ok {
			lastSector := first + count - 1
			logging.Info(logging.CatDVD, "Main VOB disc sectors: %d – %d", first, lastSector)
			if hasChapters && len(mainNavSectors) > 0 {
				// Convert VOB-relative NAV_PCK sectors to disc-absolute.
				discNav := make([]uint32, len(mainNavSectors))
				for j, s := range mainNavSectors {
					discNav[j] = first + s
				}
				cells := ifo.ChapterCellsFromNAV(discNav, chapterTimestamps, mainDuration, lastSector)
				if cells != nil {
					mainPGC = ifo.BuildChapterPGC(cells, mainDuration, isNTSC)
				} else {
					mainPGC = ifo.BuildSingleCellPGC(first, lastSector, mainDuration, isNTSC)
				}
			} else {
				mainPGC = ifo.BuildSingleCellPGC(first, lastSector, mainDuration, isNTSC)
			}
			for i := uint16(0); i < nAudio && i < 8; i++ {
				mainPGC.AudioControl[i] = 0x8000 | uint16(i<<8)
			}
		}
		// Update TT_SRPT StartSector for the main title.
		if ifoFirst, _, ok := vtsSector("VTS_01_0.IFO"); ok {
			srpt.Titles[0].StartSector = ifoFirst
		}

		// Re-build IFOs for extra title sets with correct sector addresses.
		for i, clip := range extraClips {
			vtsNum := i + 2
			vobName := fmt.Sprintf("VTS_%02d_1.VOB", vtsNum)
			ifoName := fmt.Sprintf("VTS_%02d_0.IFO", vtsNum)
			if first, count, ok := vtsSector(vobName); ok {
				extraPGC2 := ifo.BuildSingleCellPGC(first, first+count-1, clip.Duration, isNTSC)
				for j := uint16(0); j < nAudio && j < 8; j++ {
					extraPGC2.AudioControl[j] = 0x8000 | uint16(j<<8)
				}
				st := extraStates[i]
				if err := ifoBuilder.GenerateVTS_IFO(vtsNum, st.mat, extraPGC2, st.tmapt, st.admap, st.pttsrpt); err != nil {
					logging.Info(logging.CatDVD, "IFO sector patch failed for extra %d: %v", vtsNum, err)
				}
			}
			if ifoFirst, _, ok := vtsSector(ifoName); ok && i+1 < len(srpt.Titles) {
				srpt.Titles[i+1].StartSector = ifoFirst
			}
		}

		// Pass 2: Rewrite VTS_01_0.IFO and VIDEO_TS.IFO with correct sectors.
		if err := ifoBuilder.GenerateVTS_IFO(1, vtsMat, mainPGC, mainTMAPT, mainAdmap, mainPTTSRPT); err != nil {
			return fmt.Errorf("ifo sector patch failed: %w", err)
		}
		if err := ifoBuilder.GenerateVMG_IFO(vmgMat, srpt, menuPGC, vtsAtrt); err != nil {
			return fmt.Errorf("vmg ifo sector patch failed: %w", err)
		}
		logFn("IFO sector addresses patched")

		// Pass 3: Build the ISO — AddDirFS(discRoot) adds VIDEO_TS/ and AUDIO_TS/
		// as proper subdirectories, matching the DVD-Video layout spec.
		logFn("Creating ISO image (Native Go UDF Writer)...")
		isoFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create iso file: %w", err)
		}
		defer isoFile.Close()

		uw := udf.NewWriter(isoFile, title)
		if err := uw.AddDirFS(discRoot); err != nil {
			return fmt.Errorf("adding disc tree to iso: %w", err)
		}
		if err := uw.Build(); err != nil {
			return fmt.Errorf("native udf build failed: %w", err)
		}

		accumulatedProgress += progressForOtherStep
		progressFn(accumulatedProgress)

		if info, err := os.Stat(outputPath); err == nil {
			logFn(fmt.Sprintf("ISO created successfully: %s (%d bytes)", filepath.Base(outputPath), info.Size()))
		}
	}

	progressFn(100.0)
	return nil
}

// authorCopyFile copies src to dst using a streaming io.Copy.
func authorCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func runAuthorFFmpeg(ctx context.Context, args []string, duration float64, logFn func(string), progressFn func(float64)) error {
	finalArgs := append([]string{"-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), finalArgs...)
	utils.ApplyNoWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if logFn != nil {
				logFn(scanner.Text())
			}
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) < 2 {
				continue
			}
			key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if key == "out_time_ms" {
				if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
					currentSec := float64(ms) / 1000000.0
					if duration > 0 {
						stepPct := (currentSec / duration) * 100.0
						if stepPct > 100 {
							stepPct = 100
						}
						if progressFn != nil {
							progressFn(stepPct)
						}
					}
				}
			}
			if logFn != nil {
				logFn(line)
			}
		}
	}()
	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

func (s *appState) executeAuthorJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		return fmt.Errorf("author job config missing")
	}
	if videoTSPath := strings.TrimSpace(toString(cfg["videoTSPath"])); videoTSPath != "" {
		outputPath := toString(cfg["outputPath"])
		title := toString(cfg["title"])
		if err := ensureAuthorDependencies(true, false); err != nil {
			return err
		}

		logFile, logPath, logErr := createAuthorLog([]string{videoTSPath}, outputPath, true, "", "", title)
		if logErr != nil {
			logging.Debug(logging.CatSystem, "author log open failed: %v", logErr)
		} else {
			job.LogPath = logPath
			s.authorLogFilePath = logPath // Store for UI access
			defer logFile.Close()
		}

		appendLog := func(line string) {
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.appendAuthorLog(line)
				}, false)
			}
		}

		updateProgress := func(percent float64) {
			progressCallback(percent)
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.setAuthorProgress(percent)
				}, false)
			}
		}

		appendLog(fmt.Sprintf("Packaging VIDEO_TS to ISO (native): %s", videoTSPath))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create ISO output directory: %w", err)
		}
		updateProgress(10)
		isoFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create ISO file: %w", err)
		}
		defer isoFile.Close()
		uw := udf.NewWriter(isoFile, isoVolumeLabel(title))
		if err := uw.AddDirFS(videoTSPath); err != nil {
			return fmt.Errorf("failed to add VIDEO_TS to ISO: %w", err)
		}
		if err := uw.Build(); err != nil {
			return fmt.Errorf("failed to build ISO: %w", err)
		}
		updateProgress(100)
		appendLog("ISO creation completed successfully.")
		return nil
	}

	rawPaths, _ := cfg["paths"].([]interface{})
	var paths []string
	for _, p := range rawPaths {
		paths = append(paths, toString(p))
	}
	if len(paths) == 0 {
		if path, ok := cfg["paths"].([]string); ok {
			paths = append(paths, path...)
		}
	}
	if len(paths) == 0 {
		if input, ok := cfg["inputPath"].(string); ok && input != "" {
			paths = append(paths, input)
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("no input paths for author job")
	}

	region := toString(cfg["region"])
	aspect := toString(cfg["aspect"])
	title := toString(cfg["title"])
	outputPath := toString(cfg["outputPath"])
	makeISO, _ := cfg["makeISO"].(bool)
	treatAsChapters, _ := cfg["treatAsChapters"].(bool)
	createMenu := toBool(cfg["createMenu"])

	if err := ensureAuthorDependencies(makeISO, createMenu); err != nil {
		return err
	}

	var clips []authorClip
	if rawClips, ok := cfg["clips"].([]interface{}); ok {
		for _, rc := range rawClips {
			if m, ok := rc.(map[string]interface{}); ok {
				clips = append(clips, authorClip{
					Path:         toString(m["path"]),
					DisplayName:  toString(m["displayName"]),
					Duration:     toFloat(m["duration"]),
					ChapterTitle: toString(m["chapterTitle"]),
					IsExtra:      toBool(m["isExtra"]),
				})
			}
		}
	}

	var chapters []authorChapter
	if rawChapters, ok := cfg["chapters"].([]interface{}); ok {
		for _, rc := range rawChapters {
			if m, ok := rc.(map[string]interface{}); ok {
				chapters = append(chapters, authorChapter{
					Timestamp: toFloat(m["timestamp"]),
					Title:     toString(m["title"]),
					Auto:      toBool(m["auto"]),
				})
			}
		}
	}

	logFile, logPath, logErr := createAuthorLog(paths, outputPath, makeISO, region, aspect, title)
	if logErr != nil {
		logging.Debug(logging.CatSystem, "author log open failed: %v", logErr)
	} else {
		job.LogPath = logPath
		s.authorLogFilePath = logPath // Store for UI access
		defer logFile.Close()
	}

	appendLog := func(line string) {
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.appendAuthorLog(line)
			}, false)
		}
	}

	updateProgress := func(percent float64) {
		progressCallback(percent)
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.setAuthorProgress(percent)
			}, false)
		}
	}

	appendLog(fmt.Sprintf("Authoring started: %s", time.Now().Format(time.RFC3339)))
	appendLog(fmt.Sprintf("Inputs: %s", strings.Join(paths, ", ")))
	appendLog(fmt.Sprintf("Output: %s", outputPath))
	if makeISO {
		appendLog("Output mode: ISO")
	} else {
		appendLog("Output mode: VIDEO_TS")
	}

	app := fyne.CurrentApp()
	if app != nil && app.Driver() != nil {
		app.Driver().DoFromGoroutine(func() {
			s.setAuthorStatus("Authoring in progress...")
		}, false)
	}

	err := s.runAuthoringPipeline(
		ctx,
		paths,
		region,
		aspect,
		title,
		outputPath,
		makeISO,
		clips,
		chapters,
		treatAsChapters,
		createMenu,
		toString(cfg["menuTemplate"]),
		toString(cfg["menuBackgroundImage"]),
		toString(cfg["menuMotionBackground"]),
		toString(cfg["menuTheme"]),
		toString(cfg["menuCustomBgColor"]),
		toString(cfg["menuCustomTextColor"]),
		toString(cfg["menuCustomAccentColor"]),
		menuLogoOptions{
			TitleLogo: menuLogo{
				Enabled:  toBool(cfg["menuTitleLogoEnabled"]),
				Path:     toString(cfg["menuTitleLogoPath"]),
				Position: toString(cfg["menuTitleLogoPosition"]),
				Scale:    toFloat(cfg["menuTitleLogoScale"]),
				Margin:   int(toFloat(cfg["menuTitleLogoMargin"])),
			},
			StudioLogo: menuLogo{
				Enabled:  toBool(cfg["menuStudioLogoEnabled"]),
				Path:     toString(cfg["menuStudioLogoPath"]),
				Position: toString(cfg["menuStudioLogoPosition"]),
				Scale:    toFloat(cfg["menuStudioLogoScale"]),
				Margin:   int(toFloat(cfg["menuStudioLogoMargin"])),
			},
		},
		appendLog,
		updateProgress,
	)
	if err != nil {
		friendly := authorFriendlyError(err)
		appendLog("ERROR: " + friendly)
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.setAuthorStatus(friendly)
			}, false)
		}
		return fmt.Errorf("%s\nSee Authoring Log for details.", friendly)
	}

	if app != nil && app.Driver() != nil {
		app.Driver().DoFromGoroutine(func() {
			s.setAuthorStatus("Authoring complete")
			s.setAuthorProgress(100)
		}, false)
	}
	appendLog("Authoring completed successfully.")
	return nil
}

func authorFriendlyError(err error) string {
	if err == nil {
		return "Authoring failed"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "disk quota exceeded"),
		strings.Contains(lower, "no space left"),
		strings.Contains(lower, "not enough space"):
		return "Not enough disk space for authoring output."
	case strings.Contains(lower, "output folder must be empty"):
		return "Output folder must be empty before authoring."
	case strings.Contains(lower, "spumux not found"), strings.Contains(lower, "spumux"):
		return "spumux not found. Install dvdauthor package for menu support."
	case strings.Contains(lower, "permission denied"):
		return "Permission denied writing to output folder."
	case strings.Contains(lower, "ffmpeg"):
		return "FFmpeg failed during DVD encoding."
	default:
		if len(msg) > 140 {
			return "Authoring failed. See Authoring Log for details."
		}
		return msg
	}
}

func prepareDiscRoot(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}
	var nonTemp []os.DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "videotools-") {
			// Auto-clean stale VideoTools workspace dirs left by crashed prior runs.
			_ = os.RemoveAll(filepath.Join(path, e.Name()))
			continue
		}
		nonTemp = append(nonTemp, e)
	}
	if len(nonTemp) > 0 {
		return fmt.Errorf("output folder must be empty: %s", path)
	}
	return nil
}

func encodeAuthorSources(clips []authorClip, region, aspect, workDir string) ([]string, error) {
	var mpgPaths []string
	for i, clip := range clips {
		idx := i + 1
		outPath := filepath.Join(workDir, fmt.Sprintf("title_%02d.mpg", idx))
		src, err := probeVideo(clip.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to probe %s: %w", clip.DisplayName, err)
		}
		args := buildAuthorFFmpegArgs(clip, outPath, region, aspect, src.IsProgressive())
		if err := runCommand(utils.GetFFmpegPath(), args); err != nil {
			return nil, err
		}
		mpgPaths = append(mpgPaths, outPath)
	}
	return mpgPaths, nil
}

func buildAuthorFFmpegArgs(clip authorClip, outputPath, region, aspect string, progressive bool) []string {
	width := 720
	height := 480
	fps := "30000/1001"
	gop := "15"
	bitrate := "6000k"
	maxrate := "9000k"

	if region == "PAL" {
		height = 576
		fps = "25"
		gop = "12"
		bitrate = "8000k"
		maxrate = "9500k"
	}

	vf := []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", width, height),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", width, height),
		fmt.Sprintf("setdar=%s", aspect),
		"setsar=1",
		fmt.Sprintf("fps=%s", fps),
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", clip.Path,
	}

	// Add external inputs
	inputMap := make(map[string]int)
	inputMap[clip.Path] = 0
	nextInputIdx := 1

	for _, at := range clip.AudioTracks {
		if at.ExternalPath != "" {
			if _, ok := inputMap[at.ExternalPath]; !ok {
				args = append(args, "-i", at.ExternalPath)
				inputMap[at.ExternalPath] = nextInputIdx
				nextInputIdx++
			}
		}
	}
	for _, st := range clip.SubtitleTracks {
		if st.ExternalPath != "" {
			if _, ok := inputMap[st.ExternalPath]; !ok {
				args = append(args, "-i", st.ExternalPath)
				inputMap[st.ExternalPath] = nextInputIdx
				nextInputIdx++
			}
		}
	}

	// Complex mapping for multitrack
	// Map video (always from primary input)
	args = append(args, "-map", "0:v:0")

	// Map all selected audio tracks
	for i, at := range clip.AudioTracks {
		inIdx := 0
		streamIdx := at.Index
		if at.ExternalPath != "" {
			inIdx = inputMap[at.ExternalPath]
		}
		args = append(args, "-map", fmt.Sprintf("%d:%d", inIdx, streamIdx))
		args = append(args, fmt.Sprintf("-c:a:%d", i), "ac3", fmt.Sprintf("-b:a:%d", i), "192k")
	}

	// Map all selected subtitle tracks
	for i, st := range clip.SubtitleTracks {
		inIdx := 0
		streamIdx := st.Index
		if st.ExternalPath != "" {
			inIdx = inputMap[st.ExternalPath]
		}
		args = append(args, "-map", fmt.Sprintf("%d:%d", inIdx, streamIdx))
		args = append(args, fmt.Sprintf("-c:s:%d", i), "dvdsub")
	}
	args = append(args,
		"-vf", strings.Join(vf, ","),
		"-c:v", "mpeg2video",
		"-r", fps,
		"-b:v", bitrate,
		"-maxrate", maxrate,
		"-bufsize", "1835k",
		"-g", gop,
		"-pix_fmt", "yuv420p",
	)

	if !progressive {
		args = append(args, "-flags", "+ilme+ildct")
	}

	args = append(args,
		"-f", "dvd",
		"-muxrate", "10080000",
		"-packetsize", "2048",
		outputPath,
	)
	return args
}

func ensureAuthorDependencies(makeISO bool, createMenu bool) error {
	if err := ensureExecutable(utils.GetFFmpegPath(), "ffmpeg"); err != nil {
		return err
	}
	// Native engine handles IFO/VOB/ISO creation.
	// dvdauthor/spumux/mkisofs are now optional fallbacks.
	return nil
}

func createAuthorLog(inputs []string, outputPath string, makeISO bool, region, aspect, title string) (*os.File, string, error) {
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	if base == "" {
		base = "author"
	}
	// Sanitize log filename to remove special characters
	base = sanitizeForPath(base)
	if base == "" {
		base = "author"
	}
	// Add timestamp prefix for chronological sorting and uniqueness
	timestamp := time.Now().Format("20060102_150405")
	logFilename := fmt.Sprintf("%s-%s-author%s", timestamp, base, conversionLogSuffix)
	logPath := filepath.Join(getLogsDir(), logFilename)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, logPath, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, logPath, err
	}
	mode := "VIDEO_TS"
	if makeISO {
		mode = "ISO"
	}
	header := fmt.Sprintf(`VideoTools Authoring Log
Started: %s
Inputs: %s
Output: %s
Mode: %s
Region: %s
Aspect: %s
Title: %s

`, time.Now().Format(time.RFC3339), strings.Join(inputs, ", "), outputPath, mode, region, aspect, title)
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

func runCommandWithLogger(ctx context.Context, name string, args []string, logFn func(string)) error {
	// Log the command being executed for debugging
	if logFn != nil {
		logFn(fmt.Sprintf(">> %s %s", name, strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, name, args...)
	utils.ApplyNoWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%s stdout: %w", name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("%s stderr: %w", name, err)
	}

	if err := cmd.Start(); err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("ERROR starting command: %v", err))
		}
		return fmt.Errorf("%s start: %w", name, err)
	}

	var wg sync.WaitGroup
	stream := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if logFn != nil {
				logFn(scanner.Text())
			}
		}
	}
	wg.Add(2)
	go stream(stdout)
	go stream(stderr)

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("ERROR command failed: %v (exit code: %v)", err, cmd.ProcessState.ExitCode()))
		}
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return false
	}
}

func ensureExecutable(path, label string) error {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath(path); err == nil {
		return nil
	}
	return fmt.Errorf("%s not found (%s)", label, path)
}

func isoVolumeLabel(title string) string {
	label := strings.ToUpper(strings.TrimSpace(title))
	if label == "" {
		label = "VIDEO_TOOLS"
	}
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	clean := strings.Trim(b.String(), "_")
	if clean == "" {
		clean = "VIDEO_TOOLS"
	}
	if len(clean) > 32 {
		clean = clean[:32]
	}
	return clean
}

func runCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	utils.ApplyNoWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *appState) showChapterPreview(videoPath string, chapters []authorChapter, callback func(bool, []authorChapter)) {
	t := i18n.T()
	loadingDlg := dialog.NewCustom("Chapter Preview", "Close", container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Detected %d chapters — generating thumbnails...", len(chapters))),
		widget.NewProgressBarInfinite(),
	), s.window)
	loadingDlg.Resize(fyne.NewSize(900, 600))
	loadingDlg.Show()

	localChapters := make([]authorChapter, len(chapters))
	copy(localChapters, chapters)

	go func() {
		thumbPaths := make([]string, len(localChapters))
		for i, ch := range localChapters {
			path, err := extractChapterThumbnail(videoPath, ch.Timestamp)
			if err != nil {
				logging.Debug(logging.CatSystem, "thumbnail extract at %.2f: %v", ch.Timestamp, err)
				continue
			}
			thumbPaths[i] = path
		}

		runOnUI(func() {
			loadingDlg.Hide()

			frameImg := canvas.NewImageFromResource(nil)
			frameImg.FillMode = canvas.ImageFillContain
			frameImg.SetMinSize(fyne.NewSize(380, 215))
			frameLabel := widget.NewLabel(t.AuthorSelectChapter)
			frameLabel.Alignment = fyne.TextAlignCenter
			frameLabel.TextStyle = fyne.TextStyle{Italic: true}

			var frameTimerMu sync.Mutex
			var frameTimer *time.Timer
			updateFrame := func(ts float64) {
				frameTimerMu.Lock()
				if frameTimer != nil {
					frameTimer.Stop()
				}
				frameTimer = time.AfterFunc(250*time.Millisecond, func() {
					path, err := extractChapterThumbnail(videoPath, ts)
					if err != nil {
						return
					}
					runOnUI(func() {
						frameImg.File = path
						frameImg.Resource = nil
						frameImg.Refresh()
						frameLabel.SetText(fmt.Sprintf("Frame at %.2fs", ts))
					})
				})
				frameTimerMu.Unlock()
			}

			listBox := container.NewVBox()
			var buildList func()
			buildList = func() {
				listBox.Objects = nil
				for i := range localChapters {
					i := i
					ch := &localChapters[i]

					var thumbObj fyne.CanvasObject
					if i < len(thumbPaths) && thumbPaths[i] != "" {
						img := canvas.NewImageFromFile(thumbPaths[i])
						img.FillMode = canvas.ImageFillContain
						img.SetMinSize(fyne.NewSize(96, 54))
						thumbObj = img
					} else {
						r := canvas.NewRectangle(color.NRGBA{R: 30, G: 32, B: 48, A: 255})
						r.SetMinSize(fyne.NewSize(96, 54))
						thumbObj = r
					}

					numLabel := widget.NewLabelWithStyle(fmt.Sprintf("%d", i+1),
						fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

					tsEntry := widget.NewEntry()
					tsEntry.SetText(fmt.Sprintf("%.2f", ch.Timestamp))
					tsEntry.SetPlaceHolder("0.00")
					tsEntry.OnChanged = func(val string) {
						ts, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
						if err != nil || ts < 0 || i >= len(localChapters) {
							return
						}
						localChapters[i].Timestamp = ts
						updateFrame(ts)
						go func() {
							time.Sleep(700 * time.Millisecond)
							path, err2 := extractChapterThumbnail(videoPath, ts)
							if err2 != nil || i >= len(thumbPaths) {
								return
							}
							runOnUI(func() {
								thumbPaths[i] = path
								buildList()
							})
						}()
					}

					titleEntry := widget.NewEntry()
					if ch.Title != "" {
						titleEntry.SetText(ch.Title)
					} else {
						titleEntry.SetPlaceHolder(fmt.Sprintf("Chapter %d", i+1))
					}
					titleEntry.OnChanged = func(val string) {
						if i < len(localChapters) {
							localChapters[i].Title = val
						}
					}

					previewBtn := widget.NewButton("Preview", func() {
						if i < len(localChapters) {
							updateFrame(localChapters[i].Timestamp)
						}
					})
					previewBtn.Importance = widget.LowImportance

					removeBtn := widget.NewButton("Remove", func() {
						if i >= len(localChapters) {
							return
						}
						localChapters = append(localChapters[:i], localChapters[i+1:]...)
						if i < len(thumbPaths) {
							thumbPaths = append(thumbPaths[:i], thumbPaths[i+1:]...)
						}
						buildList()
					})
					removeBtn.Importance = widget.LowImportance

					fields := container.NewVBox(
						container.NewGridWithColumns(2, widget.NewLabel("Time (s):"), tsEntry),
						container.NewGridWithColumns(2, widget.NewLabel("Title:"), titleEntry),
					)
					row := container.NewBorder(nil, nil,
						container.NewVBox(numLabel, container.NewPadded(thumbObj)),
						container.NewHBox(previewBtn, removeBtn),
						fields,
					)
					listBox.Add(container.NewPadded(row))
					listBox.Add(widget.NewSeparator())
				}
				listBox.Refresh()
			}
			buildList()

			addBtn := widget.NewButton("+ Add Chapter", func() {
				tsField := widget.NewEntry()
				tsField.SetPlaceHolder("e.g. 73.5")
				dlgAdd := dialog.NewCustomConfirm("Add Chapter", "Add", "Cancel",
					container.NewVBox(widget.NewLabel("Timestamp (seconds):"), tsField),
					func(confirmed bool) {
						if !confirmed {
							return
						}
						ts, err := strconv.ParseFloat(strings.TrimSpace(tsField.Text), 64)
						if err != nil || ts < 0 {
							return
						}
						localChapters = append(localChapters, authorChapter{Timestamp: ts})
						sort.Slice(localChapters, func(a, b int) bool {
							return localChapters[a].Timestamp < localChapters[b].Timestamp
						})
						newThumbs := make([]string, len(localChapters))
						copy(newThumbs, thumbPaths)
						thumbPaths = newThumbs
						buildList()
						go func() {
							path, err2 := extractChapterThumbnail(videoPath, ts)
							if err2 != nil {
								return
							}
							runOnUI(func() {
								for j, ch := range localChapters {
									if ch.Timestamp == ts && j < len(thumbPaths) {
										thumbPaths[j] = path
										buildList()
										break
									}
								}
							})
						}()
					}, s.window)
				dlgAdd.Show()
			})
			addBtn.Importance = widget.MediumImportance

			var editDlg *dialog.CustomDialog
			acceptBtn := widget.NewButton("Accept Chapters", func() {
				sort.Slice(localChapters, func(a, b int) bool {
					return localChapters[a].Timestamp < localChapters[b].Timestamp
				})
				editDlg.Hide()
				callback(true, localChapters)
			})
			acceptBtn.Importance = widget.HighImportance

			rejectBtn := widget.NewButton("Reject", func() {
				editDlg.Hide()
				callback(false, nil)
			})

			infoLabel := widget.NewLabel(fmt.Sprintf(
				"Found %d chapters — adjust timestamps or titles, then accept.",
				len(localChapters),
			))
			infoLabel.Wrapping = fyne.TextWrapWord

			leftPanel := container.NewBorder(
				container.NewVBox(infoLabel, widget.NewSeparator()),
				container.NewPadded(addBtn),
				nil, nil,
				ui.NewFastVScroll(listBox),
			)
			rightPanel := container.NewBorder(
				nil, container.NewPadded(frameLabel), nil, nil,
				container.NewPadded(frameImg),
			)
			split := container.NewHSplit(leftPanel, rightPanel)
			split.SetOffset(0.55)

			content := container.NewBorder(
				nil,
				container.NewPadded(container.NewHBox(rejectBtn, layout.NewSpacer(), acceptBtn)),
				nil, nil,
				split,
			)
			editDlg = dialog.NewCustom("Chapter Preview & Edit", "Close", content, s.window)
			editDlg.Resize(fyne.NewSize(900, 620))
			editDlg.Show()

			if len(localChapters) > 0 {
				updateFrame(localChapters[0].Timestamp)
			}
		})
	}()
}

func extractChapterThumbnail(videoPath string, timestamp float64) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "videotools-chapter-thumbs")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(tmpDir, fmt.Sprintf("thumbnail_%.2f.jpg", timestamp))
	args := []string{
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-vf", "scale=320:180",
		"-y",
		outputPath,
	}

	cmd := exec.Command(utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outputPath, nil
}

func runOnUI(fn func()) {
	fn()
}
