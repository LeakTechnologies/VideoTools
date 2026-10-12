package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

func TestPreviewDetailsKeepsMetadataUsefulAndHeadersReachable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	box := func(w, h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(color.Black)
		r.SetMinSize(fyne.NewSize(w, h))
		return r
	}
	header, preview, details := box(100, 40), box(320, 180), box(0, 0)
	objects := []fyne.CanvasObject{header, preview, details}
	l := &PreviewDetailsLayout{DetailsOpen: true, CollapsedDetailsHeight: func() float32 { return 48 }}
	l.Layout(objects, fyne.NewSize(600, 720))
	if details.Size().Height <= preview.Size().Height {
		t.Fatal("metadata must receive more workspace than optional preview")
	}
	preview.Hide()
	l.Layout(objects, fyne.NewSize(600, 720))
	if details.Size().Height < 650 {
		t.Fatal("closing preview must release its allocation to metadata")
	}
	preview.Show()
	l.DetailsOpen = false
	for _, height := range []float32{360, 720, 1080} {
		l.Layout(objects, fyne.NewSize(600, height))
		if details.Size().Height < 48 {
			t.Fatalf("collapsed header clipped at height %v: %v", height, details.Size())
		}
		if preview.Position().Y+preview.Size().Height > details.Position().Y {
			t.Fatal("preview overlaps the metadata header")
		}
		if details.Position().Y+details.Size().Height > height {
			t.Fatal("metadata header is outside the workspace")
		}
	}
}
