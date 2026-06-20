package theme

import (
	"image/color"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// VT_Navy colour palette — single source of truth for the app theme.
// All packages should reference these vars instead of hardcoding hex values.
var (
	// Core backgrounds (darkest → lightest)
	BgBase  = utils.MustHex("#0B0F1A") // deepest background (main.go backgroundColor)
	BgDark  = utils.MustHex("#0F1529") // VT_Navy — card/panel dark
	BgLight = utils.MustHex("#1a1f35") // navylight — elevated surface
	BgCard  = utils.MustHex("#252a42") // card background

	// Borders
	Border     = utils.MustHex("#2d3456") // standard border (roadmap vt-border)
	BorderDim  = utils.MustHex("#171C2A") // subtle border (main.go gridColor)

	// Text
	Text       = utils.MustHex("#E1EEFF") // primary text (main.go textColor)
	TextMuted  = utils.MustHex("#94a3b8") // muted/secondary text
	TextOnDark = color.White              // white text on coloured backgrounds

	// Input / hover surfaces
	InputBg = utils.MustHex("#344256") // input background, focus, border

	// Accent / status colours
	Green    = utils.MustHex("#22c55e") // shipped / complete
	Teal     = utils.MustHex("#14b8a6") // done (untested)
	Yellow   = utils.MustHex("#eab308") // active / in progress
	Blue     = utils.MustHex("#3b82f6") // planned
	Orange   = utils.MustHex("#f97316") // future
	Red      = utils.MustHex("#ef4444") // destructive / danger
	Purple   = utils.MustHex("#a855f7") // deprecated
	Magenta  = utils.MustHex("#5961FF") // queue accent (main.go queueColor)

	// Status text colours (for RGB strings in conversion stats etc.)
	GreenText  = color.NRGBA{R: 100, G: 220, B: 100, A: 255} // running
	YellowText = color.NRGBA{R: 255, G: 200, B: 100, A: 255} // pending
	GrayText   = color.NRGBA{R: 150, G: 150, B: 150, A: 255} // completed/failed
	DimText    = color.NRGBA{R: 100, G: 100, B: 100, A: 255} // no active jobs

	// App-scope colour references used across packages
	GridColor color.Color = BorderDim
	TextColor color.Color = Text
)

// SetColors overrides the shared GridColor and TextColor.
// Called once at startup from main.go.
func SetColors(grid, text color.Color) {
	GridColor = grid
	TextColor = text
}
