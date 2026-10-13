package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestHelpHoverLifecycleAndPreference(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("")
	window.Resize(fyne.NewSize(260, 180))
	button := NewHelpButton(strings.Repeat("Optional explanation. ", 30))
	window.SetContent(button)
	window.Show()
	previous := ShowTooltips
	ShowTooltips = true
	defer func() { ShowTooltips = previous }()
	button.MouseIn(&desktop.MouseEvent{})
	time.Sleep(750 * time.Millisecond)
	fyne.DoAndWait(func() {
		if button.help.popup == nil {
			t.Error("delayed hover did not show explanation")
		} else if button.help.popup.Content.Size().Width > window.Canvas().Size().Width {
			t.Error("help exceeds viewport width")
		}
		if button.help.popup != nil && button.help.popup.Content.Size().Height > window.Canvas().Size().Height {
			t.Error("help exceeds viewport height")
		}
		button.MouseOut()
		if button.help.popup != nil {
			t.Error("pointer exit retained popup")
		}
		ShowTooltips = false
		button.MouseIn(&desktop.MouseEvent{})
		if button.help.timer != nil {
			t.Error("disabled tooltip started timer")
		}
	})
	// Click and keyboard activation remain available with hover help disabled.
	test.Tap(button)
	if len(window.Canvas().Overlays().List()) == 0 {
		t.Fatal("click help unavailable with tooltips disabled")
	}
	for _, overlay := range window.Canvas().Overlays().List() {
		overlay.Hide()
	}
	window.Canvas().Focus(button)
	button.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if len(window.Canvas().Overlays().List()) == 0 {
		t.Fatal("keyboard help unavailable with tooltips disabled")
	}
}

func TestDismissTooltipsRetiresPendingAndVisibleHelp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("")
	button := NewHelpButton("Optional explanation")
	window.SetContent(button)
	window.Show()
	previous := ShowTooltips
	ShowTooltips = true
	defer func() { DismissTooltips(); ShowTooltips = previous }()
	button.MouseIn(&desktop.MouseEvent{})
	DismissTooltips()
	time.Sleep(750 * time.Millisecond)
	fyne.DoAndWait(func() {
		if button.help.popup != nil || button.help.timer != nil || len(activeHoverHelp) != 0 {
			t.Error("navigation retained pending help")
		}
		button.MouseIn(&desktop.MouseEvent{})
	})
	time.Sleep(750 * time.Millisecond)
	fyne.DoAndWait(func() {
		if button.help.popup == nil {
			t.Error("second hover did not open")
		}
		DismissTooltips()
		if button.help.popup != nil || len(window.Canvas().Overlays().List()) != 0 {
			t.Error("navigation retained visible help")
		}
	})
}

func TestTooltipEntryPreservesEditing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := NewTooltipEntry("Optional chapter syntax")
	window := app.NewWindow("")
	window.SetContent(entry)
	test.Type(entry, "1-3, 5")
	if entry.Text != "1-3, 5" {
		t.Fatalf("entry editing changed: %q", entry.Text)
	}
	entry.SetToolTip("Updated explanation")
	if entry.help.text != "Updated explanation" {
		t.Fatal("tooltip update lost")
	}
}
