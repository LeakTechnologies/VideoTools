package rip

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestContentCardRendersAndRecyclesSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(theme.DefaultTheme())
	cb := NewContentBrowser()
	cb.scanResult = &DiscScanResult{Titles: []DiscTitle{{Number: 1, Duration: 90}, {Number: 2, Duration: 60}}}
	cb.selected = map[int]bool{1: true, 2: false}
	cb.titleCards = []*titleCardState{{title: cb.scanResult.Titles[0]}, {title: cb.scanResult.Titles[1]}}
	card := cb.buildCardTemplate()
	cb.updateCard(0, card)
	markup := test.RenderObjectToMarkup(card)
	if !strings.Contains(markup, "T01") || !strings.Contains(markup, "check") {
		t.Fatalf("card children must be traversable by the canvas renderer: %s", markup)
	}
	cb.updateCard(1, card)
	check := card.(*cardBox).Container.Objects[4].(*widget.Check)
	check.SetChecked(true)
	if !cb.selected[1] || !cb.selected[2] {
		t.Fatalf("recycled card must change only its current owner: %v", cb.selected)
	}
	cb.setAllSelected(false)
	if cb.selected[1] || cb.selected[2] {
		t.Fatalf("bulk deselect must clear both rows: %v", cb.selected)
	}
}
