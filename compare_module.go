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

func mainToCompareVideoSource(v *videoSource) *compare.VideoSource {
	if v == nil {
		return nil
	}
	return &compare.VideoSource{
		Path:              v.Path,
		Format:            v.Format,
		VideoCodec:        v.VideoCodec,
		Width:             v.Width,
		Height:            v.Height,
		FrameRate:         v.FrameRate,
		Bitrate:           v.Bitrate,
		PixelFormat:       v.PixelFormat,
		ColorSpace:        v.ColorSpace,
		ColorRange:        v.ColorRange,
		FieldOrder:        v.FieldOrder,
		GOPSize:           v.GOPSize,
		AudioCodec:        v.AudioCodec,
		AudioBitrate:      v.AudioBitrate,
		AudioRate:         v.AudioRate,
		Channels:          v.Channels,
		SampleAspectRatio: v.SampleAspectRatio,
		HasChapters:       v.HasChapters,
		HasMetadata:       v.HasMetadata,
		Duration:          v.Duration,
	}
}

func buildCompareView(state *appState) fyne.CanvasObject {
	content := compare.BuildView(compare.Options{
		Window:                   state.window,
		ModuleColor:              moduleColor("compare"),
		CompareFile1:             mainToCompareVideoSource(state.compareFile1),
		CompareFile2:             mainToCompareVideoSource(state.compareFile2),
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
		OnProbeVideo: func(path string) (interface{}, error) {
			result, err := probeVideo(path)
			if err != nil {
				return nil, err
			}
			return mainToCompareVideoSource(result), nil
		},
		OnBuildVideoPane: func(_ interface{}, size fyne.Size, src interface{}, _ func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*compare.VideoSource); ok {
				ms := &videoSource{Path: vs.Path, Width: vs.Width, Height: vs.Height, FrameRate: vs.FrameRate, Duration: vs.Duration}
				return buildVideoPane(state, size, ms, nil)
			}
			return buildVideoPane(state, size, nil, nil)
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
		CompareFile1:            mainToCompareVideoSource(state.compareFile1),
		CompareFile2:            mainToCompareVideoSource(state.compareFile2),
		OnShowCompareFullscreen: state.showCompareView,
		OnGetStatsBar:           func() fyne.CanvasObject { return state.statsBar },
		OnBuildVideoPane: func(_ interface{}, size fyne.Size, src interface{}, _ func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*compare.VideoSource); ok {
				ms := &videoSource{Path: vs.Path, Width: vs.Width, Height: vs.Height, FrameRate: vs.FrameRate, Duration: vs.Duration}
				return buildVideoPane(state, size, ms, nil)
			}
			return buildVideoPane(state, size, nil, nil)
		},
	})
	return ui.NewDroppable(content, func(items []fyne.URI) {
		state.handleDrop(fyne.NewPos(0, 0), items)
	})
}
