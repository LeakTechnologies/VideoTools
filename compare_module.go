package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/compare"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func (s *appState) showCompareView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare"
	s.maximizeWindow()
	s.setContent(buildCompareView(s))
}

func (s *appState) showCompareFullscreen() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare-fullscreen"
	s.setContent(buildCompareFullscreenView(s))
}

func buildCompareView(state *appState) fyne.CanvasObject {
	content := compare.BuildView(compare.Options{
		Window:                   state.window,
		ModuleColor:              moduleColor("compare"),
		CompareFile1:             state.compareFile1,
		CompareFile2:             state.compareFile2,
		QueueBtn:                 state.queueBtn,
		OnShowMainMenu:           state.showMainMenu,
		OnShowQueue:              state.showQueue,
		OnShowCompareFullscreen:  state.showCompareFullscreen,
		OnRefreshView:            state.showCompareView,
		OnUpdateQueueButtonLabel: state.updateQueueButtonLabel,
		OnGetStatsBar:            func() fyne.CanvasObject { return state.statsBar },
		OnGetCompareFooter: func(content fyne.CanvasObject) fyne.CanvasObject {
			return moduleFooter(moduleColor("compare"), content, state.statsBar)
		},
		OnProbeVideo: func(path string) (interface{}, error) { return probeVideo(path) },
		OnBuildVideoPane: func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*videoSource); ok {
				return buildVideoPane(nil, size, vs, nil)
			}
			return nil
		},
	})
	// Window-level SetOnDropped is unreliable when a view is active; wrap the
	// compare content in its own Droppable so drag-to-load always fires.
	return ui.NewDroppable(content, func(items []fyne.URI) {
		state.handleDrop(fyne.NewPos(0, 0), items)
	})
}

func buildCompareFullscreenView(state *appState) fyne.CanvasObject {
	content := compare.BuildFullscreenView(compare.Options{
		Window:                  state.window,
		CompareFile1:            state.compareFile1,
		CompareFile2:            state.compareFile2,
		OnShowCompareFullscreen: state.showCompareView,
		OnGetStatsBar:           func() fyne.CanvasObject { return state.statsBar },
		OnBuildVideoPane: func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*videoSource); ok {
				return buildVideoPane(nil, size, vs, nil)
			}
			return nil
		},
	})
	return ui.NewDroppable(content, func(items []fyne.URI) {
		state.handleDrop(fyne.NewPos(0, 0), items)
	})
}
