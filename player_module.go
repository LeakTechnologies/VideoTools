package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/app/modules/player"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/nav"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

func (s *appState) showPlayerViewForPath(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in showPlayerViewForPath: %v", r)
			dialog.ShowInformation("Playback Error",
				fmt.Sprintf("Failed to play video: %v\n\nThe video player encountered an error. Try using a different video or rebuilding with fresh dependencies.", r),
				s.window)
		}
	}()

	src, err := probeVideo(path)
	if err != nil {
		logging.Error(logging.CatPlayer, "probeVideo failed for %s: %v", path, err)
		return
	}
	s.playerFile = src
	s.recentFiles.Add(path, filepath.Base(path), "player")
	s.showPlayerView()
	go s.loadVideoNative(path)
}

func (s *appState) showPlayerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.maximizeWindow()
	s.setContent(buildPlayerView(s))
}

func buildPlayerView(state *appState) fyne.CanvasObject {
	return player.BuildView(player.Options{
		Window:                   state.window,
		ModuleColor:              moduleColor("player"),
		QueueBtn:                 state.queueBtn,
		StatsBar:                 state.statsBar,
		PlayerFile:               state.playerFile,
		OnShowMainMenu:           state.showMainMenu,
		OnShowQueue:              state.showQueue,
		OnShowPlayerView:         state.showPlayerView,
		OnUpdateQueueButtonLabel: state.updateQueueButtonLabel,
		OnReleasePlaybackSession: state.releasePlaybackSession,
		OnStopPlayer:             state.stopPlayer,
		OnProbeVideo:             func(path string) (interface{}, error) { return probeVideo(path) },
		OnBuildVideoPane: func(_ interface{}, size fyne.Size, src interface{}, _ func(float64)) fyne.CanvasObject {
			var vs *videoSource
			if v, ok := src.(*videoSource); ok {
				vs = v
			}
			return buildVideoPane(state, size, vs, nil)
		},
		OnGetPlayerFooter: func(content fyne.CanvasObject) fyne.CanvasObject {
			return moduleFooter(moduleColor("player"), content, state.statsBar)
		},
		OnPlayerFileLoaded: func(src interface{}) {
			if vs, ok := src.(*videoSource); ok {
				state.playerFile = vs
			}
		},
		OnLoadVideo: func(path string) {
			p := GetConvertPlayer()
			if p != nil {
				_ = p.Load(path)
			}
		},
	})
}

// showDVDDiscView analyses a disc at discPath and shows the DVD player view.
// discPath may be an ISO file or a directory containing VIDEO_TS.
func (s *appState) showDVDDiscView(discPath string) {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.maximizeWindow()

	playerCol := moduleColor("player")
	backBtn := ui.MakePillButton("< PLAYER", ui.BorderDim, s.showMainMenu)
	topBar := ui.TintedBar(playerCol, container.NewHBox(backBtn, layout.NewSpacer()))
	loadingLabel := widget.NewLabel("Analysing disc…")
	loadingLabel.Alignment = fyne.TextAlignCenter
	placeholder := container.NewBorder(topBar, nil, nil, nil,
		container.NewCenter(loadingLabel))
	s.setContent(placeholder)

	go func() {
		topo, err := nav.AnalyseDisc(discPath)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			if err != nil {
				logging.Error(logging.CatDVD, "showDVDDiscView: AnalyseDisc failed: %v", err)
				dialog.ShowError(fmt.Errorf("disc analysis failed: %w", err), s.window)
				s.showMainMenu()
				return
			}
			s.setContent(buildDVDPlayerView(s, discPath, topo))
		}, false)
	}()
}

// isDVDDisc returns true when path is an ISO file or a directory that contains
// a VIDEO_TS sub-directory.
func isDVDDisc(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".iso") {
		return true
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return false
	}
	if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
		return true
	}
	_, err = os.Stat(filepath.Join(path, "VIDEO_TS"))
	return err == nil
}

// dvdDiscRoot normalises discPath to the path accepted by InlineVideoPlayer.LoadDVD.
// ISOs pass through unchanged; VIDEO_TS directories return their parent.
func dvdDiscRoot(discPath string) string {
	if strings.EqualFold(filepath.Ext(discPath), ".iso") {
		return discPath
	}
	if strings.EqualFold(filepath.Base(discPath), "VIDEO_TS") {
		return filepath.Dir(discPath)
	}
	return discPath
}

// buildDVDPlayerView constructs the DVD player view with title/chapter navigation
// and a disc topology analysis panel.
func buildDVDPlayerView(state *appState, discPath string, topo *nav.DiscTopology) fyne.CanvasObject {
	playerCol := moduleColor("player")
	dvdRoot := dvdDiscRoot(discPath)

	dvdPlayer := ui.NewInlineVideoPlayer()
	dvdPlayer.SetIdleText("DVD PLAYER")

	// ── Video pane ─────────────────────────────────────────────────────────────
	var videoPane fyne.CanvasObject
	if w := dvdPlayer.Widget(); w != nil {
		videoPane = ui.BuildPlayerContainer(w, fyne.NewSize(0, 0))
	} else {
		bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
		bg.CornerRadius = 8
		bg.StrokeColor = ui.GridColor
		bg.StrokeWidth = 1
		txt := canvas.NewText("DVD PLAYER", color.NRGBA{R: 80, G: 80, B: 80, A: 255})
		txt.TextStyle = fyne.TextStyle{Monospace: true}
		txt.Alignment = fyne.TextAlignCenter
		videoPane = container.NewMax(bg, container.NewCenter(txt))
	}

	// ── Navigation state ────────────────────────────────────────────────────────
	selectedTitle := 0
	if len(topo.Titles) == 0 {
		selectedTitle = -1
	}

	loadTitle := func(idx int) {
		if idx < 0 || idx >= len(topo.Titles) {
			return
		}
		selectedTitle = idx
		titleN := topo.Titles[idx].Number
		go func() { _ = dvdPlayer.LoadDVD(dvdRoot, titleN) }()
	}

	if selectedTitle >= 0 {
		loadTitle(0)
	}

	// ── Chapter selector (rebuilt when title changes) ──────────────────────────
	chapterBox := container.NewVBox()
	rebuildChapters := func(titleIdx int) {
		chapterBox.Objects = nil
		if titleIdx < 0 || titleIdx >= len(topo.Titles) {
			chapterBox.Refresh()
			return
		}
		chapters := topo.Titles[titleIdx].Chapters
		if len(chapters) == 0 {
			chapterBox.Refresh()
			return
		}
		const cols = 5
		row := container.NewGridWithColumns(cols)
		dur := topo.Titles[titleIdx].Duration
		for i := range chapters {
			chapN := i + 1
			chapT := chapters[i]
			btn := widget.NewButton(fmt.Sprintf("%d", chapN), func() {
				if dur <= 0 {
					return
				}
				go dvdPlayer.Seek(chapT / dur)
			})
			btn.Importance = widget.LowImportance
			row.Add(btn)
		}
		chapterBox.Add(widget.NewLabel("Chapters"))
		chapterBox.Add(row)
		chapterBox.Refresh()
	}
	if selectedTitle >= 0 {
		rebuildChapters(selectedTitle)
	}

	// ── Title list ─────────────────────────────────────────────────────────────
	titleContainer := container.NewVBox()
	for i, t := range topo.Titles {
		idx := i
		label := fmt.Sprintf("T%02d  %s  (%d chap)", t.Number, dvdFormatDuration(t.Duration), len(t.Chapters))
		btn := ui.MakePillButton(label, ui.BorderDim, func() {
			loadTitle(idx)
			rebuildChapters(idx)
		})
		titleContainer.Add(btn)
	}

	// ── Prev/next title buttons ────────────────────────────────────────────────
	prevTitleBtn := ui.MakePillButton("◀ Title", ui.BorderDim, func() {
		if selectedTitle > 0 {
			loadTitle(selectedTitle - 1)
			rebuildChapters(selectedTitle - 1)
		}
	})
	nextTitleBtn := ui.MakePillButton("Title ▶", ui.BorderDim, func() {
		if selectedTitle < len(topo.Titles)-1 {
			loadTitle(selectedTitle + 1)
			rebuildChapters(selectedTitle + 1)
		}
	})
	navRow := container.NewHBox(prevTitleBtn, layout.NewSpacer(), nextTitleBtn)

	// ── Disc info header ───────────────────────────────────────────────────────
	var infoParts []string
	if topo.DiscType != "" {
		infoParts = append(infoParts, topo.DiscType)
	}
	if topo.Region != "" {
		infoParts = append(infoParts, topo.Region)
	}
	if topo.TotalSize > 0 {
		infoParts = append(infoParts, utils.FormatBytes(topo.TotalSize))
	}
	discInfoText := strings.Join(infoParts, "  ·  ")
	if discInfoText == "" {
		discInfoText = filepath.Base(discPath)
	}
	discInfoLabel := widget.NewLabel(discInfoText)
	discInfoLabel.TextStyle = fyne.TextStyle{Bold: true}
	titleCountLabel := widget.NewLabel(fmt.Sprintf("%d title(s)", len(topo.Titles)))

	// ── Right navigation panel ─────────────────────────────────────────────────
	rightHeader := container.NewVBox(
		discInfoLabel,
		titleCountLabel,
		widget.NewSeparator(),
		widget.NewLabel("TITLES"),
	)
	titleScroll := container.NewVScroll(titleContainer)
	titleScroll.SetMinSize(fyne.NewSize(0, 100))

	rightPanel := container.NewBorder(
		rightHeader,
		container.NewVBox(widget.NewSeparator(), chapterBox, widget.NewSeparator(), navRow),
		nil, nil,
		titleScroll,
	)

	split := container.NewHSplit(videoPane, rightPanel)
	split.SetOffset(0.73)

	// ── Top bar ────────────────────────────────────────────────────────────────
	backBtn := ui.MakePillButton("< PLAYER", ui.BorderDim, func() {
		dvdPlayer.Close()
		state.showMainMenu()
	})
	analysisBtn := ui.MakePillButton("ANALYSIS", playerCol, func() {
		txt := buildDiscAnalysisText(topo)
		lbl := widget.NewLabel(txt)
		lbl.TextStyle = fyne.TextStyle{Monospace: true}
		lbl.Wrapping = fyne.TextWrapOff
		scroll := container.NewScroll(lbl)
		scroll.SetMinSize(fyne.NewSize(600, 400))
		d := dialog.NewCustom("Disc Topology Analysis", "Close", scroll, state.window)
		d.Show()
	})
	queueBtn := state.queueBtn
	if queueBtn == nil {
		queueBtn = ui.MakePillButton("Queue", playerCol, state.showQueue)
	}
	topBar := ui.TintedBar(playerCol, container.NewHBox(backBtn, layout.NewSpacer(), analysisBtn, queueBtn))

	// ── Bottom bar ─────────────────────────────────────────────────────────────
	bottomBar := moduleFooter(playerCol, layout.NewSpacer(), state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, split)
}

// dvdFormatDuration formats seconds as "Xh Ym" or "Ym Zs".
func dvdFormatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm %02ds", m, s)
}

// buildDiscAnalysisText generates a structured text summary of a DiscTopology.
// This is the authoring reverse-engineering output: it captures every title's
// structure — duration, chapters, codec/language info — in readable form.
func buildDiscAnalysisText(topo *nav.DiscTopology) string {
	var b strings.Builder

	b.WriteString("=== DISC TOPOLOGY ANALYSIS ===\n\n")
	if topo.DiscType != "" {
		b.WriteString(fmt.Sprintf("Disc Type : %s\n", topo.DiscType))
	}
	if topo.Region != "" {
		b.WriteString(fmt.Sprintf("Region    : %s\n", topo.Region))
	}
	if topo.TotalSize > 0 {
		b.WriteString(fmt.Sprintf("Size      : %s (%d bytes)\n", utils.FormatBytes(topo.TotalSize), topo.TotalSize))
	}
	b.WriteString(fmt.Sprintf("Titles    : %d\n\n", len(topo.Titles)))

	for _, t := range topo.Titles {
		b.WriteString(fmt.Sprintf("── Title %02d (VTS %02d) ──────────────────────\n", t.Number, t.VTSNumber))
		b.WriteString(fmt.Sprintf("  Duration  : %s (%.1fs)\n", dvdFormatDuration(t.Duration), t.Duration))
		b.WriteString(fmt.Sprintf("  Chapters  : %d\n", len(t.Chapters)))
		if t.HasAngles {
			b.WriteString("  Multi-angle: yes\n")
		}
		videoMode := "Film (progressive)"
		if t.Interlaced {
			videoMode = "Video (interlaced)"
		}
		b.WriteString(fmt.Sprintf("  Video     : %s\n", videoMode))

		if len(t.Audio) > 0 {
			b.WriteString(fmt.Sprintf("  Audio (%d):\n", len(t.Audio)))
			for _, a := range t.Audio {
				lang := a.Language
				if lang == "" {
					lang = "?"
				}
				ch := ""
				if a.Channels > 0 {
					ch = fmt.Sprintf(", %dch", a.Channels)
				}
				b.WriteString(fmt.Sprintf("    [%d] %s  %s%s\n", a.Index, a.Codec, lang, ch))
			}
		}

		if len(t.Subtitles) > 0 {
			b.WriteString(fmt.Sprintf("  Subtitles (%d):\n", len(t.Subtitles)))
			for _, sub := range t.Subtitles {
				lang := sub.Language
				if lang == "" {
					lang = "?"
				}
				b.WriteString(fmt.Sprintf("    [%d] %s  %s\n", sub.Index, sub.Codec, lang))
			}
		}

		if len(t.Chapters) > 0 {
			b.WriteString("  Chapter timestamps (s):\n    ")
			for i, c := range t.Chapters {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(fmt.Sprintf("%.1f", c))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
