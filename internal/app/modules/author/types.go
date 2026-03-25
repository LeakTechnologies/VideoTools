package author

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	OnStopPreview    func()
	OnShowMainMenu   func()
	OnShowQueue      func()
	OnAddToQueue     func(startNow bool)
	OnClearCompleted func()
	OnCancelJob      func()
	OnUpdateSummary  func()

	GetAuthorState func() *AuthorState
	SetAuthorState func(s *AuthorState)

	OnAddFiles                 func(paths []string)
	OnLoadEmbeddedChapters     func(path string)
	OnShowTrackSelectionDialog func(idx int, refresh func())

	OnShowSubtitlesView  func()
	OnShowChapterPreview func(path string, chapters []AuthorChapter, callback func(bool, []AuthorChapter))

	StatsBar fyne.CanvasObject
	QueueBtn *widget.Button

	AuthorTabs *container.AppTabs
}

type AuthorState struct {
	OutputType  string
	Region      string
	AspectRatio string
	DiscSize    string
	Title       string

	CreateMenu              bool
	MenuTemplate            string
	MenuTheme               string
	MenuBackgroundImage     string
	MenuMotionBackground    string
	MenuCustomBgColor       string
	MenuCustomTextColor     string
	MenuCustomAccentColor   string
	MenuTitleLogoEnabled    bool
	MenuTitleLogoPath       string
	MenuTitleLogoPosition   string
	MenuTitleLogoScale      float64
	MenuTitleLogoMargin     int
	MenuStudioLogoEnabled   bool
	MenuStudioLogoPath      string
	MenuStudioLogoPosition  string
	MenuStudioLogoScale     float64
	MenuStudioLogoMargin    int
	MenuStructure           string
	MenuExtrasEnabled       bool
	MenuChapterThumbnailSrc string
	TreatAsChapters         bool
	SceneThreshold          float64

	VideoTSPath   string
	Clips         []AuthorClip
	Chapters      []AuthorChapter
	ChapterSource string
	File          *VideoSource
	Subtitles     []string

	SummaryLabel  *widget.Label
	DiscFillBar   *widget.ProgressBar
	DiscFillLabel *widget.Label
	StatusLabel   *widget.Label
	ProgressBar   *widget.ProgressBar
	LogEntry      *widget.Label
	LogScroll     *fyne.Container
	CancelBtn     *widget.Button

	ChaptersRefresh func()

	Progress    float64
	LogText     string
	LogLines    []string
	LogFilePath string

	Detecting bool
}

type AuthorClip struct {
	Path           string
	DisplayName    string
	Duration       float64
	ChapterTitle   string
	IsExtra        bool
	AudioTracks    []AuthorAudioTrack
	SubtitleTracks []AuthorSubtitleTrack
}

type AuthorAudioTrack struct {
	Index        int
	Language     string
	Codec        string
	Channels     int
	Label        string
	ExternalPath string
}

type AuthorSubtitleTrack struct {
	Index        int
	Language     string
	Codec        string
	Label        string
	ExternalPath string
}

type AuthorChapter struct {
	Timestamp float64
	Title     string
	Auto      bool
}

type VideoSource struct {
	Path      string
	Duration  float64
	Width     int
	Height    int
	FrameRate float64
}

type AuthorCallbacks interface {
	Window() fyne.Window
	ShowMainMenu()
	ShowQueue()
	ShowSubtitlesView()
	ShowChapterPreview(path string, chapters []AuthorChapter, callback func(bool, []AuthorChapter))
	ShowAuthorView()
	AddToQueueAuthor(startNow bool)
	ClearCompletedJobs()
	CancelAuthorJob()
}
