package rip

import "testing"

func mkTitles(durations ...float64) []DiscTitle {
	out := make([]DiscTitle, 0, len(durations))
	for i, d := range durations {
		out = append(out, DiscTitle{Number: i + 1, Duration: d})
	}
	return out
}

func TestCanonicalSelection(t *testing.T) {
	titles := mkTitles(5926.48, 5932.48, 866.04, 615.40, 1977.60, 1331.08, 1141.16)
	ss := SceneSetInfo{
		Present:     true,
		WholeTitles: map[int]bool{1: true, 2: true},
		SceneTitles: map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true},
	}

	t.Run("main selects only the longest title", func(t *testing.T) {
		sel := CanonicalSelection(titles, "main", ss)
		if len(sel) != 1 || !sel[2] {
			t.Fatalf("main selection = %v, want {2:true}", sel)
		}
	})

	t.Run("segments selects only the scenes", func(t *testing.T) {
		sel := CanonicalSelection(titles, "segments", ss)
		for n := 1; n <= 7; n++ {
			want := n >= 3
			if sel[n] != want {
				t.Fatalf("segments selection[%d] = %v, want %v", n, sel[n], want)
			}
		}
	})

	t.Run("choose-titles selects everything", func(t *testing.T) {
		sel := CanonicalSelection(titles, "", ss)
		if len(sel) != 7 {
			t.Fatalf("choose-titles selection = %v, want all 7", sel)
		}
	})

	t.Run("full selects everything", func(t *testing.T) {
		sel := CanonicalSelection(titles, "full", ss)
		if len(sel) != 7 {
			t.Fatalf("full selection = %v, want all 7", sel)
		}
	})

	t.Run("segments without a scene set selects nothing", func(t *testing.T) {
		sel := CanonicalSelection(titles, "segments", SceneSetInfo{})
		if len(sel) != 0 {
			t.Fatalf("segments(no scene set) selection = %v, want empty", sel)
		}
	})

	t.Run("no titles", func(t *testing.T) {
		if sel := CanonicalSelection(nil, "", ss); len(sel) != 0 {
			t.Fatalf("empty selection = %v, want empty", sel)
		}
	})
}

func TestCanonicalLock(t *testing.T) {
	titles := mkTitles(5926.48, 5932.48, 866.04, 615.40)
	ss := SceneSetInfo{
		Present:     true,
		WholeTitles: map[int]bool{1: true, 2: true},
		SceneTitles: map[int]bool{3: true, 4: true},
	}

	t.Run("main greys every title but the longest and anchors it", func(t *testing.T) {
		cfg := CanonicalLock(titles, "main", ss)
		if len(cfg.Locked) != 3 || !cfg.Locked[1] || !cfg.Locked[3] || !cfg.Locked[4] {
			t.Fatalf("main lockdown = %v, want titles 1,3,4 locked", cfg.Locked)
		}
		if len(cfg.Anchored) != 1 || !cfg.Anchored[2] {
			t.Fatalf("main anchor = %v, want {2:true}", cfg.Anchored)
		}
		if cfg.Locked[2] {
			t.Fatalf("main title must not be locked: %v", cfg.Locked)
		}
	})

	t.Run("segments greys the whole copies, scene segments stay free", func(t *testing.T) {
		cfg := CanonicalLock(titles, "segments", ss)
		if len(cfg.Locked) != 2 || !cfg.Locked[1] || !cfg.Locked[2] {
			t.Fatalf("segments lockdown = %v, want whole copies 1,2 locked", cfg.Locked)
		}
		if cfg.Locked[3] || cfg.Locked[4] {
			t.Fatalf("scene segments must stay toggleable: %v", cfg.Locked)
		}
		if len(cfg.Anchored) != 0 {
			t.Fatalf("segments anchor = %v, want none", cfg.Anchored)
		}
	})

	t.Run("choose-titles locks nothing", func(t *testing.T) {
		cfg := CanonicalLock(titles, "", ss)
		if len(cfg.Locked) != 0 || len(cfg.Anchored) != 0 {
			t.Fatalf("choose-titles lock = %v/%v, want empty", cfg.Locked, cfg.Anchored)
		}
	})

	t.Run("segments without a scene set locks nothing", func(t *testing.T) {
		cfg := CanonicalLock(titles, "segments", SceneSetInfo{})
		if len(cfg.Locked) != 0 || len(cfg.Anchored) != 0 {
			t.Fatalf("segments(no set) lock = %v/%v, want empty", cfg.Locked, cfg.Anchored)
		}
	})
}