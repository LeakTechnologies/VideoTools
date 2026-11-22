package ui

import (
	"testing"
	"time"
)

func TestTimelineProgrammaticPaintCannotSeek(t *testing.T) {
	var timeline *PlayerTimeline
	seeks, previews := 0, 0
	timeline = &PlayerTimeline{SetPosition: func(v float64) { timeline.Changed(v) }, Preview: func(float64) { previews++ }, Seek: func(float64) { seeks++ }}
	for i := 0; i < 30; i++ {
		timeline.Update(float64(i))
	}
	if seeks != 0 || previews != 0 {
		t.Fatalf("progress became transport: seeks=%d previews=%d", seeks, previews)
	}
}
func TestTimelineDragCommitsOnceAndRejectsStaleProgress(t *testing.T) {
	position := 0.0
	var seek []float64
	timeline := &PlayerTimeline{SetPosition: func(v float64) { position = v }, Seek: func(v float64) { seek = append(seek, v) }}
	timeline.Update(10)
	timeline.Changed(40)
	timeline.Update(11)
	if position != 10 {
		t.Fatal("playback repainted drag")
	}
	timeline.Commit(40)
	timeline.Update(12)
	if position != 10 {
		t.Fatal("stale progress overwrote committed target")
	}
	timeline.Update(40.1)
	if position != 40.1 || len(seek) != 1 || seek[0] != 40 {
		t.Fatalf("commit missing: pos=%v seeks=%v", position, seek)
	}
}
func TestTimelineNewestSeekAndEndReset(t *testing.T) {
	position := 0.0
	timeline := &PlayerTimeline{SetPosition: func(v float64) { position = v }}
	timeline.Commit(50)
	timeline.Commit(20)
	timeline.Update(50)
	if position != 0 {
		t.Fatal("older seek accepted")
	}
	timeline.Update(20)
	if position != 20 {
		t.Fatal("newest target rejected")
	}
	timeline.Reset(0)
	timeline.Update(0.1)
	if position != 0.1 {
		t.Fatal("EOF reset left seeking guard")
	}
}

func TestTimelineExpiredSeekAcceptsActualProgress(t *testing.T) {
	var painted float64
	timeline := &PlayerTimeline{SetPosition: func(v float64) { painted = v }}
	timeline.Commit(90)
	timeline.pendingUntil = time.Time{}
	timeline.Update(5)
	if painted != 5 || timeline.pending {
		t.Fatal("expired seek still suppresses playback position")
	}
}
