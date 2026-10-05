package rip

// ripTargetTitle is one title's resolved rip work.
type ripTargetTitle struct {
	VTSNumber   int
	TitleNumber int
}

// ripTargetTitles returns the titles a rip should extract, and whether that set
// is non-empty.
//
// The selection is the ONLY authority for whether a title rips, at every title
// count. addToQueue previously branched on the COUNT of scanned titles: more
// than one iterated the selection and refused an empty one, while exactly one
// (and no scan result) took an unconditional path that always enqueued. Because
// "Movie + extras (choose titles)" deliberately starts with an EMPTY selection,
// a one-title disc switched to that mode reported "ready to rip — no titles
// selected" and then ripped anyway: the readiness line and the queue disagreed
// about what would happen.
//
// With no scan result there is nothing to select against, so the executor's own
// main-feature defaults apply (largest VTS set / title 1) and a single
// zero-numbered target is returned — that case stays unconditional.
func ripTargetTitles(titles []DiscTitle, selected map[int]bool) ([]ripTargetTitle, bool) {
	if len(titles) == 0 {
		return []ripTargetTitle{{}}, true
	}

	out := make([]ripTargetTitle, 0, len(titles))
	for _, dt := range titles {
		if !selected[dt.Number] {
			continue
		}
		out = append(out, ripTargetTitle{VTSNumber: dt.VTSNumber, TitleNumber: dt.Number})
	}
	return out, len(out) > 0
}

// CanonicalSelection returns the title selection the active rip mode implies:
//
//	"main"     — only the single longest title (the main feature)
//	"segments" — only the scene segments of a detected scene set
//	""         — "Movie + extras (choose titles)": the main feature plus any
//	             genuine extras, WITHOUT re-ripping the movie's scene segments
//	             (the whole movie already contains them) and WITHOUT selecting
//	             duplicate whole-movie copies (one encode suffices). When no
//	             scene set is detected, nothing is pre-selected — whatever the
//	             user ticks is exactly what rips (no surprise batch left over
//	             from the mode switch).
//	"full"     — every title (full-disc mode, single-Job path)
//
// The ContentBrowser reshapes to this selection only when the rip mode
// actually changes, so manual per-title toggles made inside a mode are
// preserved. Titles map is the scan result; ss is the detected scene-set
// layout (may be Present=false).
func CanonicalSelection(titles []DiscTitle, mode string, ss SceneSetInfo) map[int]bool {
	sel := make(map[int]bool)
	switch mode {
	case "main":
		bestNum, bestDur := 0, 0.0
		for _, dt := range titles {
			if dt.Duration > bestDur {
				bestDur, bestNum = dt.Duration, dt.Number
			}
		}
		if bestNum != 0 {
			sel[bestNum] = true
		}
	case "segments":
		if ss.Present {
			for num := range ss.SceneTitles {
				sel[num] = true
			}
		}
	case "":
		if !ss.Present {
			break
		}
		for _, dt := range titles {
			if ss.SceneTitles[dt.Number] {
				// Scene segments duplicate the whole movie's content — leave
				// unticked so the movie + extras rip doesn't download them twice.
				continue
			}
			if ss.WholeTitles[dt.Number] && dt.Number != ss.Representative {
				// Duplicate whole-movie encodes: one copy is the movie, the
				// rest are the same feature again, so leave them unticked.
				continue
			}
			sel[dt.Number] = true
		}
		// The representative whole copy is selected by the loop above when it
		// is present in titles; force it only if the caller filtered it out of
		// the slice but still wants the movie included.
		if ss.Representative != 0 {
			for _, dt := range titles {
				if dt.Number == ss.Representative {
					sel[ss.Representative] = true
					break
				}
			}
		}
	case "full":
		for _, dt := range titles {
			sel[dt.Number] = true
		}
	}
	return sel
}

// CanonicalLock maps the active rip mode onto the ContentBrowser lock layer:
// "main" greys out every title but the longest and anchors that one; "segments"
// greys the whole-movie copies while leaving every scene segment individually
// toggleable. Choose-titles ("") and full-disc modes lock nothing.
func CanonicalLock(titles []DiscTitle, mode string, ss SceneSetInfo) LockConfig {
	var cfg LockConfig
	switch mode {
	case "main":
		bestNum, bestDur := 0, 0.0
		for _, dt := range titles {
			if dt.Duration > bestDur {
				bestDur, bestNum = dt.Duration, dt.Number
			}
		}
		if bestNum == 0 {
			return cfg
		}
		cfg.Locked = map[int]bool{}
		for _, dt := range titles {
			if dt.Number != bestNum {
				cfg.Locked[dt.Number] = true
			}
		}
		cfg.Anchored = map[int]bool{bestNum: true}
	case "segments":
		if !ss.Present || len(ss.WholeTitles) == 0 {
			return cfg
		}
		cfg.Locked = map[int]bool{}
		for num := range ss.WholeTitles {
			cfg.Locked[num] = true
		}
		cfg.Anchored = map[int]bool{}
	}
	return cfg
}
