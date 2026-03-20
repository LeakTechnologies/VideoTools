package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/player"
)

func (s *appState) showPlayerViewForPath(path string) {
	src, err := probeVideo(path)
	if err != nil {
		return
	}
	s.playerFile = src
	s.showPlayerView()
}

func (s *appState) showPlayerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.maximizeWindow()
	s.setContent(buildPlayerView(s))
}

func buildPlayerView(state *appState) fyne.CanvasObject {
	return player.BuildView(player.Options{
		Window:                   state.window,
		ModuleColor:              moduleColor("player"),
		QueueBtn:                 state.queueBtn,
		StatsBar:                 state.statsBar,
		PlayerFile:               state.playerFile,
		OnShowMainMenu:           state.showMainMenu,
		OnShowQueue:              state.showQueue,
		OnShowPlayerView:         state.showPlayerView,
		OnUpdateQueueButtonLabel: state.updateQueueButtonLabel,
		OnReleasePlaybackSession: state.releasePlaybackSession,
		OnStopPlayer:             state.stopPlayer,
		OnProbeVideo: func(path string) (interface{}, error) { return probeVideo(path) },
		OnBuildVideoPane: func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*videoSource); ok {
				return buildVideoPane(nil, size, vs, nil)
			}
			return nil
		},
		OnGetPlayerFooter: func(content fyne.CanvasObject) fyne.CanvasObject {
			return moduleFooter(moduleColor("player"), content, state.statsBar)
		},
	})
}
