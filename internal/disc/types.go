// Package disc provides a UI-free DVD/ISO inspection contract for the
// VideoTools host and headless service.
package disc

import "fmt"

// SourceKind identifies how a disc source was supplied.
type SourceKind string

const (
	SourceFolder  SourceKind = "dvd_folder"
	SourceVideoTS SourceKind = "video_ts"
	SourceISO     SourceKind = "iso"
	SourceFile    SourceKind = "file"
)

// Track describes one audio or subtitle stream on a title.
type Track struct {
	Index    int    `json:"index"`
	Language string `json:"language,omitempty"`
	Codec    string `json:"codec,omitempty"`
	Channels int    `json:"channels,omitempty"`
}

// Cell describes one title cell in the global VTS title stream.
type Cell struct {
	VOBID       uint8  `json:"vobId"`
	CellID      uint8  `json:"cellId"`
	FirstSector uint32 `json:"firstSector"`
	LastSector  uint32 `json:"lastSector"`
}

// Title describes one DVD title-domain title.
type Title struct {
	Number      int       `json:"number"`
	VTSNumber   int       `json:"vtsNumber"`
	TTN         int       `json:"ttn"`
	NumChapters int       `json:"numChapters"`
	Duration    float64   `json:"duration"`
	Chapters    []float64 `json:"chapters,omitempty"`
	Audio       []Track   `json:"audio,omitempty"`
	Subtitles   []Track   `json:"subtitles,omitempty"`
	HasAngles   bool      `json:"hasAngles"`
	Interlaced  bool      `json:"interlaced"`
	Cells       []Cell    `json:"cells,omitempty"`
}

// ScanResult is the stable inspection result returned by the service.
type ScanResult struct {
	SourceKind      SourceKind `json:"sourceKind"`
	SourcePath      string     `json:"sourcePath"`
	DiscType        string     `json:"discType,omitempty"`
	TotalSize       int64      `json:"totalSize"`
	Region          string     `json:"region,omitempty"`
	RegionCode      int        `json:"regionCode"`
	VideoStandard   string     `json:"videoStandard,omitempty"`
	MainTitleNumber int        `json:"mainTitleNumber,omitempty"`
	Warnings        []string   `json:"warnings,omitempty"`
	Titles          []Title    `json:"titles"`
}

// ScanOptions controls optional scan progress notes.
type ScanOptions struct {
	// OnNote receives human-readable progress facts. It is optional and must
	// not perform UI work when called from the headless service.
	OnNote func(string)
}

func (o ScanOptions) note(text string) {
	if o.OnNote != nil && text != "" {
		o.OnNote(text)
	}
}

// Error is a machine-readable disc-service error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func serviceError(code, message string, err error) error {
	return &Error{Code: code, Message: message, Err: err}
}

func partialScanError(code, message string) error {
	return &Error{Code: code, Message: message}
}
