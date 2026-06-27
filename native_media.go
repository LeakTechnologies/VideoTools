//go:build native_media

package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/media"
	mediafilters "github.com/LeakTechnologies/VideoTools/internal/media/filters"
	"github.com/LeakTechnologies/VideoTools/internal/media/state"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

var primaryInlinePlayer *ui.InlineVideoPlayer // single player for all single-playback modules
var previewPlayer *ui.InlineVideoPlayer       // preview player for Filters/Upscale comparison

func init() {
	logging.Info(logging.CatSystem, "INIT: native_media build tag IS active - using InlineVideoPlayer")
	primaryInlinePlayer = ui.NewInlineVideoPlayer()
	previewPlayer = ui.NewInlineVideoPlayer()

	// Mirror play/pause/seek from primary to preview; disable preview controls
	// so both players are driven by the primary's transport bar only.
	primaryInlinePlayer.SetPeer(previewPlayer)
}

func autoDeinterlaceEnabled() bool {
	return media.GetDefaultDeinterlaceEnabled()
}

func setAutoDeinterlace(enabled bool) {
	media.SetDefaultDeinterlaceEnabled(enabled)
	players := []*ui.InlineVideoPlayer{primaryInlinePlayer, previewPlayer}
	for _, p := range players {
		if p != nil {
			p.SetDeinterlaceEnabled(enabled)
		}
	}
}

func hwDecodeEnabled() bool {
	return media.HWDecodeEnabled()
}

func setHWDecodeEnabled(enabled bool) {
	media.SetHWDecodeEnabled(enabled)
}

func setHWCodecDenyList(s string) {
	media.SetHWCodecDenyList(s)
}

func seekAccuracyFromString(s string) media.SeekAccuracy {
	switch s {
	case "frame":
		return media.SeekAccuracyFrame
	case "accurate":
		return media.SeekAccuracyAccurate
	default:
		return media.SeekAccuracyKeyframe
	}
}

func setPlayerSeekAccuracy(accuracy string) {
	acc := seekAccuracyFromString(accuracy)
	media.SetDefaultSeekAccuracy(acc)
	players := []*ui.InlineVideoPlayer{primaryInlinePlayer, previewPlayer}
	for _, p := range players {
		if p != nil {
			p.SetSeekAccuracy(acc)
		}
	}
}

func setPlayerAVOffset(ms int) {
	d := float64(ms) / 1000.0
	media.SetDefaultAudioDelay(d)
	players := []*ui.InlineVideoPlayer{primaryInlinePlayer, previewPlayer}
	for _, p := range players {
		if p != nil {
			p.SetAudioDelay(d)
		}
	}
}

// parseAspectRatio converts "W:H" to a float ratio. Returns 0 on parse failure.
func parseAspectRatio(s string) float64 {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0
	}
	w, errW := strconv.ParseFloat(parts[0], 64)
	h, errH := strconv.ParseFloat(parts[1], 64)
	if errW != nil || errH != nil || h == 0 {
		return 0
	}
	return w / h
}

func applyPlayerDefaultAspect(aspect string) {
	ratio := parseAspectRatio(aspect)
	if ratio <= 0 {
		ratio = 16.0 / 9.0
	}
	players := []*ui.InlineVideoPlayer{primaryInlinePlayer, previewPlayer}
	for _, p := range players {
		if p != nil {
			p.SetIdleAspectRatio(ratio)
		}
	}
}

func HasNativeMediaPlayer() bool {
	return true
}

func GetPrimaryPlayer() *ui.InlineVideoPlayer {
	return primaryInlinePlayer
}

func GetPreviewPlayer() *ui.InlineVideoPlayer {
	return previewPlayer
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetConvertPreviewPlayer() *ui.InlineVideoPlayer {
	return GetPreviewPlayer()
}

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetSubtitlePlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetUpscalePlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetAudioPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetFiltersPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetFiltersPreviewPlayer() *ui.InlineVideoPlayer {
	return GetPreviewPlayer()
}

func GetUpscalePreviewPlayer() *ui.InlineVideoPlayer {
	return GetPreviewPlayer()
}

// loadFiltersVideo loads path into both the original and preview filters players.
// The preview player gets the current filter pipeline applied after load.
func (s *appState) loadFiltersVideo(path string) {
	if err := GetFiltersPlayer().Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadFiltersVideo: %v", err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
	}
	if err := GetFiltersPreviewPlayer().Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadFiltersPreviewVideo: %v", err)
		return
	}
	GetFiltersPreviewPlayer().SetMuted(true)
	s.applyFiltersPreview()
}

// applyFiltersPreview rebuilds the filter pipeline from the current appState
// filter settings and applies it to the preview player, then forces a re-decode.
func (s *appState) applyFiltersPreview() {
	if GetFiltersPreviewPlayer() == nil {
		return
	}
	pipeline := s.buildFiltersPreviewPipeline()
	GetFiltersPreviewPlayer().SetFilterPipeline(pipeline)
	GetFiltersPreviewPlayer().RefreshCurrentFrame()
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
	if err := GetUpscalePreviewPlayer().Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadUpscalePreviewVideo: %v", err)
		return
	}
	GetUpscalePreviewPlayer().SetMuted(true)
	s.applyUpscalePreview()
}

func (s *appState) applyUpscalePreview() {
	if GetUpscalePreviewPlayer() == nil {
		return
	}
	pipeline := s.buildFiltersPreviewPipeline()
	GetUpscalePreviewPlayer().SetFilterPipeline(pipeline)
	GetUpscalePreviewPlayer().RefreshCurrentFrame()
}

func initNativeMediaAssets(s *appState) {
	ui.SetVCRFontData(vcrOSDMono)
	// Pre-detect hardware decode capability on the main goroutine before the
	// GLFW event loop starts.  D3D11VA device creation (Windows) uses COM STA
	// dispatch which deadlocks with the GLFW message pump when called later
	// from a background goroutine.  WarmHWDeviceCache() caches the result so
	// all subsequent Load() calls return immediately without touching COM.
	media.WarmHWDeviceCache()
	setAutoDeinterlace(s.prefs.AutoDeinterlace)
	setPlayerSeekAccuracy(s.prefs.SeekAccuracy)
	setPlayerAVOffset(s.prefs.AVOffset)
	setHWCodecDenyList(s.prefs.HWCodecDenyList)
	ui.SetFontSizePreference(s.prefs.FontSize)
	applyVCRFontPreference(s.prefs.PlayerFont)
	applyPlayerDefaultAspect(s.prefs.PlayerDefaultAspect)

	// Initialise shared resume-state store for the player singletons.
	if rs, err := state.NewResumeState(filepath.Join(defaultVideoToolsRoot(), "state")); err != nil {
		logging.Warning(logging.CatPlayer, "Failed to init ResumeState: %v", err)
	} else {
		primaryInlinePlayer.SetResumeState(rs)
		previewPlayer.SetResumeState(rs)
	}
}

func (s *appState) loadVideoNative(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in loadVideoNative: %v", r)
		}
	}()
	if err := GetConvertPlayer().Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadVideoNative failed: path=%s err=%v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
	}
}

func (s *appState) playNative() {
	GetConvertPlayer().Play()
}

func (s *appState) pauseNative() {
	GetConvertPlayer().Pause()
}

func (s *appState) seekNative(target float64) {
	GetConvertPlayer().Seek(target)
}

func (s *appState) stepFrameNative(dir int) {
	GetConvertPlayer().StepFrame(dir)
}

func (s *appState) scrubNative(target float64) {
	GetConvertPlayer().ScrubTo(target)
}

func (s *appState) renderDualPlayerPreview(seconds float64, duration time.Duration) {
	logging.Warning(logging.CatPlayer, "renderDualPlayerPreview: not yet implemented (pos=%.1fs duration=%v)", seconds, duration)
}

func (s *appState) selectAudioTrackNative(idx int) {
	if err := GetConvertPlayer().SelectAudioTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectAudioTrack(%d): %v", idx, err)
	}
}

func (s *appState) setVolumeNative(vol float64) {
	GetConvertPlayer().SetVolume(vol)
}

func (s *appState) setMutedNative(muted bool) {
	GetConvertPlayer().SetMuted(muted)
}

func (s *appState) selectSubtitleTrackNative(idx int) {
	if idx < 0 {
		GetConvertPlayer().DisableSubtitles()
		return
	}
	if err := GetConvertPlayer().SelectSubtitleTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectSubtitleTrack(%d): %v", idx, err)
	}
}

func (s *appState) closeNativePlayer() {
	GetConvertPlayer().Close()
}

func BuildConvertPlayerPane(size fyne.Size) (fyne.CanvasObject, *ui.InlineVideoPlayer) {
	return ui.BuildInlinePlayerPane(size)
}
