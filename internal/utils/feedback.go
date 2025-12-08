package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FeedbackBundler struct{}

func NewFeedbackBundler() *FeedbackBundler {
	return &FeedbackBundler{}
}

// Bundle collects the provided files and a user note into a zip written to destDir.
// Returns the created path.
func (fb *FeedbackBundler) Bundle(destDir string, userNote string, files ...string) (string, error) {
	if strings.TrimSpace(destDir) == "" {
		destDir = "."
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("make dir: %w", err)
	}
	ts := time.Now().Format("20060102-150405")
	zipPath := filepath.Join(destDir, fmt.Sprintf("feedback-%s.zip", ts))
	zf, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}
	defer zf.Close()

	zipw := zip.NewWriter(zf)
	defer zipw.Close()

	if strings.TrimSpace(userNote) != "" {
		if w, err := zipw.Create("note.txt"); err == nil {
			_, _ = w.Write([]byte(userNote))
		}
	}

	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		src, err := os.Open(f)
		if err != nil {
			continue
		}
		defer src.Close()
		w, err := zipw.Create(filepath.Base(f))
		if err != nil {
			continue
		}
		if _, err := io.Copy(w, src); err != nil {
			continue
		}
	}
	return zipPath, nil
}
