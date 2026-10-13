package rip

import (
	"fmt"

	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
)

func (vs *viewState) selectedChapterRanges() ([]ChapterSelectionRange, int, error) {
	if !vs.chapterOnly {
		return nil, 0, nil
	}
	ranges, err := ParseChapterSelection(vs.chapterSelection)
	if err != nil {
		return nil, 0, err
	}
	if vs.scanResult == nil || len(vs.scanResult.Titles) == 0 {
		return nil, 0, fmt.Errorf("%s", i18n.T().RipChapterScanRequired)
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
	if len(titles) == 0 {
		return nil, 0, fmt.Errorf("%s", i18n.T().RipReadyNoSelection)
	}
	for _, title := range titles {
		if err := ValidateChapterSelection(ranges, len(title.Chapters)); err != nil {
			return nil, 0, fmt.Errorf("%s %d: %w", i18n.T().RipTitleShort, title.Number, err)
		}
	}
	return ranges, len(titles) * len(ranges), nil
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
