package rip

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

func chapterControls(vs *viewState, updateSummary func()) (fyne.CanvasObject, func()) {
	t := i18n.T()
	feedback := widget.NewLabel("")
	feedback.Wrapping = fyne.TextWrapWord
	titlesBox := container.NewVBox()
	scroll := container.NewVScroll(titlesBox)
	scroll.SetMinSize(fyne.NewSize(0, 180))
	box := container.NewVBox(scroll, feedback)
	var refresh func()
	mode := widget.NewSelect([]string{t.RipWholeTitle, t.RipIndividualChapters, t.RipSelectedChapters}, nil)
	var signature string
	refresh = func() {
		available := vs.extractMode != "full" && vs.format != FormatArchivist && vs.regionConvert == ""
		if !available {
			vs.chapterOnly = false
			if mode.Selected != t.RipWholeTitle {
				mode.SetSelected(t.RipWholeTitle)
			}
			mode.Disable()
		} else {
			mode.Enable()
		}
		if !vs.chapterOnly {
			box.Hide()
			return
		}
		box.Show()
		titles := vs.chapterTargets()
		key := fmt.Sprintf("%p/%t", vs.scanResult, vs.chapterIndividual)
		for _, title := range titles {
			key += fmt.Sprintf("/%d:%d", title.Number, len(title.Chapters))
		}
		if signature != key {
			signature = key
			titlesBox.Objects = nil
			if vs.chapterSelections == nil {
				vs.chapterSelections = make(map[int]string)
			}
			for _, title := range titles {
				title := title
				heading := fmt.Sprintf(t.RipTitleChapterCountFmt, title.Number, len(title.Chapters))
				if vs.chapterIndividual {
					titlesBox.Add(widget.NewLabel(heading))
					continue
				}
				options := make([]string, len(title.Chapters))
				for n := range options {
					options[n] = strconv.Itoa(n + 1)
				}
				checks := newChapterChoices(options)
				custom := ui.NewTooltipEntry(t.RipChapterSelectionHint)
				custom.SetPlaceHolder("1-3, 5, 8")
				var updating bool
				syncChecks := func(text string) {
					updating = true
					var selected []string
					if ranges, err := ParseChapterSelection(text); err == nil {
						for _, span := range ranges {
							for n := span.Start; n <= span.End && n <= len(title.Chapters); n++ {
								selected = append(selected, strconv.Itoa(n))
							}
						}
					}
					checks.SetSelected(selected)
					updating = false
				}
				text := vs.chapterSelections[title.Number]
				custom.SetText(text)
				syncChecks(text)
				checks.OnChanged = func(selected []string) {
					if updating {
						return
					}
					// Every checked chapter is its own file; ranges remain an optional shortcut.
					vs.chapterSelections[title.Number] = strings.Join(selected, ", ")
					updating = true
					custom.SetText(vs.chapterSelections[title.Number])
					updating = false
					refresh()
					updateSummary()
				}
				custom.OnChanged = func(text string) {
					if updating {
						return
					}
					vs.chapterSelections[title.Number] = text
					syncChecks(text)
					refresh()
					updateSummary()
				}
				advanced := widget.NewAccordion(widget.NewAccordionItem(t.RipCustomChapterRanges, custom))
				details := container.NewVBox(checks, advanced)
				if len(title.Chapters) == 0 {
					details = container.NewVBox(widget.NewLabel(t.RipChapterScanRequired))
				}
				item := widget.NewAccordion(widget.NewAccordionItem(heading, details))
				if len(titles) == 1 {
					item.Open(0)
				}
				titlesBox.Add(item)
			}
			if len(titles) == 0 {
				titlesBox.Add(widget.NewLabel(t.RipReadyNoSelection))
			}
			titlesBox.Refresh()
		}
		_, count, err := vs.selectedChapterRanges()
		if err != nil {
			// The empty choice is an ordinary prompt, not an alarming syntax error.
			empty := !vs.chapterIndividual
			for _, title := range titles {
				if strings.TrimSpace(vs.chapterSelections[title.Number]) != "" {
					empty = false
				}
			}
			if empty {
				feedback.SetText(t.RipChooseChapters)
				feedback.Importance = widget.MediumImportance
			} else {
				feedback.SetText(err.Error())
				feedback.Importance = widget.DangerImportance
			}
		} else {
			feedback.SetText(fmt.Sprintf(t.RipChapterFilesReadyFmt, count))
			feedback.Importance = widget.MediumImportance
		}
		feedback.Refresh()
	}
	mode.OnChanged = func(value string) {
		vs.chapterOnly = value != t.RipWholeTitle
		vs.chapterIndividual = value == t.RipIndividualChapters
		refresh()
		updateSummary()
	}
	mode.SetSelected(t.RipWholeTitle)
	refresh()
	return container.NewVBox(widget.NewLabel(t.RipChaptersLabel), mode, box), refresh
}
