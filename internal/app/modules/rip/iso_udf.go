package rip

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/udf"
)

// resolveISOWithUDF extracts VIDEO_TS (or BDMV) from an ISO using the native UDF reader.
func resolveISOWithUDF(ctx context.Context, f io.ReadSeeker, isoPath, tempDir string, cleanup func()) (string, func(), error) {
	reader := udf.NewReader(f)
	defer reader.Cleanup()

	// Determine target directory (VIDEO_TS for DVD, BDMV for Blu-ray)
	targetDir := "VIDEO_TS"
	discType, err := reader.DetectDiscType()
	if err == nil && discType == udf.DiscTypeBluRay {
		targetDir = "BDMV"
	}

	if err := reader.ExtractDirectory(ctx, targetDir, tempDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("native extraction failed: %w", err)
	}

	videoTS := filepath.Join(tempDir, targetDir)
	if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
		return videoTS, cleanup, nil
	}
	cleanup()
	return "", nil, fmt.Errorf("%s not found in ISO", targetDir)
}
