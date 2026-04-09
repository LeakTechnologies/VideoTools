package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/player"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

func (s *appState) showPlayerViewForPath(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in showPlayerViewForPath: %v", r)
			dialog.ShowInformation("Playback Error",
				fmt.Sprintf("Failed to play video: %v\n\nThe video player encountered an error. Try using a different video or rebuilding with fresh dependencies.", r),
				s.window)
		}
	}()

	src, err := probeVideo(path)
	if err != nil {
		logging.Error(logging.CatPlayer, "probeVideo failed for %s: %v", path, err)
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
		OnProbeVideo:             func(path string) (interface{}, error) { return probeVideo(path) },
		OnBuildVideoPane: func(_ interface{}, size fyne.Size, src interface{}, _ func(float64)) fyne.CanvasObject {
			if vs, ok := src.(*videoSource); ok {
				return buildVideoPane(state, size, vs, nil)
			}
			return nil
		},
		OnGetPlayerFooter: func(content fyne.CanvasObject) fyne.CanvasObject {
			return moduleFooter(moduleColor("player"), content, state.statsBar)
		},
		OnLoadVideo: func(path string) {
			player := GetConvertPlayer()
			if player != nil {
				_ = player.Load(path)
			}
		},
	})
}
