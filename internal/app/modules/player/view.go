package player

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

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
	Window      fyne.Window
	ModuleColor color.Color

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
	OnLoadVideo              func(path string) // Load video into player
}

func BuildView(opts Options) fyne.CanvasObject {
	playerColor := opts.ModuleColor
	if playerColor == nil {
		playerColor = utils.MustHex("#1565C0")
	}
	t := i18n.T()

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModulePlayer), func() {
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
	opts.QueueBtn = queueBtn
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

					// If we have a path, load it into the player
					if vs, ok := src.(interface{ Path() string }); ok {
						if opts.OnLoadVideo != nil {
							opts.OnLoadVideo(vs.Path())
						}
					}

					// Rebuild the view to show the loaded video
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
