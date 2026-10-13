package rip

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
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
	entry := box.Objects[0].(*widget.Entry)
	feedback := box.Objects[2].(*widget.Label)
	if box.Visible() || vs.chapterOnly {
		t.Fatal("default must be all chapters")
	}
	mode.SetSelected(i18n.T().RipSelectedChapters)
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
