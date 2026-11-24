package disc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSource classifies a disc source and returns the VIDEO_TS directory for
// folder-like sources. ISO sources are handled by ScanSource directly because
// scanning reads only the IFO files; a future rip service will use the existing
// full-extraction path.
func ResolveSource(path string) (SourceKind, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", serviceError("source_not_found", "source not found", err)
	}
	if info.IsDir() {
		if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
			return SourceVideoTS, path, nil
		}
		videoTS := filepath.Join(path, "VIDEO_TS")
		if fi, err := os.Stat(videoTS); err == nil && fi.IsDir() {
			return SourceFolder, videoTS, nil
		}
		return "", "", serviceError("no_video_ts", fmt.Sprintf("no VIDEO_TS folder found in %s", path), nil)
	}

	if strings.EqualFold(filepath.Ext(path), ".iso") {
		return SourceISO, path, nil
	}

	// File dialogs may return an IFO/VOB from inside VIDEO_TS.
	dir := filepath.Dir(path)
	if strings.EqualFold(filepath.Base(dir), "VIDEO_TS") {
		return SourceFile, dir, nil
	}
	videoTS := filepath.Join(dir, "VIDEO_TS")
	if fi, err := os.Stat(videoTS); err == nil && fi.IsDir() {
		return SourceFile, videoTS, nil
	}
	return "", "", serviceError("unsupported_source", fmt.Sprintf("unsupported source: %s", path), nil)
}

// ScanSource resolves and scans a DVD folder, ISO or VIDEO_TS file source.
func ScanSource(ctx context.Context, path string, opts ScanOptions) (*ScanResult, error) {
	kind, resolved, err := ResolveSource(path)
	if err != nil {
		return nil, err
	}
	if kind == SourceISO {
		return scanISO(ctx, resolved, opts)
	}
	return scanVideoTS(resolved, kind, path, opts)
}
