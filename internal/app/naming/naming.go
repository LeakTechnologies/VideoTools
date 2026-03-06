package naming

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SourceInfo struct {
	DisplayName string
	Path        string
	Format      string
	VideoCodec  string
	Width       int
	Height      int
	Metadata    map[string]string
}

func DefaultOutputBase(src *SourceInfo) string {
	if src == nil {
		return "converted"
	}
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	return base
}

func DefaultOutputBaseWithSuffix(src *SourceInfo) string {
	if src == nil {
		return "converted"
	}
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	return base + "-convert"
}

func BuildNamingMetadata(src *SourceInfo) map[string]string {
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
