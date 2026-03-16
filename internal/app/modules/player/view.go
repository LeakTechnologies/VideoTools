package player

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type Options struct {
	Window fyne.Window

	PlayerFile interface{}
	QueueBtn   *widget.Button
	StatsBar   fyne.CanvasObject

	OnShowMainMenu           func()
	OnShowQueue              func()
	OnShowPlayerView         func()
	OnUpdateQueueButtonLabel func()
	OnReleasePlaybackSession func()
	OnStopPlayer             func()
	OnProbeVideo             func(path string) (interface{}, error)
	OnBuildVideoPane         func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject
	OnGetPlayerFooter        func(content fyne.CanvasObject) fyne.CanvasObject
}

func BuildView(opts Options) fyne.CanvasObject {
	playerColor := utils.MustHex("#4CAF50")
	t := i18n.T()

	backBtn := widget.NewButton("< "+t.ModulePlayer, func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})
	if opts.QueueBtn != nil {
		opts.QueueBtn = queueBtn
	}
	if opts.OnUpdateQueueButtonLabel != nil {
		opts.OnUpdateQueueButtonLabel()
	}
	topBar := ui.TintedBar(playerColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	instructions := widget.NewLabel(t.PlayerInstructions)
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	fileLabel := widget.NewLabel(t.LabelNoFile)
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	playerSize := fyne.NewSize(640, 360)

	var videoContainer fyne.CanvasObject
	if opts.PlayerFile != nil {
		if getPath, ok := opts.PlayerFile.(interface{ Path() string }); ok {
			fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filepath.Base(getPath.Path())))
		}
		if opts.OnBuildVideoPane != nil {
			videoContainer = opts.OnBuildVideoPane(nil, playerSize, opts.PlayerFile, nil)
		}
	} else {
		videoContainer = container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))
	}

	loadBtn := widget.NewButton(t.ActionLoadVideo, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				var src interface{}
				var probeErr error
				if opts.OnProbeVideo != nil {
					src, probeErr = opts.OnProbeVideo(path)
				}
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if probeErr != nil {
						dialog.ShowError(probeErr, opts.Window)
						return
					}

					opts.PlayerFile = src
					if opts.OnShowPlayerView != nil {
						opts.OnShowPlayerView()
					}
				}, false)
			}()
		}, opts.Window)
	})
	loadBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton(t.ActionClearVideo, func() {
		if opts.OnReleasePlaybackSession != nil {
			opts.OnReleasePlaybackSession()
		}
		if opts.OnStopPlayer != nil {
			opts.OnStopPlayer()
		}
		opts.PlayerFile = nil
		if opts.OnShowPlayerView != nil {
			opts.OnShowPlayerView()
		}
	})
	clearBtn.Importance = widget.MediumImportance

	buttonContainer := container.NewHBox(loadBtn, clearBtn)

	mainContent := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		buttonContainer,
		videoContainer,
	)

	content := container.NewPadded(mainContent)
	var bottomBar fyne.CanvasObject
	if opts.OnGetPlayerFooter != nil {
		bottomBar = opts.OnGetPlayerFooter(layout.NewSpacer())
	} else {
		bottomBar = container.NewVBox(opts.StatsBar, layout.NewSpacer())
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
