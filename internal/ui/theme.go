package ui

// VT_Navy colour palette re-exported from internal/theme so the ui package
// can reference colour vars by short name without an import qualifier.
// internal/theme is the single source of truth — edit colours there.

import vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"

var (
	BgBase  = vtheme.BgBase
	BgDark  = vtheme.BgDark
	BgLight = vtheme.BgLight
	BgCard  = vtheme.BgCard

	Border    = vtheme.Border
	BorderDim = vtheme.BorderDim

	Text       = vtheme.Text
	TextMuted  = vtheme.TextMuted
	TextOnDark = vtheme.TextOnDark

	InputBg = vtheme.InputBg

	Green   = vtheme.Green
	Teal    = vtheme.Teal
	Yellow  = vtheme.Yellow
	Blue    = vtheme.Blue
	Orange  = vtheme.Orange
	Purple  = vtheme.Purple
	Magenta = vtheme.Magenta

	GreenText  = vtheme.GreenText
	YellowText = vtheme.YellowText
	GrayText   = vtheme.GrayText
	DimText    = vtheme.DimText
)
