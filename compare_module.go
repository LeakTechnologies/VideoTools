package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/compare"
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
	return compare.BuildView(compare.Options{
		Window:                   state.window,
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
}

func buildCompareFullscreenView(state *appState) fyne.CanvasObject {
	return compare.BuildFullscreenView(compare.Options{
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
}
