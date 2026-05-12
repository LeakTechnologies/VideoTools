package rip

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"image/color"
)

const (
	FormatLosslessMKV = "Lossless MKV (Copy)"
	FormatH264MKV     = "H.264 MKV (CRF 18)"
	FormatH264MP4     = "H.264 MP4 (CRF 18)"
	FormatArchivist   = "Archivist (Reconstructible Project)"
)

// AudioStream holds minimal audio stream info for rip operations.
type AudioStream struct {
	Index    int
	Language string
}

// SubtitleStream holds minimal subtitle stream info for rip operations.
type SubtitleStream struct {
	Index    int
	Language string
}

// ProbeResult is the minimal video probe info needed by the rip executor.
type ProbeResult struct {
	Audio     []AudioStream
	Subtitles []SubtitleStream
}

// Options wires the internal rip module to the root appState.
type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	// Initial state values.
	RipSourcePath string
	RipOutputPath string
	RipFormat     string
	RipLogText    string
	RipProgress   float64

	// Widget refs set back on the caller via Set* callbacks.
	QueueBtn       *widget.Button
	RipStatusLabel *widget.Label
	RipProgressBar *widget.ProgressBar
	RipLogEntry    *widget.Entry
	RipLogScroll   *container.Scroll

	// Navigation.
	OnShowMainMenu           func()
	OnShowQueue              func()
	OnClearCompleted         func()
	OnUpdateQueueButtonLabel func()

	// State setters.
	SetRipSourcePath func(string)
	SetRipOutputPath func(string)

	// Queue.
	JobQueue func() *queue.Queue
	AddJob   func(job *queue.Job)

	// Helpers from root package.
	OnGetStatsBar    func() *ui.ConversionStatsBar
	OnModuleFooter   func(col color.Color, actions fyne.CanvasObject, stats *ui.ConversionStatsBar) fyne.CanvasObject
	OnDropFirstLocal func(items []fyne.URI) string
	OnScanDVDStruct  func(path string) error
	OnProbeVideo     func(path string) (*ProbeResult, error)

	// Widget refs written back to caller after build.
	SetQueueBtn       func(*widget.Button)
	SetRipStatusLabel func(*widget.Label)
	SetRipProgressBar func(*widget.ProgressBar)
	SetRipLogEntry    func(*widget.Entry)
	SetRipLogScroll   func(*container.Scroll)
}

// ExecuteOptions holds everything the executor needs (no UI access).
type ExecuteOptions struct {
	SourcePath string
	OutputPath string
	Format     string

	// Enrichment options — all default to false for backwards compat.
	EmbedChapters   bool   // read IFO and write chapter metadata into output
	AllAudioTracks  bool   // map every audio stream (not just the first)
	IncludeSubtitles bool  // include dvd_subtitle bitmap streams
	DiscTitle       string // embedded as MKV/MP4 title tag; empty = skip

	GetLogsDir   func() string
	LogSuffix    string
	OnProbeVideo func(path string) (*ProbeResult, error)
	OnRunCommand func(name string, args []string, logFn func(string)) error
	OnAppendLog  func(line string)
	OnSetProgress func(percent float64)
	ProgressCallback func(float64)
	OnLogFileCreated func(logPath string)
}
