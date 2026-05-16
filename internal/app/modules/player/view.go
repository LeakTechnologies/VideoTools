package player

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
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
	OnLoadVideo              func(path string)     // Load video into player engine
	OnPlayerFileLoaded       func(src interface{}) // Called after probe succeeds so the host can persist the file reference
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

	clearBtn := ui.NewPillButton(t.ActionClearVideo, playerColor, func() {
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

	var bottomBar fyne.CanvasObject
	if opts.OnGetPlayerFooter != nil {
		bottomBar = opts.OnGetPlayerFooter(layout.NewSpacer())
	} else {
		bottomBar = container.NewVBox(opts.StatsBar, layout.NewSpacer())
	}

	// When a video is loaded, fill the entire available space with the player
	// pane (which already includes its own seek bar and controls).
	if opts.PlayerFile != nil {
		var fileName string
		if getPath, ok := opts.PlayerFile.(interface{ Path() string }); ok {
			fileName = filepath.Base(getPath.Path())
		}

		fileLabel := widget.NewLabel(fmt.Sprintf(t.LabelFileFmt, fileName))
		fileLabel.TextStyle = fyne.TextStyle{Bold: true}

		headerRow := container.NewHBox(fileLabel, layout.NewSpacer(), clearBtn)

		// Pass size (0,0) so buildVideoPaneNative uses the defaults and the
		// container layout handles actual sizing via expansion.
		var videoPane fyne.CanvasObject
		if opts.OnBuildVideoPane != nil {
			videoPane = opts.OnBuildVideoPane(nil, fyne.NewSize(0, 0), opts.PlayerFile, nil)
		}
		if videoPane == nil {
			videoPane = container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))
		}

		return container.NewBorder(
			container.NewVBox(topBar, container.NewPadded(headerRow)),
			bottomBar,
			nil, nil,
			videoPane,
		)
	}

	// No video loaded — still use native player widget (shows SMPTE bars)
	var videoPane fyne.CanvasObject
	if opts.OnBuildVideoPane != nil {
		videoPane = opts.OnBuildVideoPane(nil, fyne.NewSize(0, 0), opts.PlayerFile, nil)
	}
	if videoPane == nil {
		videoPane = container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, videoPane)
}
