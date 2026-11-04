package player

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"testing"
)

func findClearButton(o fyne.CanvasObject) *ui.PillButton {
	if button, ok := o.(*ui.PillButton); ok && button.Label == i18n.T().ActionClearVideo {
		return button
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if child != nil {
				if b := findClearButton(child); b != nil {
					return b
				}
			}
		}
	}
	return nil
}

func TestClearNotifiesSourceOwnerBeforeRetirementAndRebuild(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	var selected interface{} = "source A"
	var shownSource interface{}
	var view fyne.CanvasObject
	var build func()
	var calls []string
	build = func() {
		view = BuildView(Options{
			PlayerFile: selected, SourceName: "source", StatsBar: widget.NewLabel("status"),
			OnClearSource: func() { selected = nil; calls = append(calls, "clear") },
			OnReleasePlaybackSession: func() {
				if selected != nil {
					t.Error("retirement saw old host source")
				}
				calls = append(calls, "release")
			},
			OnStopPlayer:     func() { calls = append(calls, "stop") },
			OnShowPlayerView: func() { calls = append(calls, "rebuild"); build() },
			OnBuildVideoPane: func(_ interface{}, _ fyne.Size, source interface{}, _ func(float64)) fyne.CanvasObject {
				shownSource = source
				return widget.NewLabel("player fixture")
			},
		})
	}
	build()
	clear := findClearButton(view)
	if clear == nil {
		t.Fatal("loaded view has no Clear action")
	}
	test.Tap(clear)
	if selected != nil || shownSource != nil || findClearButton(view) != nil {
		t.Fatal("cleared source returned in rebuilt view")
	}
	if got := len(calls); got != 4 || calls[0] != "clear" || calls[1] != "release" || calls[2] != "stop" || calls[3] != "rebuild" {
		t.Fatalf("clear order: %v", calls)
	}
	build()
	if shownSource != nil || findClearButton(view) != nil {
		t.Fatal("later rebuild resurrected source")
	}
	selected = "source B"
	build()
	if shownSource != selected || findClearButton(view) == nil {
		t.Fatal("replacement did not load after clear")
	}
}
