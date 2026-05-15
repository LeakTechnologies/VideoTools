//go:build native_media

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media"
	mediafilters "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media/filters"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
)

var convertInlinePlayer *ui.InlineVideoPlayer
var convertPreviewPlayer *ui.InlineVideoPlayer // right pane for processed preview
var trimInlinePlayer *ui.InlineVideoPlayer
var inspectInlinePlayer *ui.InlineVideoPlayer
var subtitleInlinePlayer *ui.InlineVideoPlayer
var upscaleInlinePlayer *ui.InlineVideoPlayer
var audioInlinePlayer *ui.InlineVideoPlayer
var filtersInlinePlayer *ui.InlineVideoPlayer
var filtersPreviewPlayer *ui.InlineVideoPlayer
var upscalePreviewPlayer *ui.InlineVideoPlayer

func init() {
	logging.Info(logging.CatSystem, "INIT: native_media build tag IS active - using InlineVideoPlayer")
	convertInlinePlayer = ui.NewInlineVideoPlayer()
	convertPreviewPlayer = ui.NewInlineVideoPlayer()
	trimInlinePlayer = ui.NewInlineVideoPlayer()
	inspectInlinePlayer = ui.NewInlineVideoPlayer()
	subtitleInlinePlayer = ui.NewInlineVideoPlayer()
	upscaleInlinePlayer = ui.NewInlineVideoPlayer()
	audioInlinePlayer = ui.NewInlineVideoPlayer()
	filtersInlinePlayer = ui.NewInlineVideoPlayer()
	filtersPreviewPlayer = ui.NewInlineVideoPlayer()
	upscalePreviewPlayer = ui.NewInlineVideoPlayer()
}

func hwDecodeEnabled() bool {
	return media.HWDecodeEnabled()
}

func setHWDecodeEnabled(enabled bool) {
	media.SetHWDecodeEnabled(enabled)
}

func HasNativeMediaPlayer() bool {
	return true
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return convertInlinePlayer
}

func GetConvertPreviewPlayer() *ui.InlineVideoPlayer {
	return convertPreviewPlayer
}

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return trimInlinePlayer
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return inspectInlinePlayer
}

func GetSubtitlePlayer() *ui.InlineVideoPlayer {
	return subtitleInlinePlayer
}

func GetUpscalePlayer() *ui.InlineVideoPlayer {
	return upscaleInlinePlayer
}

func GetAudioPlayer() *ui.InlineVideoPlayer {
	return audioInlinePlayer
}

func GetFiltersPlayer() *ui.InlineVideoPlayer {
	return filtersInlinePlayer
}

func GetFiltersPreviewPlayer() *ui.InlineVideoPlayer {
	return filtersPreviewPlayer
}

func GetUpscalePreviewPlayer() *ui.InlineVideoPlayer {
	return upscalePreviewPlayer
}

// loadFiltersVideo loads path into both the original and preview filters players.
// The preview player gets the current filter pipeline applied after load.
func (s *appState) loadFiltersVideo(path string) {
	if err := filtersInlinePlayer.Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadFiltersVideo: %v", err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
	}
	if err := filtersPreviewPlayer.Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadFiltersPreviewVideo: %v", err)
		return
	}
	s.applyFiltersPreview()
}

// applyFiltersPreview rebuilds the filter pipeline from the current appState
// filter settings and applies it to the preview player, then forces a re-decode.
func (s *appState) applyFiltersPreview() {
	if filtersPreviewPlayer == nil {
		return
	}
	pipeline := s.buildFiltersPreviewPipeline()
	filtersPreviewPlayer.SetFilterPipeline(pipeline)
	filtersPreviewPlayer.RefreshCurrentFrame()
}

// buildFiltersPreviewPipeline constructs a FilterPipeline from the current
// filter state for live preview. Supports colour correction, sharpening, and
// denoising; transform and stylistic effects are encode-only.
func (s *appState) buildFiltersPreviewPipeline() *mediafilters.FilterPipeline {
	pipeline := mediafilters.NewFilterPipeline()

	sat := s.filterSaturation
	if s.filterGrayscale {
		sat = 0
	}
	if s.filterBrightness != 0 || s.filterContrast != 1 || sat != 1 {
		pipeline.Add(mediafilters.FilterConfig{
			Type: mediafilters.FilterColor,
			Params: map[string]interface{}{
				"brightness": s.filterBrightness,
				"contrast":   s.filterContrast,
				"saturation": sat,
				"gamma":      1.0,
			},
			Enable: true,
		})
	}

	if s.filterSharpness > 0 {
		pipeline.Add(mediafilters.FilterConfig{
			Type: mediafilters.FilterSharpen,
			Params: map[string]interface{}{
				"luma":   s.filterSharpness,
				"chroma": s.filterSharpness * 0.5,
			},
			Enable: true,
		})
	}

	if s.filterDenoise > 0 {
		spatial := int(s.filterDenoise/2) + 1
		if spatial > 10 {
			spatial = 10
		}
		pipeline.Add(mediafilters.FilterConfig{
			Type: mediafilters.FilterDenoise,
			Params: map[string]interface{}{
				"spatial":  spatial,
				"temporal": spatial,
				"env":      "s",
			},
			Enable: true,
		})
	}

	return pipeline
}

func (s *appState) loadUpscalePreviewVideo(path string) {
	if err := upscalePreviewPlayer.Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadUpscalePreviewVideo: %v", err)
		return
	}
	s.applyUpscalePreview()
}

func (s *appState) applyUpscalePreview() {
	if upscalePreviewPlayer == nil {
		return
	}
	pipeline := s.buildFiltersPreviewPipeline()
	upscalePreviewPlayer.SetFilterPipeline(pipeline)
	upscalePreviewPlayer.RefreshCurrentFrame()
}

func initNativeMediaAssets(s *appState) {
	ui.SetVCRFontData(vcrOSDMono)
	// Pre-detect hardware decode capability on the main goroutine before the
	// GLFW event loop starts.  D3D11VA device creation (Windows) uses COM STA
	// dispatch which deadlocks with the GLFW message pump when called later
	// from a background goroutine.  WarmHWDeviceCache() caches the result so
	// all subsequent Load() calls return immediately without touching COM.
	media.WarmHWDeviceCache()
	ui.SetFontSizePreference(s.prefs.FontSize)
	applyVCRFontPreference(s.prefs.PlayerFont)
}

func (s *appState) loadVideoNative(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in loadVideoNative: %v", r)
		}
	}()
	if err := convertInlinePlayer.Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadVideoNative failed: path=%s err=%v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
	}
}

func (s *appState) playNative() {
	convertInlinePlayer.Play()
}

func (s *appState) pauseNative() {
	convertInlinePlayer.Pause()
}

func (s *appState) seekNative(target float64) {
	convertInlinePlayer.Seek(target)
}

func (s *appState) stepFrameNative(dir int) {
	convertInlinePlayer.StepFrame(dir)
}

func (s *appState) scrubNative(target float64) {
	convertInlinePlayer.ScrubTo(target)
}

func (s *appState) renderDualPlayerPreview(seconds float64, duration time.Duration) {
	// Renders 5 seconds of processed video at the seek position called from upscale module
	logging.Info(logging.CatPlayer, "renderDualPlayerPreview: pos=%.1fs duration=%v", seconds, duration)
	
	if s.upscaleFile == nil {
		logging.Warning(logging.CatPlayer, "renderDualPlayerPreview: no source file loaded")
		return
	}
	
	// TODO: Implement actual FFmpeg rendering with filter/AI settings
	// 1. Get current filter chain or AI settings
	// 2. Run FFmpeg to render segment 
	// 3. Load result into convertPreviewPlayer
}

func (s *appState) selectAudioTrackNative(idx int) {
	if err := convertInlinePlayer.SelectAudioTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectAudioTrack(%d): %v", idx, err)
	}
}

func (s *appState) setVolumeNative(vol float64) {
	convertInlinePlayer.SetVolume(vol)
}

func (s *appState) setMutedNative(muted bool) {
	convertInlinePlayer.SetMuted(muted)
}

func (s *appState) selectSubtitleTrackNative(idx int) {
	if idx < 0 {
		convertInlinePlayer.DisableSubtitles()
		return
	}
	if err := convertInlinePlayer.SelectSubtitleTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectSubtitleTrack(%d): %v", idx, err)
	}
}

func (s *appState) closeNativePlayer() {
	convertInlinePlayer.Close()
}

func BuildConvertPlayerPane(size fyne.Size) (fyne.CanvasObject, *ui.InlineVideoPlayer) {
	return ui.BuildInlinePlayerPane(size)
}
