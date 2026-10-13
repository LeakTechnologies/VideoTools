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
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

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

func TestThumbnailCellConcatKeepsOnlyRequestedTitle(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{10})
	payload := make([]byte, 10*2048)
	for sector := 0; sector < 10; sector++ {
		for i := sector * 2048; i < (sector+1)*2048; i++ {
			payload[i] = byte(sector)
		}
	}
	if err := os.WriteFile(set.Files[0], payload, 0644); err != nil {
		t.Fatal(err)
	}
	list, cleanup, err := thumbnailCellConcat(dir, set, &ifo.TitleInfo{Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 3, LastSector: 4}, {VOBID: 1, FirstSector: 7, LastSector: 7}}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	contents, err := os.ReadFile(list)
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	var slices []string
	for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		name := strings.TrimSuffix(strings.TrimPrefix(line, "file '"), "'")
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, data...)
		slices = append(slices, name)
	}
	want := append(append([]byte(nil), payload[3*2048:5*2048]...), payload[7*2048:8*2048]...)
	if !bytes.Equal(got, want) {
		t.Fatal("thumbnail fallback included content from outside the requested title")
	}
	cleanup()
	for _, name := range append(slices, list) {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("scratch file retained: %s", filepath.Base(name))
		}
	}
}

func TestThumbnailCellConcatRejectsAmbiguousOrInvalidBounds(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{10})
	for _, info := range []*ifo.TitleInfo{nil, {}, {HasAngles: true, Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 1, LastSector: 2}}}, {Cells: []ifo.TitleCell{{VOBID: 1, FirstSector: 0, LastSector: 10}}}, {Cells: []ifo.TitleCell{{VOBID: 2, FirstSector: 0, LastSector: 1}}}} {
		list, cleanup, err := thumbnailCellConcat(dir, set, info)
		if cleanup != nil {
			cleanup()
		}
		if err == nil || list != "" {
			t.Fatal("ambiguous title must not fall back to whole VTS content")
		}
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

func TestThumbnailFallbackRejectsMissingVMG(t *testing.T) {
	dir, _ := writeTestVOBSet(t, []uint32{10})
	list, cleanup, err := thumbnailTitleConcat(dir, 2)
	if cleanup != nil {
		cleanup()
	}
	if err == nil || list != "" {
		t.Fatal("a VOB alone cannot identify the requested title")
	}
}
