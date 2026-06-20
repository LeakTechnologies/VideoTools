package rip

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
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
	QueueBtn       *ui.PillButton
	RipStatusLabel *widget.Label
	RipProgressBar *widget.ProgressBar
	RipLogEntry    *widget.Label
	RipLogScroll   *container.Scroll

	// Navigation.
	OnShowMainMenu           func()
	OnShowQueue              func()
	OnClearCompleted         func()
	OnUpdateQueueButtonLabel func()
	OnOpenInPlayer           func(path string) // open the loaded disc in the VT DVD player

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
	SetQueueBtn       func(*ui.PillButton)
	SetRipStatusLabel func(*widget.Label)
	SetRipProgressBar func(*widget.ProgressBar)
	SetRipLogEntry    func(*widget.Label)
	SetRipLogScroll   func(*container.Scroll)
}

// DiscTitleTrack describes one audio or subtitle stream on a disc title.
type DiscTitleTrack struct {
	Language string
	Codec    string
	Channels int // 0 for subtitle tracks
}

// DiscTitle describes one title entry from the DVD's TT_SRPT.
type DiscTitle struct {
	Number      int // 1-based title number on the disc
	VTSNumber   int
	NumChapters int
	Duration    float64
	Audio       []DiscTitleTrack
	Subtitles   []DiscTitleTrack
	HasAngles   bool
}

// DiscScanResult holds the outcome of scanning a disc source directory.
type DiscScanResult struct {
	DiscType  string // "DVD-5", "DVD-9", "DVD-10", "BD-25", "BD-50", or ""
	TotalSize int64  // total size of all files in VIDEO_TS in bytes
	Region    string // e.g. "Region 1", "Region Free", or ""
	Titles    []DiscTitle
}

// ExecuteOptions holds everything the executor needs (no UI access).
type ExecuteOptions struct {
	SourcePath string
	OutputPath string
	Format     string

	// VTSNumber selects a specific VTS to rip. 0 = largest set (default = main feature).
	VTSNumber int

	// TitleNumber is the 1-based title index from VMG TT_SRPT.
	// Used with -f dvdvideo for seamless branching support. 0 = auto-detect.
	TitleNumber int

	// ExtractMode controls whether only the main feature (default) or the full disc
	// (all VTS sets + menu VOB) is extracted. "" or "main" = main feature only.
	// "full" = full disc extraction with IFO regeneration (DVD-Video output).
	ExtractMode string

	// Enrichment options — all default to false/"" for backwards compat.
	EmbedChapters    bool   // read IFO and write chapter metadata into output
	AllAudioTracks   bool   // map every audio stream (not just the first)
	IncludeSubtitles bool   // include dvd_subtitle bitmap streams (MKV only)
	IncludeMenus     bool   // export menu VOBs as separate files (default false = skip menus)
	DiscTitle        string // embedded as MKV/MP4 title tag; empty = skip
	RegionConvert    string // "" (none), "pal2ntsc", "ntsc2pal"

	GetLogsDir   func() string
	LogSuffix    string
	OnProbeVideo func(path string) (*ProbeResult, error)
	OnRunCommand func(name string, args []string, logFn func(string)) error
	OnAppendLog  func(line string)
	OnSetProgress func(percent float64)
	OnSetStatus  func(string)
	ProgressCallback func(float64)
	OnLogFileCreated func(logPath string)
}
