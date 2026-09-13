package rip

import (
	"reflect"
	"testing"
)

func toDurations(numDur map[int]float64) []DiscTitle {
	titles := make([]DiscTitle, 0, len(numDur))
	for num, dur := range numDur {
		titles = append(titles, DiscTitle{Number: num, Duration: dur, VTSNumber: 1})
	}
	return titles
}

func TestDetectSceneSetsRedHairyTeens(t *testing.T) {
	titles := []DiscTitle{
		{Number: 1, Duration: 5926.48, VTSNumber: 1, NumChapters: 15},
		{Number: 2, Duration: 5932.48, VTSNumber: 1, NumChapters: 15},
		{Number: 3, Duration: 866.04, VTSNumber: 1, NumChapters: 3},
		{Number: 4, Duration: 615.40, VTSNumber: 1, NumChapters: 3},
		{Number: 5, Duration: 1977.60, VTSNumber: 1, NumChapters: 3},
		{Number: 6, Duration: 1331.08, VTSNumber: 1, NumChapters: 3},
		{Number: 7, Duration: 1141.16, VTSNumber: 1, NumChapters: 3},
	}
	info := DetectSceneSets(titles)
	if !info.Present {
		t.Fatalf("expected a scene set on the scene-segmented disc, got none")
	}
	wantWhole := map[int]bool{1: true, 2: true}
	wantScene := map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true}
	if !reflect.DeepEqual(info.WholeTitles, wantWhole) {
		t.Errorf("whole titles = %v, want %v", info.WholeTitles, wantWhole)
	}
	if !reflect.DeepEqual(info.SceneTitles, wantScene) {
		t.Errorf("scene titles = %v, want %v", info.SceneTitles, wantScene)
	}
	if info.Representative != 2 {
		t.Errorf("representative = %d, want 2", info.Representative)
	}
}

func TestDetectSceneSetsNoStructure(t *testing.T) {
	titles := toDurations(map[int]float64{
		1: 6000, 2: 900, 3: 700, 4: 300,
	})
	if info := DetectSceneSets(titles); info.Present {
		t.Errorf("movie + unrelated extras must not be a scene set: %+v", info)
	}
}

func TestDetectSceneSetsSingleAndTwin(t *testing.T) {
	if info := DetectSceneSets(toDurations(map[int]float64{1: 6000})); info.Present {
		t.Errorf("single title must not be a scene set")
	}
	// One whole + one near-identical twin but no scene segments.
	if info := DetectSceneSets(toDurations(map[int]float64{1: 6000, 2: 6005})); info.Present {
		t.Errorf("twin without scenes must not be a scene set: %+v", info)
	}
}

func TestDetectSceneSetsHalvesToleranceBoundary(t *testing.T) {
	// Two segments that each run exactly half the whole duration match the
	// sum tolerance; this is the documented acceptance boundary.
	info := DetectSceneSets(toDurations(map[int]float64{
		1: 5000, 2: 2500, 3: 2500,
	}))
	if !info.Present {
		t.Fatalf("half-half partition should be detected at the tolerance boundary")
	}
	if !info.WholeTitles[1] || !info.SceneTitles[2] || !info.SceneTitles[3] {
		t.Errorf("half-half: whole=%v scenes=%v", info.WholeTitles, info.SceneTitles)
	}
}

func TestDetectSceneSetsDuplicateSceneDurations(t *testing.T) {
	titles := toDurations(map[int]float64{
		1: 4600, 2: 1000, 3: 1000, 4: 1000, 5: 1000, 6: 600,
	})
	info := DetectSceneSets(titles)
	if !info.Present {
		t.Fatalf("duplicate-duration scenes should be detected")
	}
	if !info.WholeTitles[1] {
		t.Errorf("expected T01 whole, got whole=%v", info.WholeTitles)
	}
	if !info.SceneTitles[2] || !info.SceneTitles[3] || !info.SceneTitles[4] || !info.SceneTitles[5] || !info.SceneTitles[6] {
		t.Errorf("expected all five scenes, got %v", info.SceneTitles)
	}
}