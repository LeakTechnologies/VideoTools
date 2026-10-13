package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/css"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/nav"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
)

// prepareDVDTitle stages only title payloads, not interactive menu domains.
// Cancellation is checked between bounded reads; no source file is modified.
func prepareDVDTitle(ctx context.Context, source string, title int) (root string, selected int, cleanup func(), err error) {
	if err = ctx.Err(); err != nil {
		return
	}
	scratch, err := os.MkdirTemp("", "vt-player-dvd-*")
	if err != nil {
		return "", 0, nil, err
	}
	var once sync.Once
	cleanup = func() { once.Do(func() { _ = os.RemoveAll(scratch) }) }
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	sourceRoot := source
	if strings.EqualFold(filepath.Ext(source), ".iso") {
		f, openErr := os.Open(source)
		if openErr != nil {
			err = openErr
			return
		}
		reader := udf.NewReader(f)
		extractErr := reader.ExtractDirectory(ctx, "VIDEO_TS", filepath.Join(scratch, "input", "VIDEO_TS"))
		f.Close()
		if extractErr != nil {
			reader.Cleanup()
			err = fmt.Errorf("DVD ISO extraction: %w", extractErr)
			return
		}
		sourceRoot = filepath.Join(scratch, "input")
	}
	if strings.EqualFold(filepath.Base(sourceRoot), "VIDEO_TS") {
		sourceRoot = filepath.Dir(sourceRoot)
	}
	topo, analyseErr := nav.AnalyseDisc(sourceRoot)
	if analyseErr != nil {
		err = analyseErr
		return
	}
	selected = title
	if selected == 0 {
		bestDuration := -1.0
		for _, candidate := range topo.Titles {
			if candidate.Duration > bestDuration {
				selected, bestDuration = candidate.Number, candidate.Duration
			}
		}
	}
	vts := 0
	for _, t := range topo.Titles {
		if t.Number == selected {
			vts = t.VTSNumber
			break
		}
	}
	if vts == 0 {
		err = fmt.Errorf("DVD title %d unavailable", selected)
		return
	}
	srcDir := filepath.Join(sourceRoot, "VIDEO_TS")
	entries, readErr := os.ReadDir(srcDir)
	if readErr != nil {
		err = readErr
		return
	}
	root = filepath.Join(scratch, "playback")
	dstDir := filepath.Join(root, "VIDEO_TS")
	if err = os.MkdirAll(dstDir, 0700); err != nil {
		return
	}
	prefix := fmt.Sprintf("VTS_%02d_", vts)
	var payloads []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		upper := strings.ToUpper(name)
		if strings.HasSuffix(upper, ".IFO") || strings.HasSuffix(upper, ".BUP") {
			if err = copyDVDMetadata(ctx, filepath.Join(srcDir, name), filepath.Join(dstDir, name)); err != nil {
				return
			}
		} else if strings.HasPrefix(upper, prefix) && strings.HasSuffix(upper, ".VOB") && upper != prefix+"0.VOB" {
			payloads = append(payloads, name)
		}
	}
	if len(payloads) == 0 {
		err = fmt.Errorf("DVD title %d has no readable VOB payloads", selected)
		return
	}
	sort.Strings(payloads)
	var titleKey *css.Decryptor
	for _, name := range payloads {
		if err = copyDVDTitleVOBWithKey(ctx, filepath.Join(srcDir, name), filepath.Join(dstDir, name), scratch, &titleKey); err != nil {
			return
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	return
}

func copyDVDMetadata(ctx context.Context, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	buf := make([]byte, 64*1024)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err = out.Write(buf[:n]); err != nil {
				break
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = readErr
			break
		}
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func dvdSectorScrambled(sec []byte) bool {
	return len(sec) == css.SectorSize && sec[0] == 0 && sec[1] == 0 && sec[2] == 1 && sec[3] == 0xba && sec[0x11] != 0xbb && sec[0x11] != 0xbe && sec[0x11] != 0xbf && css.IsScrambledSector(sec)
}

func recoverDVDFileKey(ctx context.Context, in *os.File, offset int64, scratch string) (*css.Decryptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Begin at an observed scrambled PES, avoiding the primitive's clear-lead cutoff.
	sample := make([]byte, 2*1024*1024)
	n, err := in.ReadAt(sample, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	probe, err := os.CreateTemp(scratch, "key-sample-*")
	if err != nil {
		return nil, err
	}
	name := probe.Name()
	defer os.Remove(name)
	if _, err = probe.Write(sample[:n]); err != nil {
		probe.Close()
		return nil, err
	}
	if err = probe.Close(); err != nil {
		return nil, err
	}
	key, err := css.CrackTitleKeyFiles([]string{name})
	if err != nil {
		return nil, fmt.Errorf("DVD CSS title-key recovery: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return css.NewDecryptor(key), nil
}

func copyDVDTitleVOB(ctx context.Context, src, dst, scratch string) error {
	return copyDVDTitleVOBWithKey(ctx, src, dst, scratch, nil)
}
func copyDVDTitleVOBWithKey(ctx context.Context, src, dst, scratch string, shared **css.Decryptor) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	reader := bufio.NewReaderSize(in, 1<<20)
	writer := bufio.NewWriterSize(out, 1<<20)
	var decryptor *css.Decryptor
	if shared != nil {
		decryptor = *shared
	}
	sec := make([]byte, css.SectorSize)
	for offset := int64(0); ; offset += css.SectorSize {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, readErr := io.ReadFull(reader, sec)
		if readErr == io.EOF {
			if err := writer.Flush(); err != nil {
				return err
			}
			return out.Close()
		}
		if readErr != nil {
			return fmt.Errorf("DVD title sector read at %d: %w", offset, readErr)
		}
		if dvdSectorScrambled(sec) {
			if decryptor == nil {
				decryptor, err = recoverDVDFileKey(ctx, in, offset, scratch)
				if err != nil {
					return err
				}
				if shared != nil {
					*shared = decryptor
				}
			}
			if err = decryptor.DecryptSector(sec); err != nil {
				return err
			}
			if dvdSectorScrambled(sec) {
				return fmt.Errorf("DVD sector remains scrambled after decryption")
			}
		}
		if _, err = writer.Write(sec[:n]); err != nil {
			return err
		}
	}
}

// Route automatic disc opens through the same preparation as explicit LoadDVD.
func isDVDPlayerSource(path string) bool {
	if strings.EqualFold(filepath.Ext(path), ".iso") {
		return true
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	root := path
	if !strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
		root = filepath.Join(path, "VIDEO_TS")
	}
	ifo, err := os.Stat(filepath.Join(root, "VIDEO_TS.IFO"))
	return err == nil && !ifo.IsDir()
}
