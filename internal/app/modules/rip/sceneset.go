package rip

import (
	"math/bits"
	"sort"
)

// SceneSetInfo describes a disc authored as a whole movie plus the scene
// segments that partition it — the "sketch show / adult / wrestling" layout
// where the full feature is presented as a single (or duplicated) title and
// again as individual scenes. A whole title's play time matches the sum of
// several shorter titles' durations within SceneSetTolerance.
type SceneSetInfo struct {
	Present        bool
	WholeTitles    map[int]bool // disc title numbers that are whole-movie copies
	SceneTitles    map[int]bool // disc title numbers that are the movie's scene segments
	Representative int          // disc title number of the longest whole copy (for labels)
}

const (
	// sceneSetTolerance bounds the relative difference between a whole
	// title's duration and the sum of its scene segments (0.75%).
	sceneSetTolerance = 0.0075
	// sceneSetMinMembers is the minimum number of segments before a title
	// group is treated as a deliberate scene-set rather than coincidence.
	sceneSetMinMembers = 2
)

type sceneEntry struct {
	num int
	dur int
}

// DetectSceneSets looks for a movie/scene partition in the disc's titles. A
// title is a "whole" copy when at least sceneSetMinMembers other, shorter
// titles sum (within sceneSetTolerance) to its duration; titles participating
// in such a partition are its scenes. Near-identical durations (within the
// same tolerance, and >= 95% of the longest whole) are treated as duplicate
// whole copies (two encodes of the full movie). Present=false is returned for
// discs without that structure so the UI only offers the scenes-only mode
// when it is meaningful.
func DetectSceneSets(titles []DiscTitle) SceneSetInfo {
	info := SceneSetInfo{WholeTitles: map[int]bool{}, SceneTitles: map[int]bool{}}
	if len(titles) < sceneSetMinMembers+1 {
		return info
	}

	byDur := make([]sceneEntry, 0, len(titles))
	for _, dt := range titles {
		if dt.Duration <= 0 {
			return info
		}
		byDur = append(byDur, sceneEntry{num: dt.Number, dur: int(dt.Duration + 0.5)})
	}
	sort.SliceStable(byDur, func(i, j int) bool { return byDur[i].dur > byDur[j].dur })

	wholeDurs := map[int]int{} // whole title number -> duration (for twin detection)

	for i, w := range byDur {
		if info.WholeTitles[w.num] || info.SceneTitles[w.num] {
			continue
		}
		if i == len(byDur)-1 {
			break
		}
		pool := make([]sceneEntry, 0, len(byDur))
		for _, e := range byDur {
			if e.num != w.num && e.dur < w.dur && !info.WholeTitles[e.num] {
				pool = append(pool, e)
			}
		}
		if len(pool) < sceneSetMinMembers {
			continue
		}
		mask, _ := subsetSumWithin(pool, w.dur, sceneSetTolerance)
		if mask == 0 || bits.OnesCount32(mask) < sceneSetMinMembers {
			continue
		}
		info.WholeTitles[w.num] = true
		wholeDurs[w.num] = w.dur
		for pi := range pool {
			if mask&(1<<uint(pi)) != 0 {
				info.SceneTitles[pool[pi].num] = true
			}
		}
	}

	// Twin pass: duplicate whole copies (e.g. two encodes of the full movie).
	for _, e := range byDur {
		if info.WholeTitles[e.num] || info.SceneTitles[e.num] {
			continue
		}
for _, d := range wholeDurs {
			if float64(e.dur) >= float64(d)*0.95 && nearInt(e.dur, d, sceneSetTolerance) {
				info.WholeTitles[e.num] = true
				wholeDurs[e.num] = e.dur
				break
			}
		}
	}

	info.Present = len(info.WholeTitles) >= 1 && len(info.SceneTitles) >= sceneSetMinMembers
	best := 0
	for num, d := range wholeDurs {
		if d > best {
			best, info.Representative = d, num
		}
	}
	return info
}

// subsetSumWithin finds a subset of pool whose sum is within tol of target.
// It returns the chosen subset as a bitmask over pool indices (0 when none)
// plus the sum actually reached. With the small title counts found on optical
// media a plain bounded knapsack over seconds is cheap.
func subsetSumWithin(pool []sceneEntry, target int, tol float64) (uint32, int) {
	if target <= 0 {
		return 0, 0
	}
	upper := int(float64(target)*(1+tol)) + 1
	reach := make([]bool, upper+1)
	maskAt := make([]uint32, upper+1)
	reach[0] = true
	for i, e := range pool {
		if e.dur > upper {
			continue
		}
		for s := upper; s >= e.dur; s-- {
			if reach[s] || !reach[s-e.dur] {
				continue
			}
			reach[s] = true
			maskAt[s] = maskAt[s-e.dur] | (1 << uint(i))
		}
	}
	lo := int(float64(target) * (1 - tol))
	best, bestMask, bestDelta := -1, uint32(0), int(^uint(0)>>1)
	for s := lo; s <= upper; s++ {
		if !reach[s] {
			continue
		}
		delta := s - target
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestMask, bestDelta = s, maskAt[s], delta
		}
	}
	if best == -1 {
		return 0, 0
	}
	return bestMask, best
}

func nearInt(a, b int, tol float64) bool {
	delta := a - b
	if delta < 0 {
		delta = -delta
	}
	return float64(delta) <= float64(b)*tol
}