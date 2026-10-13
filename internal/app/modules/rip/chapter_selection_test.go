package rip

import (
	"reflect"
	"testing"
)

func TestParseChapterSelectionSeparateOutputs(t *testing.T) {
	for _, input := range []string{"1-3, 5-6, 8", "8, 5 - 6, 1 - 3", "01-03,05-06,08"} {
		got, err := ParseChapterSelection(input)
		want := []ChapterSelectionRange{{1, 3}, {5, 6}, {8, 8}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("parse %q = %v, %v; want %v", input, got, err, want)
		}
	}
	got, err := ParseChapterSelection("1, 2-3, 4")
	if err != nil || len(got) != 3 {
		t.Fatalf("adjacent items must remain three outputs: %v, %v", got, err)
	}
}

func TestParseChapterSelectionRejectsInvalidAndOverlappingInput(t *testing.T) {
	inputs := []string{"", " ", "1,", ",1", "1,,2", "0", "-1", "+1", "3-1", "1-", "1--3", "1-2-3", "1.5", "one", "1 2", "999999999999999999999999999999999999", "1,1", "1-3,3-5", "2-4,1-5"}
	for _, input := range inputs {
		if got, err := ParseChapterSelection(input); err == nil || got != nil {
			t.Errorf("invalid input %q accepted: %v, %v", input, got, err)
		}
	}
}

func TestValidateChapterSelectionUsesScannedCount(t *testing.T) {
	valid := []ChapterSelectionRange{{8, 8}, {1, 3}, {5, 6}}
	original := append([]ChapterSelectionRange(nil), valid...)
	if err := ValidateChapterSelection(valid, 8); err != nil {
		t.Fatalf("valid selection rejected: %v", err)
	}
	if !reflect.DeepEqual(valid, original) {
		t.Fatal("validation changed the caller's selection")
	}
	for _, count := range []int{-1, 0, 7} {
		if err := ValidateChapterSelection(valid, count); err == nil {
			t.Errorf("count %d must reject unavailable selection", count)
		}
	}
	for _, ranges := range [][]ChapterSelectionRange{nil, {{0, 1}}, {{3, 2}}, {{1, 3}, {3, 4}}} {
		if err := ValidateChapterSelection(ranges, 8); err == nil {
			t.Errorf("invalid constructed selection accepted: %v", ranges)
		}
	}
	if err := ValidateChapterSelection([]ChapterSelectionRange{{1000, 1000}}, 1000); err != nil {
		t.Fatalf("must not impose an arbitrary chapter limit: %v", err)
	}
}

func TestChapterRangeOutputPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		span ChapterSelectionRange
		want string
	}{
		{"output.mkv", ChapterSelectionRange{1, 3}, "output_Chapters_01-03.mkv"},
		{"output.mkv", ChapterSelectionRange{8, 8}, "output_Chapter_08.mkv"},
		{"folder/output.part.mp4", ChapterSelectionRange{98, 101}, "folder/output.part_Chapters_98-101.mp4"},
		{"output", ChapterSelectionRange{1, 1}, "output_Chapter_01"},
	} {
		if got := ChapterRangeOutputPath(tc.path, tc.span); got != tc.want {
			t.Errorf("output path = %q, want %q", got, tc.want)
		}
	}
}
