package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// PreviewDetailsLayout gives source details the flexible workspace while an
// optional preview stays compact. Objects are preview header, preview body,
// and the details scroll. CollapsedDetailsHeight measures the real details
// header instead of reserving an arbitrary fraction of the window.
type PreviewDetailsLayout struct {
	DetailsOpen            bool
	CollapsedDetailsHeight func() float32
}

func (l *PreviewDetailsLayout) detailsFloor() float32 {
	if !l.DetailsOpen && l.CollapsedDetailsHeight != nil {
		return l.CollapsedDetailsHeight()
	}
	return 180
}

func (l *PreviewDetailsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 3 {
		return
	}
	header, preview, details := objects[0], objects[1], objects[2]
	gap := theme.Padding()
	headerH := header.MinSize().Height
	header.Move(fyne.NewPos(0, 0))
	header.Resize(fyne.NewSize(size.Width, headerH))
	y := headerH + gap
	available := fyne.Max(0, size.Height-y)
	if preview.Visible() {
		previewH := fyne.Max(0, available-gap-l.detailsFloor())
		if l.DetailsOpen {
			previewH = fyne.Min(preview.MinSize().Height, previewH)
		}
		preview.Move(fyne.NewPos(0, y))
		preview.Resize(fyne.NewSize(size.Width, previewH))
		y += previewH + gap
	}
	details.Move(fyne.NewPos(0, y))
	details.Resize(fyne.NewSize(size.Width, fyne.Max(0, size.Height-y)))
}

func (l *PreviewDetailsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 3 {
		return fyne.Size{}
	}
	header, preview, details := objects[0], objects[1], objects[2]
	width := fyne.Max(header.MinSize().Width, details.MinSize().Width)
	height := header.MinSize().Height + theme.Padding() + l.detailsFloor()
	if preview.Visible() {
		width = fyne.Max(width, preview.MinSize().Width)
		height += preview.MinSize().Height + theme.Padding()
	}
	return fyne.NewSize(width, height)
}
