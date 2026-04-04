package queue

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
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
	OnBurnISO     func(string) // Called when user wants to burn an ISO (nil = not available)

	OnOpenInModule   func(string, string) // jobID, module name - for completed jobs
	OnScheduleModule func(string, string) // jobID, module name - for pending jobs

	TitleColor color.Color
	BgColor    color.Color
	TextColor  color.Color
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
