package rip

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/iso9660"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// resolveISOWithUDF extracts VIDEO_TS (or BDMV) from an ISO using the native
// UDF reader, falling back to the native ISO 9660 reader when the image has no
// usable UDF volume (grey-market DVDs are often burnt with an ISO 9660
// filesystem plus a broken or dummy UDF bridge written by the burner).
func resolveISOWithUDF(ctx context.Context, f io.ReadSeeker, isoPath, tempDir string, cleanup func()) (string, func(), error) {
	targetDir := "VIDEO_TS"
	udfR := udf.NewReader(f)
	discType, err := udfR.DetectDiscType()
	if err == nil && discType == udf.DiscTypeBluRay {
		targetDir = "BDMV"
	}

	if err := udfR.ExtractDirectory(ctx, targetDir, tempDir); err != nil {
		udfErr := err
		udfR.Cleanup()
		logging.Warning(logging.CatDVD, "UDF extraction of %s failed (%v); falling back to ISO 9660 reader", targetDir, udfErr)

		cleanup()
		if mkErr := os.MkdirAll(tempDir, 0755); mkErr != nil {
			return "", nil, fmt.Errorf("recreate temp dir for ISO 9660 extraction: %w", mkErr)
		}

		isoR := iso9660.NewReader(f)
		isoErr := isoR.ExtractDirectory(ctx, targetDir, tempDir)
		if isoErr != nil {
			logging.Warning(logging.CatDVD, "ISO 9660 extraction of %s also failed: %v", targetDir, isoErr)
			return "", nil, fmt.Errorf("native extraction failed: UDF: %v; ISO 9660: %v", udfErr, isoErr)
		}

		videoTS := filepath.Join(tempDir, targetDir)
		if info, statErr := os.Stat(videoTS); statErr == nil && info.IsDir() {
			logging.Info(logging.CatDVD, "Extracted %s via ISO 9660 fallback from %s (%d files)", targetDir, isoPath, extractFileCount(videoTS))
			return videoTS, cleanup, nil
		}
		return "", nil, fmt.Errorf("%s not found in ISO 9660 image", targetDir)
	}

	videoTS := filepath.Join(tempDir, targetDir)
	if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
		return videoTS, cleanup, nil
	}
	cleanup()
	return "", nil, fmt.Errorf("%s not found in ISO", targetDir)
}

// extractFileCount counts regular files under dir for logging purposes.
func extractFileCount(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}