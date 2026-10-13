package rip

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// chapterChoices keeps numbered checks in a readable grid instead of one long row.
type chapterChoices struct {
	widget.BaseWidget
	Options   []string
	Selected  []string
	OnChanged func([]string)
	checks    []*widget.Check
	updating  bool
	content   *fyne.Container
}

func newChapterChoices(options []string) *chapterChoices {
	c := &chapterChoices{Options: options}
	c.ExtendBaseWidget(c)
	c.content = container.NewGridWithColumns(4)
	for _, option := range options {
		check := widget.NewCheck(option, func(bool) {
			if c.updating {
				return
			}
			c.Selected = nil
			for _, check := range c.checks {
				if check.Checked {
					c.Selected = append(c.Selected, check.Text)
				}
			}
			if c.OnChanged != nil {
				c.OnChanged(append([]string(nil), c.Selected...))
			}
		})
		c.checks = append(c.checks, check)
		c.content.Add(check)
	}
	return c
}
func (c *chapterChoices) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}
func (c *chapterChoices) SetSelected(selected []string) {
	c.updating = true
	c.Selected = append([]string(nil), selected...)
	for _, check := range c.checks {
		checked := false
		for _, value := range selected {
			if value == check.Text {
				checked = true
				break
			}
		}
		check.SetChecked(checked)
	}
	c.updating = false
	if c.OnChanged != nil {
		c.OnChanged(append([]string(nil), selected...))
	}
}
