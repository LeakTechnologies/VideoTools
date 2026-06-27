package queue

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

type Options struct {
	Window fyne.Window

	Jobs []*queue.Job

	OnStopPreview func()

	OnBack        func()
	OnPause       func(string)
	OnResume      func(string)
	OnCancel      func(string)
	OnRemove      func(string)
	OnMoveUp      func(string)
	OnMoveDown    func(string)
	OnPauseAll    func()
	OnResumeAll   func()
	OnStart       func()
	OnClear       func()
	OnClearAll    func()
	OnCancelAll   func()
	OnRetry       func(string)
	OnCopyError   func(string)
	OnViewLog     func(string)
	OnCopyCommand func(string)
	OnOpenFolder  func(string)
	OnOpenOutput  func(string)
	OnBurnISO     func(string)

	OnOpenInModule   func(string, string) // jobID, module name
	OnScheduleModule func(string, string) // jobID, module name

	TitleColor  color.Color
	BgColor     color.Color
	TextColor   color.Color
	AccentColor color.Color
	StatsBar    *ui.ConversionStatsBar
}

type ViewAPI interface {
	UpdateJobs(jobs []*queue.Job)
	UpdateRunningStatus(jobs []*queue.Job)
	StopAnimations()
	GetScroll() *container.Scroll
	GetRoot() fyne.CanvasObject
}

type ViewCallbacks interface {
	OnStopPreview()
	OnSetContent(obj fyne.CanvasObject)
	OnStartAutoRefresh()
	OnStopAutoRefresh()
	OnStartElapsedTicker()
	OnStopElapsedTicker()
}
