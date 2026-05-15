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
}

func BuildVisibleModules(source []SourceModule, vis Visibility) []ui.ModuleInfo {
	out := make([]ui.ModuleInfo, 0, len(source))
	for _, m := range source {
		if !isVisibleByPreference(m.ID, vis) {
			continue
		}
		// Modules without handlers (settings, burn, filemanager) are always enabled
		enabled := m.ID == "settings" || m.ID == "burn" || m.ID == "filemanager" || (m.HasHandler && m.DepsAvailable)
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
		return vis.ShowUpscale
	case "author", "rip":
		return vis.ShowDisc
	default:
		return true
	}
}
