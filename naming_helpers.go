package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/metadata"
)

func defaultOutputBase(src *videoSource) string {
	if src == nil {
		return "converted"
	}
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	return base
}

func defaultOutputBaseWithSuffix(src *videoSource) string {
	if src == nil {
		return "converted"
	}
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	return base + "-convert"
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
	meta := map[string]string{}
	if src == nil {
		return meta
	}

	meta["filename"] = strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path))
	meta["format"] = src.Format
	meta["codec"] = src.VideoCodec
	if src.Width > 0 && src.Height > 0 {
		meta["width"] = fmt.Sprintf("%d", src.Width)
		meta["height"] = fmt.Sprintf("%d", src.Height)
		meta["resolution"] = fmt.Sprintf("%dx%d", src.Width, src.Height)
	}

	for k, v := range src.Metadata {
		meta[k] = v
	}

	aliasMetadata(meta, "title", "title")
	aliasMetadata(meta, "scene", "title", "comment", "description")
	aliasMetadata(meta, "studio", "studio", "publisher", "label")
	aliasMetadata(meta, "actress", "actress", "performer", "performers", "artist", "actors", "cast")
	aliasMetadata(meta, "series", "series", "album")
	aliasMetadata(meta, "date", "date", "year")

	return meta
}

func aliasMetadata(meta map[string]string, target string, keys ...string) {
	if meta[target] != "" {
		return
	}
	for _, key := range keys {
		if val := meta[strings.ToLower(key)]; strings.TrimSpace(val) != "" {
			meta[target] = val
			return
		}
	}
}
