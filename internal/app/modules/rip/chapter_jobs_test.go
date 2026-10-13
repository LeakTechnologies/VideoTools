package rip

import (
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/queue"
)

func TestChapterJobsKeepIndependentRangesAndSettings(t *testing.T) {
	ranges, err := ParseChapterSelection("1-3, 5-6, 8")
	if err != nil {
		t.Fatal(err)
	}
	job := &queue.Job{Type: queue.JobTypeRip, InputFile: "source.iso", OutputFile: "output.mkv", Config: map[string]interface{}{"titleNumber": 2, "vtsNumber": 1, "embedChapters": true, "format": FormatH264MKV}}
	jobs := chapterOutputJobs(job, ranges)
	if len(jobs) != 3 {
		t.Fatalf("got %d outputs", len(jobs))
	}
	for i, got := range jobs {
		if got.OutputFile != ChapterRangeOutputPath(job.OutputFile, ranges[i]) || got.Config["outputPath"] != got.OutputFile || got.Config["chapterStart"] != ranges[i].Start || got.Config["chapterEnd"] != ranges[i].End || got.Config["chapterSelectionStrict"] != true || got.Config["titleNumber"] != 2 || got.Config["embedChapters"] != true {
			t.Fatalf("output %d lost range/settings: %+v", i, got)
		}
	}
	jobs[0].Config["titleNumber"] = 99
	if jobs[1].Config["titleNumber"] != 2 || job.Config["titleNumber"] != 2 {
		t.Fatal("outputs share mutable configuration")
	}
	if got := chapterOutputJobs(job, nil); len(got) != 1 || got[0] != job {
		t.Fatal("all chapters must preserve whole-title job")
	}
}

func TestChapterSelectionPreflightsEveryTarget(t *testing.T) {
	vs := &viewState{chapterOnly: true, chapterSelection: "1-3, 5-6, 8", selectedTitles: map[int]bool{1: true, 2: true}, scanResult: &DiscScanResult{Titles: []DiscTitle{{Number: 1, Duration: 100, Chapters: make([]float64, 8)}, {Number: 2, Duration: 50, Chapters: make([]float64, 6)}}}}
	if ranges, count, err := vs.selectedChapterRanges(); err == nil || ranges != nil || count != 0 {
		t.Fatal("shorter title must reject the entire batch")
	}
	vs.extractMode = "main"
	if ranges, count, err := vs.selectedChapterRanges(); err != nil || len(ranges) != 3 || count != 3 {
		t.Fatalf("main title selection: %v %d %v", ranges, count, err)
	}
	vs.extractMode = ""
	vs.selectedTitles[2] = false
	if _, count, err := vs.selectedChapterRanges(); err != nil || count != 3 {
		t.Fatalf("unselected titles affect preflight: %d %v", count, err)
	}
	vs.selectedTitles[2] = true
	vs.scanResult.Titles[1].Chapters = make([]float64, 8)
	if _, count, err := vs.selectedChapterRanges(); err != nil || count != 6 {
		t.Fatalf("two titles should yield six files: %d %v", count, err)
	}
	vs.scanResult = nil
	if _, _, err := vs.selectedChapterRanges(); err == nil {
		t.Fatal("unknown chapter data must reject selected mode")
	}
	vs.chapterOnly = false
	if ranges, _, err := vs.selectedChapterRanges(); err != nil || ranges != nil {
		t.Fatal("all chapters must not require scan")
	}
}

func TestIndividualChaptersUseEachTitlesOwnCount(t *testing.T) {
	vs := &viewState{chapterOnly: true, chapterIndividual: true, selectedTitles: map[int]bool{1: true, 2: true}, scanResult: &DiscScanResult{Titles: []DiscTitle{{Number: 1, Chapters: make([]float64, 5)}, {Number: 2, Chapters: make([]float64, 2)}}}}
	if _, count, err := vs.selectedChapterRanges(); err != nil || count != 7 {
		t.Fatalf("individual files: %d %v", count, err)
	}
	for _, title := range vs.chapterTargets() {
		ranges, err := vs.rangesForTitle(title)
		if err != nil || len(ranges) != len(title.Chapters) {
			t.Fatalf("title-specific ranges: %v %v", ranges, err)
		}
		for i, span := range ranges {
			if span.Start != i+1 || span.End != i+1 {
				t.Fatal("individual chapters merged")
			}
		}
	}
	vs.chapterIndividual = false
	vs.chapterSelections = map[int]string{1: "1, 3-4", 2: "2"}
	if _, count, err := vs.selectedChapterRanges(); err != nil || count != 3 {
		t.Fatalf("per-title selection: %d %v", count, err)
	}
	vs.chapterSelections[2] = "3"
	if _, count, err := vs.selectedChapterRanges(); err == nil || count != 0 {
		t.Fatal("invalid per-title choice accepted")
	}
}
