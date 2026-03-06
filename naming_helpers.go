package main

import (
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/naming"
	"git.leaktechnologies.dev/stu/VideoTools/internal/metadata"
)

func defaultOutputBase(src *videoSource) string {
	return naming.DefaultOutputBase(toNamingSourceInfo(src))
}

func defaultOutputBaseWithSuffix(src *videoSource) string {
	return naming.DefaultOutputBaseWithSuffix(toNamingSourceInfo(src))
}

// resolveOutputBase returns the output base for a source.
// keepExisting preserves manual edits when auto-naming is disabled; it is ignored when auto-naming is on.
func (s *appState) resolveOutputBase(src *videoSource, keepExisting bool) string {
	// Use suffix if AppendSuffix is enabled
	var fallback string
	if s.convert.AppendSuffix {
		fallback = defaultOutputBaseWithSuffix(src)
	} else {
		fallback = defaultOutputBase(src)
	}

	// Auto-naming overrides manual values.
	if s.convert.UseAutoNaming && src != nil && strings.TrimSpace(s.convert.AutoNameTemplate) != "" {
		if name, ok := metadata.RenderTemplate(s.convert.AutoNameTemplate, buildNamingMetadata(src), fallback); ok || name != "" {
			return name
		}
		return fallback
	}

	if keepExisting {
		if base := strings.TrimSpace(s.convert.OutputBase); base != "" {
			return base
		}
	}
	return fallback
}

func buildNamingMetadata(src *videoSource) map[string]string {
	return naming.BuildNamingMetadata(toNamingSourceInfo(src))
}

func toNamingSourceInfo(src *videoSource) *naming.SourceInfo {
	if src == nil {
		return nil
	}
	return &naming.SourceInfo{
		DisplayName: src.DisplayName,
		Path:        src.Path,
		Format:      src.Format,
		VideoCodec:  src.VideoCodec,
		Width:       src.Width,
		Height:      src.Height,
		Metadata:    src.Metadata,
	}
}
