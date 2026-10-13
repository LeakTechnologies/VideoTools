package rip

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

func TestChapterControlsShowValidationAndWholeDiscGuard(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	vs := &viewState{format: FormatH264MKV, extractMode: "main", scanResult: &DiscScanResult{Titles: []DiscTitle{{Number: 1, Duration: 100, Chapters: make([]float64, 8)}}}}
	updates := 0
	content, refresh := chapterControls(vs, func() { updates++ })
	root := content.(*fyne.Container)
	mode := root.Objects[1].(*widget.Select)
	box := root.Objects[2].(*fyne.Container)
	feedback := box.Objects[1].(*widget.Label)
	if box.Visible() || vs.chapterOnly {
		t.Fatal("default must be all chapters")
	}
	mode.SetSelected(i18n.T().RipIndividualChapters)
	if feedback.Text != fmt.Sprintf(i18n.T().RipChapterFilesReadyFmt, 8) {
		t.Fatal("individual chapters not counted")
	}
	mode.SetSelected(i18n.T().RipSelectedChapters)
	scroll := box.Objects[0].(*container.Scroll)
	titles := scroll.Content.(*fyne.Container)
	accordion := titles.Objects[0].(*widget.Accordion)
	details := accordion.Items[0].Detail.(*fyne.Container)
	checks := details.Objects[0].(*chapterChoices)
	if len(checks.Options) != 8 {
		t.Fatal("available chapters not visible")
	}
	checks.SetSelected([]string{"1", "8"})
	if feedback.Text != fmt.Sprintf(i18n.T().RipChapterFilesReadyFmt, 2) {
		t.Fatal("checked chapters not separate files")
	}
	custom := details.Objects[1].(*widget.Accordion)
	entry := custom.Items[0].Detail.(*ui.TooltipEntry)
	entry.SetText("1-3, 5-6, 8")
	if !box.Visible() || !vs.chapterOnly || feedback.Text != fmt.Sprintf(i18n.T().RipChapterFilesReadyFmt, 3) {
		t.Fatalf("selected controls: %q", feedback.Text)
	}
	entry.SetText("1-9")
	if feedback.Importance != widget.DangerImportance {
		t.Fatal("unavailable chapters must show an error")
	}
	vs.extractMode = "full"
	refresh()
	if box.Visible() || vs.chapterOnly || !mode.Disabled() {
		t.Fatal("full disc must disable selected chapters")
	}
	vs.extractMode = "main"
	refresh()
	if mode.Disabled() || vs.chapterOnly {
		t.Fatal("returning to title mode must default safely to all")
	}
	if updates == 0 {
		t.Fatal("control changes must update readiness")
	}
}
