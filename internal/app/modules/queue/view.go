package queue

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

type queueViewWrapper struct {
	view *ui.QueueView
}

func (w *queueViewWrapper) UpdateJobs(jobs []*queue.Job) {
	w.view.UpdateJobs(jobs)
}

func (w *queueViewWrapper) UpdateRunningStatus(jobs []*queue.Job) {
	w.view.UpdateRunningStatus(jobs)
}

func (w *queueViewWrapper) StopAnimations() {
	w.view.StopAnimations()
}

func (w *queueViewWrapper) GetScroll() *container.Scroll {
	return w.view.Scroll
}

func (w *queueViewWrapper) GetRoot() fyne.CanvasObject {
	return w.view.Root
}

func BuildView(opts Options) (fyne.CanvasObject, ViewAPI) {
	logging.Debug(logging.CatUI, "queue module: BuildView called")

	_ = i18n.T()

	win := opts.Window

	cancelWithConfirm := func(id string) {
		dialog.ShowConfirm("Cancel Job", "Cancel this job? Any progress will be lost.", func(ok bool) {
			if ok && opts.OnCancel != nil {
				opts.OnCancel(id)
			}
		}, win)
	}

	cancelAllWithConfirm := func() {
		dialog.ShowConfirm("Cancel All Jobs", "Cancel all running and queued jobs? All progress will be lost.", func(ok bool) {
			if ok && opts.OnCancelAll != nil {
				opts.OnCancelAll()
			}
		}, win)
	}

	queueView := ui.BuildQueueView(
		opts.Jobs,
		func() {
			opts.OnStopPreview()
			opts.OnBack()
		},
		opts.OnPause,
		opts.OnResume,
		cancelWithConfirm,
		opts.OnRemove,
		opts.OnMoveUp,
		opts.OnMoveDown,
		opts.OnPauseAll,
		opts.OnResumeAll,
		opts.OnStart,
		opts.OnClear,
		opts.OnClearAll,
		cancelAllWithConfirm,
		opts.OnRetry,
		opts.OnCopyError,
		opts.OnViewLog,
		opts.OnCopyCommand,
		opts.OnOpenFolder,
		opts.OnOpenOutput,
		opts.OnBurnISO,
		opts.OnOpenInModule,
		opts.OnScheduleModule,
		opts.Window,
		opts.TitleColor,
		opts.BgColor,
		opts.TextColor,
	)

	return queueView.Root, &queueViewWrapper{view: queueView}
}
