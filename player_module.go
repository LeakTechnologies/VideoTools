package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/player"
)

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
		QueueBtn:                 state.queueBtn,
		StatsBar:                 state.statsBar,
		PlayerFile:               state.playerFile,
		OnShowMainMenu:           state.showMainMenu,
		OnShowQueue:              state.showQueue,
		OnShowPlayerView:         state.showPlayerView,
		OnUpdateQueueButtonLabel: state.updateQueueButtonLabel,
		OnReleasePlaybackSession: state.releasePlaybackSession,
		OnStopPlayer:             state.stopPlayer,
		OnProbeVideo:             probeVideo,
		OnBuildVideoPane: func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject {
			return buildVideoPane(nil, size, src.(*videoSource), onSeek)
		},
		OnGetPlayerFooter: func(content fyne.CanvasObject) fyne.CanvasObject {
			return moduleFooter(moduleColor("player"), content, state.statsBar)
		},
	})
}
