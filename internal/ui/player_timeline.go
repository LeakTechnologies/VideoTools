package ui

import (
	"math"
	"time"
)

// PlayerTimeline is used on the UI thread. Programmatic paints cannot become
// seeks, and stale playback progress cannot overwrite an edit/committed target.
type PlayerTimeline struct {
	SetPosition                func(float64)
	Preview                    func(float64)
	Seek                       func(float64)
	updating, editing, pending bool
	target                     float64
	pendingUntil               time.Time
}

func (t *PlayerTimeline) Update(value float64) {
	if t.editing || (t.pending && time.Now().Before(t.pendingUntil) && math.Abs(value-t.target) > 1) {
		return
	}
	t.pending = false
	t.paint(value)
}
func (t *PlayerTimeline) paint(value float64) {
	t.updating = true
	defer func() { t.updating = false }()
	if t.SetPosition != nil {
		t.SetPosition(value)
	}
}
func (t *PlayerTimeline) Changed(value float64) {
	if t.updating {
		return
	}
	t.editing = true
	t.target = value
	if t.Preview != nil {
		t.Preview(value)
	}
}
func (t *PlayerTimeline) Commit(value float64) {
	if t.updating {
		return
	}
	t.editing = false
	t.pending = true
	// A failed/cancelled seek must not suppress actual progress indefinitely.
	t.pendingUntil = time.Now().Add(3 * time.Second)
	t.target = value
	if t.Seek != nil {
		t.Seek(value)
	}
}
func (t *PlayerTimeline) Reset(value float64) { t.editing = false; t.pending = false; t.paint(value) }
