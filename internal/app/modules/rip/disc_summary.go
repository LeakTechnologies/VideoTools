package rip

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

// DiscSummary is the disc identity card shown directly under Source. Unlike
// the old single-line disc-info label, it is a real card with distinct fields
// (identity, technical info, main feature) and explicit scan states
// (empty / scanning / ok / error), each of which updates independently as the
// scan progresses. All Set* methods must be called from the UI thread.
type DiscSummary struct {
	outer *fyne.Container // rounded navy box (header + body)

	titleLbl  *widget.Label
	statusLbl *widget.Label
	techLbl   *widget.Label
	mainLbl   *widget.Label
}

// NewDiscSummary builds an empty disc summary card in the "no disc" state.
func NewDiscSummary() *DiscSummary {
	d := &DiscSummary{}

	d.titleLbl = widget.NewLabel("")
	d.titleLbl.TextStyle = fyne.TextStyle{Bold: true}
	d.titleLbl.Wrapping = fyne.TextTruncate

	d.statusLbl = widget.NewLabel("")
	d.statusLbl.Importance = widget.MediumImportance
	d.statusLbl.Wrapping = fyne.TextWrapWord

	d.techLbl = widget.NewLabel("")
	d.techLbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	d.mainLbl = widget.NewLabel("")
	d.mainLbl.Importance = widget.LowImportance
	d.mainLbl.Wrapping = fyne.TextWrapWord

	body := container.NewVBox(
		d.titleLbl,
		d.statusLbl,
		layout.NewSpacer(),
		d.techLbl,
		d.mainLbl,
	)

	d.outer = sectionBox(i18n.T().RipDiscSection, body)
	d.SetEmpty()
	return d
}

// GetContainer returns the root canvas object for embedding in the view.
func (d *DiscSummary) GetContainer() fyne.CanvasObject { return d.outer }

// SetEmpty resets the card to the "no disc loaded" state.
func (d *DiscSummary) SetEmpty() {
	d.titleLbl.SetText(i18n.T().RipDiscNone)
	d.statusLbl.SetText(i18n.T().RipDiscNoneHint)
	d.statusLbl.Importance = widget.MediumImportance
	d.techLbl.SetText("")
	d.mainLbl.SetText("")
}

// SetScanning shows the "reading disc information" state while a scan runs.
func (d *DiscSummary) SetScanning() {
	d.titleLbl.SetText(i18n.T().RipDiscScanning)
	d.statusLbl.SetText(i18n.T().RipDiscScanningHint)
	d.statusLbl.Importance = widget.MediumImportance
	d.techLbl.SetText("")
	d.mainLbl.SetText("")
}

// SetResult fills the card with complete scan data. discTitle is the resolved
// disc / title name (or "" to fall back to a generic label).
func (d *DiscSummary) SetResult(res *DiscScanResult, discTitle string) {
	if discTitle == "" {
		discTitle = i18n.T().RipDiscSection
	}
	d.titleLbl.SetText(discTitle)
	d.statusLbl.SetText(i18n.T().RipDiscScanned)
	d.statusLbl.Importance = widget.SuccessImportance

	tech := make([]string, 0, 5)
	if res != nil {
		if res.DiscType != "" {
			tech = append(tech, res.DiscType)
		}
		if res.Region != "" {
			tech = append(tech, res.Region)
		}
		if res.VideoStandard != "" {
			tech = append(tech, res.VideoStandard)
		}
		if res.TotalSize > 0 {
			tech = append(tech, fmt.Sprintf("%.2f GB", float64(res.TotalSize)/1e9))
		}
		if len(res.Titles) > 0 {
			tech = append(tech, fmt.Sprintf("%d %s", len(res.Titles), i18n.T().RipTitleCount))
		}
	}
	d.techLbl.SetText(strings.Join(tech, "      "))

	// Main feature = the longest title.
	mainNum := 0
	mainDur := 0.0
	if res != nil {
		for _, dt := range res.Titles {
			if dt.Duration > mainDur {
				mainDur = dt.Duration
				mainNum = dt.Number
			}
		}
	}
	if mainNum > 0 {
		d.mainLbl.SetText(fmt.Sprintf("%s    %s %02d    %s",
			strings.ToUpper(i18n.T().RipMainFeature), i18n.T().RipTitleShort, mainNum, FormatDuration(mainDur)))
	} else {
		d.mainLbl.SetText("")
	}
}

// SetError shows a scan failure state with a short human-readable message.
func (d *DiscSummary) SetError(msg string) {
	d.titleLbl.SetText(i18n.T().RipDiscScanFailed)
	d.statusLbl.SetText(msg)
	d.statusLbl.Importance = widget.WarningImportance
	d.techLbl.SetText("")
	d.mainLbl.SetText("")
}

// sectionBox wraps content in the app's standard rounded navy box with a teal
// header bar, matching the other rip sections. Extracted here so both the disc
// summary and the rest of the view share one implementation.
func sectionBox(title string, content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(ripNavy)
	bg.CornerRadius = 10
	bg.StrokeColor = ui.GridColor
	bg.StrokeWidth = 1

	headerBg := canvas.NewRectangle(ripTeal)
	headerBg.CornerRadius = 10
	headerBg.SetMinSize(fyne.NewSize(0, 34))
	headerTitle := canvas.NewText(title, color.White)
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}
	headerTitle.TextSize = 12
	header := container.NewMax(
		headerBg,
		container.NewPadded(container.NewHBox(headerTitle, layout.NewSpacer())),
	)
	body := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))
	return container.NewMax(bg, body)
}
