package rip

import (
	"fmt"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
)

func (vs *viewState) selectedChapterRanges() ([]ChapterSelectionRange, int, error) {
	if !vs.chapterOnly {
		return nil, 0, nil
	}
	if vs.scanResult == nil || len(vs.scanResult.Titles) == 0 {
		return nil, 0, fmt.Errorf("%s", i18n.T().RipChapterScanRequired)
	}
	titles := vs.chapterTargets()
	if len(titles) == 0 {
		return nil, 0, fmt.Errorf("%s", i18n.T().RipReadyNoSelection)
	}
	var first []ChapterSelectionRange
	count := 0
	for _, title := range titles {
		ranges, err := vs.rangesForTitle(title)
		if err != nil {
			return nil, 0, fmt.Errorf("%s %d: %w", i18n.T().RipTitleShort, title.Number, err)
		}
		if first == nil {
			first = ranges
		}
		count += len(ranges)
	}
	return first, count, nil
}

func (vs *viewState) chapterTargets() []DiscTitle {
	if vs.scanResult == nil || len(vs.scanResult.Titles) == 0 {
		return nil
	}
	var titles []DiscTitle
	if vs.extractMode == "main" {
		main := vs.scanResult.Titles[0]
		for _, title := range vs.scanResult.Titles {
			if title.Duration > main.Duration {
				main = title
			}
		}
		titles = []DiscTitle{main}
	} else {
		for _, title := range vs.scanResult.Titles {
			if vs.selectedTitles[title.Number] {
				titles = append(titles, title)
			}
		}
	}
	return titles
}

func (vs *viewState) rangesForTitle(title DiscTitle) ([]ChapterSelectionRange, error) {
	if vs.chapterIndividual {
		ranges := make([]ChapterSelectionRange, len(title.Chapters))
		for i := range ranges {
			ranges[i] = ChapterSelectionRange{Start: i + 1, End: i + 1}
		}
		return ranges, ValidateChapterSelection(ranges, len(title.Chapters))
	}
	text := vs.chapterSelection
	if vs.chapterSelections != nil {
		text = vs.chapterSelections[title.Number]
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%s", i18n.T().RipChooseChapters)
	}
	ranges, err := ParseChapterSelection(text)
	if err != nil {
		return nil, err
	}
	return ranges, ValidateChapterSelection(ranges, len(title.Chapters))
}

// Expand only after every target title has passed selection preflight.
func chapterOutputJobs(job *queue.Job, ranges []ChapterSelectionRange) []*queue.Job {
	if len(ranges) == 0 {
		return []*queue.Job{job}
	}
	jobs := make([]*queue.Job, 0, len(ranges))
	for _, chapterRange := range ranges {
		copyJob := *job
		copyJob.Config = make(map[string]interface{}, len(job.Config)+2)
		for key, value := range job.Config {
			copyJob.Config[key] = value
		}
		copyJob.OutputFile = ChapterRangeOutputPath(job.OutputFile, chapterRange)
		copyJob.Config["outputPath"] = copyJob.OutputFile
		copyJob.Config["chapterStart"] = chapterRange.Start
		copyJob.Config["chapterEnd"] = chapterRange.End
		copyJob.Config["chapterSelectionStrict"] = true
		copyJob.Title = fmt.Sprintf("%s · %s %d–%d", job.Title, i18n.T().RipChaptersLabel, chapterRange.Start, chapterRange.End)
		copyJob.Description = copyJob.OutputFile
		jobs = append(jobs, &copyJob)
	}
	return jobs
}
