package disc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
)

func writeMinimalDVD(t *testing.T, regionMask uint32) string {
	t.Helper()
	root := t.TempDir()
	videoTS := filepath.Join(root, "VIDEO_TS")
	if err := os.MkdirAll(videoTS, 0755); err != nil {
		t.Fatal(err)
	}
	mat := ifo.NewVMGMAT()
	mat.VMG_Category = regionMask
	srpt := &ifo.TT_SRPT{
		NumTitles: 2,
		Titles: []ifo.TitleSearchPointer{
			{VTSNumber: 1, VTS_TitleNumber: 1, NumChapters: 3},
			{VTSNumber: 1, VTS_TitleNumber: 2, NumChapters: 1},
		},
	}
	if err := ifo.NewBuilder(videoTS).GenerateVMG_IFO(mat, srpt, nil, nil); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestScanSourceFolderAndJSONContract(t *testing.T) {
	root := writeMinimalDVD(t, 0x01)
	var notes []string
	result, err := ScanSource(context.Background(), root, ScanOptions{
		OnNote: func(note string) { notes = append(notes, note) },
	})
	if err != nil {
		t.Fatalf("ScanSource: %v", err)
	}
	if result.SourceKind != SourceFolder {
		t.Fatalf("SourceKind = %q, want %q", result.SourceKind, SourceFolder)
	}
	if len(result.Titles) != 2 {
		t.Fatalf("titles = %d, want 2", len(result.Titles))
	}
	if result.RegionCode != 1 || result.Region != "Region 1" {
		t.Fatalf("region = %d %q, want 1 Region 1", result.RegionCode, result.Region)
	}
	if result.MainTitleNumber != 1 {
		t.Fatalf("main title = %d, want 1", result.MainTitleNumber)
	}
	if len(notes) == 0 {
		t.Fatal("expected scan notes")
	}
}

func TestResolveSourceVideoTSFile(t *testing.T) {
	root := writeMinimalDVD(t, 0)
	kind, videoTS, err := ResolveSource(filepath.Join(root, "VIDEO_TS", "VIDEO_TS.IFO"))
	if err != nil {
		t.Fatalf("ResolveSource: %v", err)
	}
	if kind != SourceFile || videoTS != filepath.Join(root, "VIDEO_TS") {
		t.Fatalf("resolved = %q %q", kind, videoTS)
	}
}

func TestResolveSourceRejectsUnsupported(t *testing.T) {
	if _, _, err := ResolveSource(filepath.Join(t.TempDir(), "not-a-disc.iso")); err == nil {
		t.Fatal("unsupported source should fail")
	}
}

func TestClassifyDiscType(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{100_000_000, "MiniDVD"},
		{4_000_000_000, "DVD-5"},
		{8_000_000_000, "DVD-9"},
		{8_700_000_000, "DVD-10"},
		{14_000_000_000, "DVD-18"},
		{20_000_000_000, ""},
	}
	for _, tc := range cases {
		if got := classifyDiscType(tc.size); got != tc.want {
			t.Errorf("classifyDiscType(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}
