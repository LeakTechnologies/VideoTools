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

func TestContentCardOutputNamesFollowRecycledOwner(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(theme.DefaultTheme())
	cb := NewContentBrowser()
	cb.scanResult = &DiscScanResult{Titles: []DiscTitle{{Number: 1}, {Number: 2}}}
	cb.selected = map[int]bool{1: true, 2: true}
	cb.titleCards = []*titleCardState{{title: cb.scanResult.Titles[0]}, {title: cb.scanResult.Titles[1]}}
	var edits []int
	cb.SetOnOutputName(func(number int, name string) {
		// A callback may consult browser state; it must run outside cb.mu.
		cb.GetSelected()
		edits = append(edits, number)
	})
	card := cb.buildCardTemplate().(*cardBox)
	cb.updateCard(0, card)
	if card.nameEntry.Text != "Title_01" || len(edits) != 0 {
		t.Fatalf("default name must render without an edit: %q, %v", card.nameEntry.Text, edits)
	}
	card.nameEntry.SetText("Opening")
	cb.updateCard(1, card)
	if card.nameEntry.Text != "Title_02" || len(edits) != 1 {
		t.Fatalf("rebind must not edit the old owner: %q, %v", card.nameEntry.Text, edits)
	}
	card.nameEntry.SetText("Extras")
	cb.updateCard(0, card)
	if card.nameEntry.Text != "Opening" || cb.outputNames[2] != "Extras" || len(edits) != 2 || edits[1] != 2 {
		t.Fatalf("names must survive recycling independently: %v, %v", cb.outputNames, edits)
	}
	cb.SetOutputNames(map[int]string{1: "Restored", 2: "Bonus"})
	cb.updateCard(1, card)
	if card.nameEntry.Text != "Bonus" || len(edits) != 2 {
		t.Fatalf("programmatic restore must not emit an edit: %q, %v", card.nameEntry.Text, edits)
	}
	card.nameEntry.SetText("")
	cb.updateCard(0, card)
	cb.updateCard(1, card)
	if card.nameEntry.Text != "" || cb.outputNames[1] != "Restored" {
		t.Fatalf("empty edit must remain on its current owner: %v", cb.outputNames)
	}
}
