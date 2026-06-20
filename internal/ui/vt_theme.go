package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	vtheme "github.com/LeakTechnologies/VideoTools/internal/theme"
)

// VTTheme applies the VT_Navy palette to every built-in Fyne widget
// (Select, Entry, Radio, Check, Slider, etc.) by implementing fyne.Theme.
//
// Font/Icon/Size are delegated to MonoTheme. Only Color is overridden
// to map VT_Navy semantic colours to Fyne's colour names.
//
// PillButton and PillIconButton are NOT affected — they use their own
// per-module accent-colour system and should not be migrated.
type VTTheme struct {
	MonoTheme
}

func (v *VTTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	// ── Backgrounds ──────────────────────────────────────────────────────────
	case theme.ColorNameBackground:
		return vtheme.BgBase
	case theme.ColorNameInputBackground:
		return vtheme.InputBg
	case theme.ColorNameMenuBackground:
		return vtheme.BgDark
	case theme.ColorNameOverlayBackground:
		return vtheme.BgDark

	// ── Foreground / Text ─────────────────────────────────────────────────────
	case theme.ColorNameForeground:
		return vtheme.Text
	case theme.ColorNameForegroundOnPrimary:
		return vtheme.BgBase
	case theme.ColorNamePlaceHolder:
		return vtheme.TextMuted
	case theme.ColorNameDisabled:
		return vtheme.Border
	case theme.ColorNameDisabledButton:
		return vtheme.Border

	// ── Input elements ────────────────────────────────────────────────────────
	case theme.ColorNameInputBorder:
		return vtheme.Border
	case theme.ColorNameFocus:
		return vtheme.Green

	// ── Interactive states ────────────────────────────────────────────────────
	case theme.ColorNameButton:
		return vtheme.BgLight
	case theme.ColorNameHover:
		return vtheme.BgCard
	case theme.ColorNamePressed:
		return vtheme.Border
	case theme.ColorNameSelection:
		return vtheme.Green
	case theme.ColorNamePrimary:
		return vtheme.Green

	// ── Scrollbar / Separator ─────────────────────────────────────────────────
	case theme.ColorNameScrollBar:
		return vtheme.Border
	case theme.ColorNameSeparator:
		return vtheme.BorderDim

	// ── Status ────────────────────────────────────────────────────────────────
	case theme.ColorNameError:
		return vtheme.Magenta
	case theme.ColorNameSuccess:
		return vtheme.Green
	case theme.ColorNameWarning:
		return vtheme.Yellow

	default:
		return v.MonoTheme.Color(name, variant)
	}
}
