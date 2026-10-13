package rip

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
)

func chapterControls(vs *viewState, updateSummary func()) (fyne.CanvasObject, func()) {
	t := i18n.T()
	entry := widget.NewEntry()
	entry.SetPlaceHolder("1-3, 5-6, 8")
	help := widget.NewLabel(t.RipChapterSelectionHint)
	help.Wrapping = fyne.TextWrapWord
	feedback := widget.NewLabel("")
	feedback.Wrapping = fyne.TextWrapWord
	box := container.NewVBox(entry, help, feedback)
	var refresh func()
	mode := widget.NewSelect([]string{t.RipAllChapters, t.RipSelectedChapters}, nil)
	refresh = func() {
		available := vs.extractMode != "full" && vs.format != FormatArchivist && vs.regionConvert == ""
		if !available {
			vs.chapterOnly = false
			if mode.Selected != t.RipAllChapters {
				mode.SetSelected(t.RipAllChapters)
			}
			mode.Disable()
		} else {
			mode.Enable()
		}
		if vs.chapterOnly {
			box.Show()
			_, count, err := vs.selectedChapterRanges()
			if err != nil {
				feedback.SetText(err.Error())
				feedback.Importance = widget.DangerImportance
			} else {
				feedback.SetText(fmt.Sprintf(t.RipChapterFilesReadyFmt, count))
				feedback.Importance = widget.MediumImportance
			}
			feedback.Refresh()
		} else {
			box.Hide()
		}
	}
	mode.OnChanged = func(value string) {
		vs.chapterOnly = value == t.RipSelectedChapters
		refresh()
		updateSummary()
	}
	entry.OnChanged = func(text string) {
		vs.chapterSelection = text
		refresh()
		updateSummary()
	}
	entry.SetText(vs.chapterSelection)
	mode.SetSelected(t.RipAllChapters)
	refresh()
	return container.NewVBox(widget.NewLabel(t.RipChaptersLabel), mode, box), refresh
}
