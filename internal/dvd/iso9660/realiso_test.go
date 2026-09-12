package iso9660

import (
	"os"
	"testing"
)

// TestRealWorldISO is an optional integration test: set VT_REAL_ISO to a path
// of an ISO 9660-only DVD image (no usable UDF LVD) to verify the reader
// against real authored media. Skipped when the variable is unset.
func TestRealWorldISO(t *testing.T) {
	path := os.Getenv("VT_REAL_ISO")
	if path == "" {
		t.Skip("VT_REAL_ISO not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	r := NewReader(f)
	wantFiles := []string{
		"VIDEO_TS/VIDEO_TS.IFO",
		"VIDEO_TS/VTS_01_0.IFO",
	}
	for _, p := range wantFiles {
		data, err := r.ReadFileData(p)
		if err != nil {
			t.Errorf("ReadFileData(%s): %v", p, err)
			continue
		}
		t.Logf("%s: %d bytes", p, len(data))
	}
	if t.Failed() {
		return
	}
	t.Logf("reader accepted real-world ISO at %s", path)
}