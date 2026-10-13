package rip

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/i18n"
)

// ChapterSelectionRange describes one output file's inclusive chapter span.
type ChapterSelectionRange struct {
	Start int
	End   int
}

// ParseChapterSelection keeps each comma-separated item as an independent
// output span, ordered by its position on the disc. Adjacent items stay separate.
func ParseChapterSelection(text string) ([]ChapterSelectionRange, error) {
	invalid := func() error { return fmt.Errorf("%s", i18n.T().RipChapterInputInvalid) }
	var ranges []ChapterSelectionRange
	for _, token := range strings.Split(text, ",") {
		parts := strings.Split(strings.TrimSpace(token), "-")
		if len(parts) > 2 {
			return nil, invalid()
		}
		numbers := make([]int, len(parts))
		for n, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, invalid()
			}
			for _, ch := range part {
				if ch < '0' || ch > '9' {
					return nil, invalid()
				}
			}
			value, err := strconv.Atoi(part)
			if err != nil || value < 1 {
				return nil, invalid()
			}
			numbers[n] = value
		}
		span := ChapterSelectionRange{Start: numbers[0], End: numbers[0]}
		if len(numbers) == 2 {
			span.End = numbers[1]
		}
		if span.End < span.Start {
			return nil, invalid()
		}
		ranges = append(ranges, span)
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].Start < ranges[j].Start })
	for n := 1; n < len(ranges); n++ {
		if ranges[n].Start <= ranges[n-1].End {
			return nil, fmt.Errorf("%s", i18n.T().RipChapterInputOverlap)
		}
	}
	return ranges, nil
}

// ValidateChapterSelection checks the scanned title without silently clamping
// the requested spans. It also protects callers constructing ranges directly.
func ValidateChapterSelection(ranges []ChapterSelectionRange, chapterCount int) error {
	if chapterCount < 1 {
		return fmt.Errorf("%s", i18n.T().RipChapterScanRequired)
	}
	if len(ranges) == 0 {
		return fmt.Errorf("%s", i18n.T().RipChapterInputInvalid)
	}
	ordered := append([]ChapterSelectionRange(nil), ranges...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	for n, span := range ordered {
		if span.Start < 1 || span.End < span.Start {
			return fmt.Errorf("%s", i18n.T().RipChapterInputInvalid)
		}
		if span.End > chapterCount {
			return fmt.Errorf(i18n.T().RipChapterUnavailableFmt, chapterCount)
		}
		if n > 0 && span.Start <= ordered[n-1].End {
			return fmt.Errorf("%s", i18n.T().RipChapterInputOverlap)
		}
	}
	return nil
}

// ChapterRangeOutputPath adds a stable chapter suffix before the extension.
func ChapterRangeOutputPath(path string, span ChapterSelectionRange) string {
	ext := filepath.Ext(path)
	suffix := fmt.Sprintf("_Chapter_%02d", span.Start)
	if span.End != span.Start {
		suffix = fmt.Sprintf("_Chapters_%02d-%02d", span.Start, span.End)
	}
	return strings.TrimSuffix(path, ext) + suffix + ext
}
