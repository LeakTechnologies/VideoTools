package encoding_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestSourceTreeHasNoMojibakeSignatures guards the public tree against the
// encoding damage that survives compilation and tests. The signatures are
// written as escapes so this file cannot trip on its own source.
func TestSourceTreeHasNoMojibakeSignatures(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate guard source")
	}
	file, err := filepath.Abs(file)
	if err != nil {
		t.Fatalf("absolute guard path: %v", err)
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root %q: %v", root, err)
	}

	skipDirs := map[string]bool{
		".git":         true,
		".opencode":    true,
		"_fyne":        true,
		"bin":          true,
		"build":        true,
		"coverage":     true,
		"dist":         true,
		"node_modules": true,
		"target":       true,
		"vendor":       true,
	}
	skipExt := map[string]bool{
		".a": true, ".appx": true, ".bin": true, ".dat": true, ".dll": true,
		".dylib": true, ".exe": true, ".gif": true, ".gz": true, ".ico": true,
		".jpeg": true, ".jpg": true, ".lib": true, ".mkv": true, ".mp4": true,
		".msi": true, ".msix": true, ".o": true, ".otf": true, ".pdf": true,
		".png": true, ".so": true, ".syso": true, ".ttf": true, ".wav": true,
		".woff": true, ".woff2": true, ".zip": true,
	}

	var findings []string
	add := func(path string, line, column int, message string) {
		if len(findings) >= 40 {
			return
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		findings = append(findings, fmt.Sprintf("%s:%d:%d %s", rel, line, column, message))
	}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if skipExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 5<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		if !utf8.Valid(data) {
			add(path, 1, 1, "invalid UTF-8")
			return nil
		}

		for lineIndex, line := range strings.Split(string(data), "\n") {
			runes := []rune(strings.TrimSuffix(line, "\r"))
			for column, r := range runes {
				switch r {
				case '\ufffd':
					add(path, lineIndex+1, column+1, "U+FFFD replacement character")
				case '\u00c3':
					add(path, lineIndex+1, column+1, "U+00C3 encoding signature")
				case '\u00c2':
					add(path, lineIndex+1, column+1, "U+00C2 encoding signature")
				case '\u00e2':
					if column+1 < len(runes) && runes[column+1] == '\u20ac' {
						add(path, lineIndex+1, column+1, "U+00E2 U+20AC encoding signature")
					}
					if column+1 < len(runes) && runes[column+1] == '\u2020' {
						add(path, lineIndex+1, column+1, "U+00E2 U+2020 partial-arrow signature")
					}
				case '\u00f0':
					if column+1 < len(runes) && runes[column+1] == '\u0178' {
						add(path, lineIndex+1, column+1, "U+00F0 U+0178 partial-emoji signature")
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	if len(findings) > 0 {
		t.Fatalf("found %d encoding signature(s):\n%s", len(findings), strings.Join(findings, "\n"))
	}
}
