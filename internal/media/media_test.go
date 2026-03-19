//go:build native_media

package media

import (
	"testing"
	"time"
)

func TestPacketQueueBasic(t *testing.T) {
	q := NewPacketQueue()
	if q == nil {
		t.Fatal("NewPacketQueue returned nil")
	}

	if q.Size() != 0 {
		t.Errorf("expected size 0, got %d", q.Size())
	}

	if q.MaxSize() != DefaultMaxQueueSize {
		t.Errorf("expected max size %d, got %d", DefaultMaxQueueSize, q.MaxSize())
	}
}

func TestPacketQueueSetMaxSize(t *testing.T) {
	q := NewPacketQueue()
	q.SetMaxSize(100)

	if q.MaxSize() != 100 {
		t.Errorf("expected max size 100, got %d", q.MaxSize())
	}
}

func TestPacketQueueWithCustomMaxSize(t *testing.T) {
	q := NewPacketQueueWithMaxSize(50)
	if q == nil {
		t.Fatal("NewPacketQueueWithMaxSize returned nil")
	}

	if q.MaxSize() != 50 {
		t.Errorf("expected max size 50, got %d", q.MaxSize())
	}
}

func TestPacketQueueWithZeroMaxSize(t *testing.T) {
	q := NewPacketQueueWithMaxSize(0)
	if q.MaxSize() != DefaultMaxQueueSize {
		t.Errorf("expected max size %d (default), got %d", DefaultMaxQueueSize, q.MaxSize())
	}
}

func TestPacketQueueIsClosed(t *testing.T) {
	q := NewPacketQueue()

	if q.IsClosed() {
		t.Error("expected queue to not be closed initially")
	}

	q.Close()

	if !q.IsClosed() {
		t.Error("expected queue to be closed")
	}
}

func TestPacketQueueIsFull(t *testing.T) {
	q := NewPacketQueueWithMaxSize(5)

	if q.IsFull() {
		t.Error("expected queue to not be full initially")
	}
}

func TestMasterClockBasic(t *testing.T) {
	clock := NewMasterClock()
	if clock == nil {
		t.Fatal("NewMasterClock returned nil")
	}

	time.Sleep(10 * time.Millisecond)

	currentTime := clock.GetTime()
	if currentTime < 0 {
		t.Errorf("expected time >= 0, got %f", currentTime)
	}
}

func TestMasterClockSetTime(t *testing.T) {
	clock := NewMasterClock()

	clock.SetTime(5.0)
	if clock.GetTime() != 5.0 {
		t.Errorf("expected time 5.0, got %f", clock.GetTime())
	}
}

func TestMasterClockPauseResume(t *testing.T) {
	clock := NewMasterClock()

	clock.SetTime(5.0)
	clock.SetPaused(true)

	time.Sleep(10 * time.Millisecond)

	if clock.GetTime() != 5.0 {
		t.Errorf("expected time 5.0 while paused, got %f", clock.GetTime())
	}

	clock.SetPaused(false)

	time.Sleep(10 * time.Millisecond)

	if clock.GetTime() <= 5.0 {
		t.Errorf("expected time > 5.0 after resume, got %f", clock.GetTime())
	}
}

func TestMasterClockSyncVideo(t *testing.T) {
	clock := NewMasterClock()
	clock.SetTime(0)

	delay := clock.SyncVideo(0.1)
	if delay < 0 {
		t.Errorf("expected non-negative delay for pts >= master, got %v", delay)
	}

	delay = clock.SyncVideo(0)
	if delay != 0 {
		t.Errorf("expected zero delay for pts <= master, got %v", delay)
	}
}

func TestSeekAccuracy(t *testing.T) {
	acc := SeekAccuracyFrame
	if acc != SeekAccuracyFrame {
		t.Errorf("expected SeekAccuracyFrame, got %v", acc)
	}

	acc = SeekAccuracyKeyframe
	if acc != SeekAccuracyKeyframe {
		t.Errorf("expected SeekAccuracyKeyframe, got %v", acc)
	}

	acc = SeekAccuracyAccurate
	if acc != SeekAccuracyAccurate {
		t.Errorf("expected SeekAccuracyAccurate, got %v", acc)
	}
}

func TestVideoInfo(t *testing.T) {
	info := &VideoInfo{
		Width:     1920,
		Height:    1080,
		FrameRate: 30.0,
		Duration:  120.0,
		CodecName: "h264",
	}

	if info.Width != 1920 {
		t.Errorf("expected width 1920, got %d", info.Width)
	}

	if info.Height != 1080 {
		t.Errorf("expected height 1080, got %d", info.Height)
	}

	if info.FrameRate != 30.0 {
		t.Errorf("expected frame rate 30.0, got %f", info.FrameRate)
	}
}

func TestSubtitleTrack(t *testing.T) {
	track := SubtitleTrack{
		Index:     0,
		Language:  "eng",
		CodecName: "ass",
		Title:     "English",
		IsForced:  false,
		IsDefault: true,
	}

	if track.Language != "eng" {
		t.Errorf("expected language 'eng', got '%s'", track.Language)
	}

	if track.IsDefault != true {
		t.Error("expected IsDefault to be true")
	}
}

func TestSubtitle(t *testing.T) {
	sub := Subtitle{
		Index:     0,
		StartTime: 1000 * time.Millisecond,
		EndTime:   2000 * time.Millisecond,
		Text:      "Hello World",
		Format:    SubtitleTypeText,
	}

	if sub.Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got '%s'", sub.Text)
	}

	if sub.Format != SubtitleTypeText {
		t.Errorf("expected format SubtitleTypeText, got %v", sub.Format)
	}
}

func TestFormatSRTTime(t *testing.T) {
	d := 1*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond
	result := formatSRTTime(d)
	expected := "01:02:03,456"

	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestFormatASSTime(t *testing.T) {
	d := 1*time.Hour + 2*time.Minute + 3*time.Second + 456*time.Millisecond
	result := formatASSTime(d)
	expected := "1:02:03.45"

	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestEscapeASSText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "Hello"},
		{"Hello\nWorld", "Hello\\NWorld"},
		{"{bold}", "\\{bold}"},
		{"\\backslash", "\\\\backslash"},
	}

	for _, tc := range tests {
		result := escapeASSText(tc.input)
		if result != tc.expected {
			t.Errorf("escapeASSText(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestHWDeviceType(t *testing.T) {
	tests := []struct {
		hwType HWDeviceType
		name   string
	}{
		{HWDeviceNone, "HWDeviceNone"},
		{HWDeviceVAAPI, "HWDeviceVAAPI"},
		{HWDeviceD3D11VA, "HWDeviceD3D11VA"},
		{HWDeviceQSV, "HWDeviceQSV"},
	}

	for _, tc := range tests {
		if tc.hwType < 0 || tc.hwType > HWDeviceQSV {
			t.Errorf("HWDeviceType %s has invalid value: %d", tc.name, tc.hwType)
		}
	}
}

func TestDetectHWDevice(t *testing.T) {
	hwType := DetectHWDevice()

	if hwType < HWDeviceNone || hwType > HWDeviceQSV {
		t.Errorf("DetectHWDevice returned invalid value: %d", hwType)
	}
}

func TestEngineHWDevice(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.GetHWDevice() != HWDeviceNone {
		t.Error("expected default HWDevice to be HWDeviceNone")
	}

	engine.SetHWDevice(HWDeviceVAAPI)
	if engine.GetHWDevice() != HWDeviceVAAPI {
		t.Errorf("expected HWDeviceVAAPI, got %v", engine.GetHWDevice())
	}

	engine.SetHWDevice(HWDeviceD3D11VA)
	if engine.GetHWDevice() != HWDeviceD3D11VA {
		t.Errorf("expected HWDeviceD3D11VA, got %v", engine.GetHWDevice())
	}
}

func TestVideoInfoHWDevice(t *testing.T) {
	info := &VideoInfo{
		Width:    1920,
		Height:   1080,
		HWDevice: HWDeviceVAAPI,
	}

	if info.HWDevice != HWDeviceVAAPI {
		t.Errorf("expected HWDeviceVAAPI, got %v", info.HWDevice)
	}
}

func TestStreamInfo(t *testing.T) {
	info := StreamInfo{
		Index:     0,
		CodecName: "aac",
		Language:  "eng",
		Title:     "English",
	}

	if info.Index != 0 {
		t.Errorf("expected Index 0, got %d", info.Index)
	}
	if info.CodecName != "aac" {
		t.Errorf("expected CodecName 'aac', got %s", info.CodecName)
	}
	if info.Language != "eng" {
		t.Errorf("expected Language 'eng', got %s", info.Language)
	}
}

func TestVideoInfoStreamTracks(t *testing.T) {
	info := &VideoInfo{
		Width:  1920,
		Height: 1080,
		AudioTracks: []StreamInfo{
			{Index: 0, CodecName: "aac", Language: "eng"},
			{Index: 1, CodecName: "ac3", Language: "fra"},
		},
		SubtitleTracks: []StreamInfo{
			{Index: 2, CodecName: "subrip", Language: "eng"},
		},
	}

	if len(info.AudioTracks) != 2 {
		t.Errorf("expected 2 audio tracks, got %d", len(info.AudioTracks))
	}
	if len(info.SubtitleTracks) != 1 {
		t.Errorf("expected 1 subtitle track, got %d", len(info.SubtitleTracks))
	}
}

func TestEngineLooping(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.IsLooping() {
		t.Error("expected default looping to be false")
	}

	engine.SetLooping(true)
	if !engine.IsLooping() {
		t.Error("expected looping to be true after SetLooping(true)")
	}

	engine.SetLooping(false)
	if engine.IsLooping() {
		t.Error("expected looping to be false after SetLooping(false)")
	}
}

func TestPacketQueueEOF(t *testing.T) {
	q := NewPacketQueue()
	if q == nil {
		t.Fatal("NewPacketQueue returned nil")
	}

	if q.IsEOF() {
		t.Error("expected EOF to be false initially")
	}

	q.SetEOF()
	if !q.IsEOF() {
		t.Error("expected EOF to be true after SetEOF()")
	}

	q.Flush()
	if q.IsEOF() {
		t.Error("expected EOF to be false after Flush()")
	}
}

func TestPacketQueueGetAfterEOF(t *testing.T) {
	q := NewPacketQueueWithMaxSize(10)

	q.SetEOF()

	_, ok := q.Get()
	if ok {
		t.Error("expected Get() to return false after EOF with empty queue")
	}

	q.Close()
}

func TestMasterClockSpeed(t *testing.T) {
	clock := NewMasterClock()

	clock.SetSpeed(2.0)
	if clock.GetSpeed() != 2.0 {
		t.Errorf("expected speed 2.0, got %f", clock.GetSpeed())
	}

	clock.SetSpeed(0.5)
	if clock.GetSpeed() != 0.5 {
		t.Errorf("expected speed 0.5, got %f", clock.GetSpeed())
	}
}

func TestMasterClockWaitForPTS(t *testing.T) {
	clock := NewMasterClock()

	clock.SetTime(1.0)

	clock.WaitForPTS(0.5)
}

func TestEngineHasAudio(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	info := engine.Info()
	if info != nil {
		t.Error("expected nil info before Open()")
	}
}

func TestEngineSelectAudioTrackWithoutOpen(t *testing.T) {
	engine := NewEngine()

	err := engine.SelectAudioTrack(0)
	if err == nil {
		t.Error("expected error when selecting track without opening file")
	}
}

func TestEngineSelectSubtitleTrackWithoutOpen(t *testing.T) {
	engine := NewEngine()

	err := engine.SelectSubtitleTrack(0)
	if err == nil {
		t.Error("expected error when selecting track without opening file")
	}
}

func TestEngineSelectVideoTrackWithoutOpen(t *testing.T) {
	engine := NewEngine()

	err := engine.SelectVideoTrack(0)
	if err == nil {
		t.Error("expected error when selecting video track without opening file")
	}
}

func TestVideoInfoVideoTracks(t *testing.T) {
	info := &VideoInfo{
		Width:  1920,
		Height: 1080,
		VideoTracks: []StreamInfo{
			{Index: 0, CodecName: "h264", Language: "eng"},
			{Index: 1, CodecName: "hevc", Language: "eng"},
		},
	}

	if len(info.VideoTracks) != 2 {
		t.Errorf("expected 2 video tracks, got %d", len(info.VideoTracks))
	}
}

func TestSubtitleOverlayBounds(t *testing.T) {
	sub := &SubtitleOverlay{
		X:      10,
		Y:      20,
		Width:  100,
		Height: 50,
	}

	bounds := sub.Bounds()
	if bounds.Min.X != 10 || bounds.Min.Y != 20 {
		t.Errorf("unexpected min: %v", bounds.Min)
	}
	if bounds.Max.X != 110 || bounds.Max.Y != 70 {
		t.Errorf("unexpected max: %v", bounds.Max)
	}
}

func TestEngineNumThreads(t *testing.T) {
	engine := NewEngine()

	if engine.GetNumThreads() != 0 {
		t.Errorf("expected default threads 0, got %d", engine.GetNumThreads())
	}

	engine.SetNumThreads(4)
	if engine.GetNumThreads() != 4 {
		t.Errorf("expected threads 4, got %d", engine.GetNumThreads())
	}

	engine.SetNumThreads(-1)
	if engine.GetNumThreads() != 0 {
		t.Errorf("expected threads 0 for negative input, got %d", engine.GetNumThreads())
	}
}

func TestEngineFramePool(t *testing.T) {
	engine := NewEngine()

	if engine.GetFramePoolSize() != 0 {
		t.Errorf("expected empty pool, got %d", engine.GetFramePoolSize())
	}

	engine.ReleaseFrame(nil)
	if engine.GetFramePoolSize() != 0 {
		t.Error("nil frame should not add to pool")
	}
}
