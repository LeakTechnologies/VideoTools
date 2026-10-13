package rip

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type largeCardTheme struct{ fyne.Theme }

func (t largeCardTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 24
	}
	return t.Theme.Size(name)
}

func TestCompactTitleCardGeometry(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	for _, fontTheme := range []fyne.Theme{theme.DefaultTheme(), largeCardTheme{theme.DefaultTheme()}} {
		a.Settings().SetTheme(fontTheme)
		cb := NewContentBrowser()
		card := cb.buildCardTemplate().(*cardBox)
		details := card.Container.Objects[2].(*fyne.Container)
		details.Objects[0].(*widget.Label).SetText("T13  1h 29m  Main feature")
		for _, width := range []float32{360, 600, 1000} {
			card.Resize(fyne.NewSize(width, card.MinSize().Height))
			title, info, entry := details.Objects[0], details.Objects[1], details.Objects[2]
			if title.Position().Y != entry.Position().Y || info.Position().Y < entry.Size().Height || entry.Size().Width < 80 {
				t.Fatalf("bad compact layout at %v: title=%v entry=%v/%v info=%v", width, title.Position(), entry.Position(), entry.Size(), info.Position())
			}
			if details.Position().X+details.Size().Width > card.Container.Objects[4].Position().X || entry.Position().X+entry.Size().Width > details.Size().Width+0.01 {
				t.Fatal("name/details overlap the selection control or leave the card")
			}
			thumb := card.Container.Objects[1]
			if thumb.Size().Width != thumbWidth || thumb.Size().Height != thumbHeight {
				t.Fatal("preview slot must remain available before images arrive")
			}
		}
	}
}
