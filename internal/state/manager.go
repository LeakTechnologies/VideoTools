package state

import (
	"fyne.io/fyne/v2/widget"
	"sync"
)

type StateManager struct {
	mu sync.RWMutex

	// Current mode settings
	crfMode         CRFMode
	vbrMode         VBRMode
	currentQuality  string
	currentBitrate  string
	currentCRFValue int64
	currentVBRValue int64

	// Registered widgets for synchronization
	qualityWidgets []*widget.Select
	bitrateWidgets []*widget.Select
}

type CRFMode string
type VBRMode string

const (
	CRFManual      CRFMode = "manual"
	CRFQuality     CRFMode = "quality"
	CRFBitrate     CRFMode = "bitrate"
	VBRStandard    VBRMode = "standard"
	VBRHQ          VBRMode = "hq"
	VBRConstrained VBRMode = "constrained"
)
