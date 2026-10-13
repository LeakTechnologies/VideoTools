package rip

import (
	"bytes"
	"context"
	"errors"
	"fyne.io/fyne/v2"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/css"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

func TestThumbnailFFmpegProducesRealImage(t *testing.T) {
	if _, err := exec.LookPath(utils.GetFFmpegPath()); err != nil {
		t.Skip("FFmpeg executable is not installed in this test environment")
	}
	data, err := runThumbnailFFmpeg(context.Background(), []string{"-f", "lavfi", "-i", "smptebars=size=720x480:rate=1,setsar=8/9"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != thumbWidth || img.Bounds().Dy() != thumbWidth*3/4 {
		t.Fatalf("thumbnail PNG invalid: %v", err)
	}
}

func TestThumbnailPreparationGateCancelsWaitingSource(t *testing.T) {
	thumbnailPreparationGate <- struct{}{}
	defer func() { <-thumbnailPreparationGate }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := prepareThumbnailSource(ctx, "unused")
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting source preparation ignored retirement")
	}
}

func TestClosedThumbnailBrowserRejectsLateScan(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	cb := NewContentBrowser()
	cb.Close()
	cb.SetScanResult(&DiscScanResult{Titles: []DiscTitle{{Number: 1}}}, "unused")
	if cb.scanResult != nil || cb.thumbnails != nil || cb.ticker != nil {
		t.Fatal("retired view started thumbnail work after a late scan")
	}
	cb.Close()
}

func TestThumbnailPublicationRejectsRetiredScan(t *testing.T) {
	oldContext, oldCancel := context.WithCancel(context.Background())
	defer oldCancel()
	oldSession := &thumbnailSession{ctx: oldContext, cancel: oldCancel}
	currentContext, currentCancel := context.WithCancel(context.Background())
	defer currentCancel()
	currentSession := &thumbnailSession{ctx: currentContext, cancel: currentCancel}
	browser := &ContentBrowser{thumbnails: currentSession}
	card := &titleCardState{previewState: "loading"}
	frame := fyne.NewStaticResource("thumbnail.png", []byte{1})
	if browser.applyThumbnailResult(oldSession, card, frame, "ready") || browser.applyThumbnailResult(oldSession, card, nil, "failed") {
		t.Fatal("retired scan result accepted")
	}
	if len(card.frames) != 0 || card.previewState != "loading" {
		t.Fatal("retired scan changed current frames/status")
	}
	if !browser.applyThumbnailResult(currentSession, card, frame, "ready") || len(card.frames) != 1 || card.previewState != "ready" {
		t.Fatal("current scan result rejected")
	}
	if browser.applyThumbnailResult(currentSession, card, nil, "failed") || card.previewState != "ready" {
		t.Fatal("a failed later still hid a usable preview")
	}
	currentCancel()
	if browser.applyThumbnailResult(currentSession, card, frame, "ready") || len(card.frames) != 1 {
		t.Fatal("canceled scan appended another frame")
	}
}

func TestThumbnailPreparationHonorsCanceledScan(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "VIDEO_TS")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path, cleanup, err := prepareThumbnailSource(ctx, dir)
	if cleanup != nil {
		cleanup()
	}
	if !errors.Is(err, context.Canceled) || path != "" {
		t.Fatalf("canceled source preparation = %q, %v", path, err)
	}
}

func TestThumbnailSampleRejectsMissingVMG(t *testing.T) {
	dir, _ := writeTestVOBSet(t, []uint32{10})
	list, cleanup, err := thumbnailTitleSample(context.Background(), dir, 2)
	if cleanup != nil {
		cleanup()
	}
	if err == nil || list != "" {
		t.Fatal("a VOB alone cannot identify the requested title")
	}
}

func TestThumbnailSampleIsBoundedAndKeepsTitleCells(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{5000})
	payload := make([]byte, 5000*2048)
	// Distinguishable sectors without CSS scrambling flags.
	for sector := 0; sector < 5000; sector++ {
		payload[sector*2048] = byte(sector)
	}
	if err := os.WriteFile(set.Files[0], payload, 0600); err != nil {
		t.Fatal(err)
	}
	info := &ifo.TitleInfo{Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 100, LastSector: 999}, {VOBID: 1, FirstSector: 2000, LastSector: 4999}}}
	name, cleanup, err := thumbnailCellSample(context.Background(), dir, set, info)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	// Skip 390 mapped sectors: start490, cross999 directly to2000.
	want := append(append([]byte{}, payload[490*2048:1000*2048]...), payload[2000*2048:(2000+2048-510)*2048]...)
	if len(data) != thumbnailSampleBytes || !bytes.Equal(data, want) {
		t.Fatal("sample exceeds budget or includes another title's sectors")
	}
	cleanup()
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("sample retained after cleanup")
	}
}

func TestThumbnailSampleRejectsRetiredAndAmbiguousSource(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{10})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	name, cleanup, err := thumbnailCellSample(ctx, dir, set, &ifo.TitleInfo{Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 1, LastSector: 3}}})
	if cleanup != nil {
		cleanup()
	}
	if name != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("retired sample continued")
	}
	for _, info := range []*ifo.TitleInfo{nil, {}, {HasAngles: true, Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 1, LastSector: 3}}}, {Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 1, LastSector: 10}}}} {
		name, cleanup, err := thumbnailCellSample(context.Background(), dir, set, info)
		if cleanup != nil {
			cleanup()
		}
		if name != "" || err == nil {
			t.Fatal("ambiguous sample must be rejected")
		}
	}
}

func TestBoundedTitleSampleProducesRealPreview(t *testing.T) {
	if _, err := exec.LookPath(utils.GetFFmpegPath()); err != nil {
		t.Skip("FFmpeg executable is not installed")
	}
	dir := t.TempDir()
	vob := filepath.Join(dir, "VTS_01_1.VOB")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "smptebars=size=720x480:rate=30", "-t", "2", "-c:v", "mpeg2video", "-g", "12", "-f", "dvd", "-y", vob}
	if out, err := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture encode: %v: %s", err, out)
	}
	sectors := vobSectorCount(vob)
	if sectors == 0 {
		t.Fatal("fixture has no sectors")
	}
	name, cleanup, err := thumbnailCellSample(ctx, dir, VobSet{Name: "VTS_01"}, &ifo.TitleInfo{Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 0, LastSector: sectors - 1}}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := runThumbnailFFmpeg(ctx, []string{"-f", "mpeg", "-i", name}, 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != thumbWidth {
		t.Fatalf("sample preview invalid: %v", err)
	}
}

func TestThumbnailSampleSpansPhysicalSegmentsWithLogicalVOBID(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{10, 10})
	var whole []byte
	for file, path := range set.Files {
		payload := make([]byte, 10*2048)
		for sector := 0; sector < 10; sector++ {
			payload[sector*2048] = byte(file*10 + sector)
		}
		if err := os.WriteFile(path, payload, 0600); err != nil {
			t.Fatal(err)
		}
		whole = append(whole, payload...)
	}
	// A logical VOB id unrelated to the file suffix, and one cell crossing
	// VTS_01_1.VOB into VTS_01_2.VOB, must still read only sectors8..13.
	name, cleanup, err := thumbnailCellSample(context.Background(), dir, set, &ifo.TitleInfo{Cells: []ifo.TitleCell{{VOBID: 42, FirstSector: 8, LastSector: 13}}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, whole[8*2048:14*2048]) {
		t.Fatal("logical title sectors mapped to wrong physical VOB segment")
	}
}

func TestThumbnailCachedVTSKeyDecryptsRepresentativeWithoutSourceReads(t *testing.T) {
	key := [5]byte{1, 2, 3, 4, 5}
	data := make([]byte, css.SectorSize)
	data[0x14] = 0x30
	data[0x80] = 0x7a
	want := append([]byte(nil), data...)
	if err := css.NewDecryptor(key).DecryptSector(want); err != nil {
		t.Fatal(err)
	}
	keys := map[string][5]byte{"VTS_01": key}
	// No source exists: a recovered key for the same scan's VTS must avoid
	// rereading menu/opening sectors and decrypt the supplied sample itself.
	if err := decryptThumbnailSampleWithKeys(context.Background(), filepath.Join(t.TempDir(), "missing"), VobSet{Name: "VTS_01"}, nil, data, keys); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatal("cached key replaced rather than decrypted the representative sample")
	}
	other := make([]byte, css.SectorSize)
	other[0x14] = 0x30
	if err := decryptThumbnailSampleWithKeys(context.Background(), t.TempDir(), VobSet{Name: "VTS_02"}, nil, other, keys); err == nil {
		t.Fatal("key from a different VTS was reused")
	}
}
