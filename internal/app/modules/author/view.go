package author

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func BuildView(opts Options) fyne.CanvasObject {
	return buildAuthorView(opts)
}

func buildAuthorView(opts Options) fyne.CanvasObject {
	opts.OnStopPreview()

	t := i18n.T()

	state := opts.GetAuthorState()
	if state == nil {
		state = &AuthorState{}
		opts.SetAuthorState(state)
	}

	initializeStateDefaults(state)

	authorColor := opts.ModuleColor
	if authorColor == nil {
		authorColor = &color.RGBA{R: 100, G: 100, B: 200, A: 255}
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleAuthor), func() {
		opts.OnShowMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := opts.QueueBtn
	if queueBtn == nil {
		queueBtn = widget.NewButton(t.ActionViewQueue, func() {
			opts.OnShowQueue()
		})
	}
	opts.QueueBtn = queueBtn

	clearCompletedBtn := widget.NewButton("⌫", func() {
		opts.OnClearCompleted()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	cancelBtn := widget.NewButton(t.ActionCancel, func() {
		opts.OnCancelJob()
	})
	cancelBtn.Importance = widget.DangerImportance

	statsBar := opts.StatsBar

	topBar := ui.TintedBar(authorColor, container.NewHBox(backBtn, layout.NewSpacer(), cancelBtn, clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(authorColor, layout.NewSpacer(), statsBar)

	tabsConfig := []struct {
		text  string
		build func() fyne.CanvasObject
	}{
		{t.AuthorVideos, func() fyne.CanvasObject { return buildVideoClipsTab(opts, state) }},
		{t.AuthorChapters, func() fyne.CanvasObject { return buildChaptersTab(opts, state) }},
		{t.ModuleSubtitles, func() fyne.CanvasObject { return buildSubtitlesTab(opts, state) }},
		{t.AuthorMenuTab, func() fyne.CanvasObject { return buildAuthorMenuTab(opts, state) }},
		{t.AuthorPreviewTab, func() fyne.CanvasObject { return buildInteractiveMenuPreviewTab(opts, state) }},
		{t.ModuleSettings, func() fyne.CanvasObject { return buildAuthorSettingsTab(opts, state) }},
		{t.AuthorGenerateTab, func() fyne.CanvasObject { return buildAuthorDiscTab(opts, state) }},
	}

	tabs := container.NewAppTabs()
	for _, cfg := range tabsConfig {
		tabs.Append(container.NewTabItem(cfg.text, cfg.build()))
	}
	tabs.SetTabLocation(container.TabLocationTop)

	opts.AuthorTabs = tabs

	return container.NewBorder(topBar, bottomBar, nil, nil, tabs)
}

func initializeStateDefaults(state *AuthorState) {
	if state.OutputType == "" {
		state.OutputType = "dvd"
	}
	if state.Region == "" {
		state.Region = "AUTO"
	}
	if state.AspectRatio == "" {
		state.AspectRatio = "AUTO"
	}
	if state.DiscSize == "" {
		state.DiscSize = "DVD5"
	}
	if state.MenuTemplate == "" {
		state.MenuTemplate = "Minimal"
	}
	if state.MenuTheme == "" {
		state.MenuTheme = "VideoTools"
	}
	if state.MenuTitleLogoPosition == "" {
		state.MenuTitleLogoPosition = "Center"
	}
	if state.MenuTitleLogoScale == 0 {
		state.MenuTitleLogoScale = 1.0
	}
	if state.MenuTitleLogoMargin == 0 {
		state.MenuTitleLogoMargin = 24
	}
	if state.MenuStudioLogoPosition == "" {
		state.MenuStudioLogoPosition = "Top Right"
	}
	if state.MenuStudioLogoScale == 0 {
		state.MenuStudioLogoScale = 1.0
	}
	if state.MenuStudioLogoMargin == 0 {
		state.MenuStudioLogoMargin = 24
	}
	if state.MenuStructure == "" {
		state.MenuStructure = "Feature + Chapters"
	}
	if state.MenuChapterThumbnailSrc == "" {
		state.MenuChapterThumbnailSrc = "Auto"
	}
	if state.SceneThreshold == 0 {
		state.SceneThreshold = 0.3
	}
}

func moduleFooter(modColor color.Color, spacer fyne.CanvasObject, statsBar fyne.CanvasObject) fyne.CanvasObject {
	footerBg := canvas.NewRectangle(modColor)
	footerContent := container.NewHBox(spacer, statsBar)
	return container.NewMax(footerBg, container.NewPadded(footerContent))
}

func buildVideoClipsTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	list := container.NewVBox()

	var rebuildList func()
	var emptyOverlay *fyne.Container
	rebuildList = func() {
		list.Objects = nil
		if len(state.Clips) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			list.Refresh()
			return
		}
		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i := range state.Clips {
			idx := i
			card := widget.NewCard(state.Clips[idx].DisplayName, "", nil)
			removeBtn := widget.NewButton(t.ActionRemove, func() {
				state.Clips = append(state.Clips[:idx], state.Clips[idx+1:]...)
				rebuildList()
				opts.OnUpdateSummary()
			})
			removeBtn.Importance = widget.MediumImportance
			cardContent := container.NewVBox(removeBtn)
			card.SetContent(cardContent)
			list.Add(card)
		}
		list.Refresh()
	}

	addBtn := widget.NewButton(t.ActionAdd, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			opts.OnAddFiles([]string{reader.URI().Path()})
			rebuildList()
		}, opts.Window)
	})
	addBtn.Importance = widget.HighImportance

	emptyLabel := widget.NewLabel(t.AuthorDragDropHint)
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(emptyOverlay)

	controls := container.NewBorder(
		widget.NewLabel(t.AuthorVideosCount),
		container.NewHBox(addBtn),
		nil,
		nil,
		listArea,
	)

	rebuildList()
	return container.NewPadded(controls)
}

func buildChaptersTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	list := container.NewVBox()
	listScroll := ui.NewFastVScroll(list)

	refreshChapters := func() {
		list.Objects = nil
		if len(state.Chapters) == 0 {
			list.Add(widget.NewLabel(t.AuthorNoChapters))
			return
		}
		for i, ch := range state.Chapters {
			title := ch.Title
			if title == "" {
				title = fmt.Sprintf("Chapter %d", i+1)
			}
			list.Add(widget.NewLabel(fmt.Sprintf("%02d. %s", i+1, title)))
		}
	}
	state.ChaptersRefresh = refreshChapters

	controlsTop := container.NewVBox(
		widget.NewLabel(t.AuthorChapters + ":"),
	)

	bottomRow := container.NewHBox()

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

func buildSubtitlesTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	list := container.NewVBox()
	listScroll := ui.NewFastVScroll(list)

	controls := container.NewBorder(
		widget.NewLabel(t.AuthorSubtitleTracks),
		nil,
		nil,
		nil,
		listScroll,
	)

	return container.NewPadded(controls)
}

func buildAuthorSettingsTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	controls := container.NewVBox(
		widget.NewLabel(t.ModuleSettings),
	)
	return ui.NewFastVScroll(container.NewPadded(controls))
}

func buildAuthorMenuTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	controls := container.NewVBox(
		widget.NewLabel(t.AuthorMenuTab),
	)
	return ui.NewFastVScroll(container.NewPadded(controls))
}

func buildInteractiveMenuPreviewTab(opts Options, state *AuthorState) fyne.CanvasObject {
	previewCanvas := canvas.NewImageFromFile("")
	previewCanvas.FillMode = canvas.ImageFillContain
	previewCanvas.SetMinSize(fyne.NewSize(320, 240))

	placeholder := widget.NewLabel("Menu preview will appear here")
	placeholder.Alignment = fyne.TextAlignCenter

	content := container.NewCenter(container.NewMax(previewCanvas, container.NewCenter(placeholder)))

	return container.NewPadded(content)
}

func buildAuthorDiscTab(opts Options, state *AuthorState) fyne.CanvasObject {
	t := i18n.T()
	summaryLabel := widget.NewLabel(authorSummary(state))
	summaryLabel.Wrapping = fyne.TextWrapWord
	state.SummaryLabel = summaryLabel

	generateBtn := widget.NewButton(t.AuthorGenerateDVD, func() {
		opts.OnAddToQueue(true)
	})
	generateBtn.Importance = widget.HighImportance

	controls := container.NewVBox(
		widget.NewLabel("Generate DVD/ISO:"),
		widget.NewSeparator(),
		summaryLabel,
		widget.NewSeparator(),
		generateBtn,
	)

	return ui.NewFastVScroll(container.NewPadded(controls))
}

func authorSummary(state *AuthorState) string {
	t := i18n.T()
	summary := t.AuthorReadyToGenerate + "\n\n"
	if len(state.Clips) > 0 {
		summary += fmt.Sprintf("%s: %d\n", t.AuthorVideos, len(state.Clips))
	}
	summary += fmt.Sprintf("Output Type: %s\n", state.OutputType)
	summary += fmt.Sprintf("Disc Size: %s\n", state.DiscSize)
	return summary
}
