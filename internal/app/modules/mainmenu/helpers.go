package mainmenu

import (
	"image/color"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
)

type SourceModule struct {
	ID            string
	Label         string
	Color         color.Color
	TextColor     color.Color
	Category      string
	HasHandler    bool
	DepsAvailable bool
}

type Visibility struct {
	ShowUpscale bool
	ShowDisc    bool
	IsDevBuild  bool
}

func BuildVisibleModules(source []SourceModule, vis Visibility) []ui.ModuleInfo {
	out := make([]ui.ModuleInfo, 0, len(source))
	for _, m := range source {
		if !isVisibleByPreference(m.ID, vis) {
			continue
		}
		// Settings is always enabled; burn and filemanager only appear in dev builds
		enabled := m.ID == "settings" || (m.HasHandler && m.DepsAvailable)
		missingDeps := m.HasHandler && !m.DepsAvailable && m.ID != "settings"
		out = append(out, ui.ModuleInfo{
			ID:                  m.ID,
			Label:               m.Label,
			Color:               m.Color,
			TextColor:           m.TextColor,
			Category:            m.Category,
			Enabled:             enabled,
			MissingDependencies: missingDeps,
		})
	}
	return out
}

func BuildActiveJobs(queueList []*queue.Job) []ui.HistoryEntry {
	active := make([]ui.HistoryEntry, 0)
	for _, job := range queueList {
		if job.Status != queue.JobStatusRunning && job.Status != queue.JobStatusPending {
			continue
		}
		active = append(active, ui.HistoryEntry{
			ID:         job.ID,
			Type:       job.Type,
			Status:     job.Status,
			Title:      job.Title,
			InputFile:  job.InputFile,
			OutputFile: job.OutputFile,
			LogPath:    job.LogPath,
			Config:     job.Config,
			CreatedAt:  job.CreatedAt,
			StartedAt:  job.StartedAt,
			Error:      job.Error,
			Progress:   job.Progress / 100.0,
		})
	}
	return active
}

func isVisibleByPreference(moduleID string, vis Visibility) bool {
	switch moduleID {
	case "upscale":
		// Gated behind settings preference in dev; hidden entirely in release.
		return vis.IsDevBuild && vis.ShowUpscale
	case "author", "rip":
		return vis.ShowDisc
	case "trim", "audio", "burn", "filemanager", "subtitles":
		// Under development — visible in dev builds only.
		return vis.IsDevBuild
	default:
		return true
	}
}
