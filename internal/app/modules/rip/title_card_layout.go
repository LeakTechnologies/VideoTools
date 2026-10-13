package rip

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Keep the recycled card's object slots stable while giving details the
// flexible width, rather than leaving it at HBox minimum size beside a spacer.
type titleCardLayout struct{}

func (titleCardLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	gap := theme.Padding()
	checkW := objects[4].MinSize().Width
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(accentWidth, size.Height))
	objects[1].Move(fyne.NewPos(accentWidth+gap, (size.Height-thumbHeight)/2))
	objects[1].Resize(fyne.NewSize(thumbWidth, thumbHeight))
	x := accentWidth + thumbWidth + 2*gap
	objects[2].Move(fyne.NewPos(x, 0))
	objects[2].Resize(fyne.NewSize(fyne.Max(0, size.Width-x-checkW-2*gap), size.Height))
	objects[3].Move(fyne.NewPos(0, 0))
	objects[3].Resize(fyne.NewSize(0, 0))
	objects[4].Move(fyne.NewPos(size.Width-checkW, (size.Height-objects[4].MinSize().Height)/2))
	objects[4].Resize(objects[4].MinSize())
}
func (titleCardLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(accentWidth+thumbWidth+objects[2].MinSize().Width+objects[4].MinSize().Width+4*theme.Padding(), fyne.Max(thumbHeight, objects[2].MinSize().Height))
}

// Title identity and editable filename share the top row; metadata uses one
// compact line below. No repeated Output title label consumes a separate row.
type titleDetailsLayout struct{}

func (titleDetailsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	gap := theme.Padding()
	h := fyne.Max(objects[0].MinSize().Height, objects[2].MinSize().Height)
	label := objects[0].(*widget.Label)
	textW := fyne.Max(120, fyne.MeasureText("T00  000m 00s", theme.TextSize(), label.TextStyle).Width+2*gap)
	labelW := fyne.Min(size.Width*0.45, textW)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(labelW, h))
	objects[2].Move(fyne.NewPos(labelW+gap, 0))
	objects[2].Resize(fyne.NewSize(fyne.Max(0, size.Width-labelW-gap), h))
	objects[1].Move(fyne.NewPos(0, h+gap))
	objects[1].Resize(fyne.NewSize(size.Width, objects[1].MinSize().Height))
}
func (titleDetailsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	identityW := fyne.Max(80, fyne.MeasureText("T00", theme.TextSize(), fyne.TextStyle{Bold: true, Monospace: true}).Width+2*theme.Padding())
	nameW := fyne.Max(140, fyne.MeasureText("Title_000", theme.TextSize(), fyne.TextStyle{}).Width+4*theme.Padding())
	return fyne.NewSize(identityW+nameW+theme.Padding(), fyne.Max(objects[0].MinSize().Height, objects[2].MinSize().Height)+objects[1].MinSize().Height+theme.Padding())
}
