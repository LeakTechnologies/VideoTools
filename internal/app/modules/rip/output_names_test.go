package rip

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/queue"
)

func TestProjectTitleResetRejectsOldCompletion(t *testing.T) {
	n := NewProjectNaming()
	n.SetSource("first.iso")
	n.SetDiscTitle("Disc A")
	token := n.Token()
	n.SetDiscTitle("Disc B")
	if n.ResetIfCurrent(token) || n.DiscTitle != "Disc B" {
		t.Fatal("old completion cleared edited title")
	}
	token = n.Token()
	n.SetSource("second.iso")
	n.SetDiscTitle("Disc C")
	if n.ResetIfCurrent(token) {
		t.Fatal("old completion cleared a different project")
	}
	token = n.Token()
	resets := 0
	n.SetOnReset(func() { resets++ })
	if !n.ResetIfCurrent(token) || n.DiscTitle != "" || !n.TitleCleared || resets != 1 {
		t.Fatal("current success must clear and notify exactly once")
	}
	if n.ResetIfCurrent(token) || n.ResetIfCurrent("") {
		t.Fatal("repeat/legacy completion must not reset")
	}
	n.SetSource("second.iso")
	if !n.TitleCleared {
		t.Fatal("view rebuild must preserve cleared state")
	}
	n.SetSource("third.iso")
	if n.TitleCleared || len(n.OutputNames) != 0 {
		t.Fatal("new source must start a fresh naming project")
	}
	if NewProjectNaming().ResetIfCurrent(n.Token()) {
		t.Fatal("completion token must not cross sessions/projects")
	}
}

func TestNamedRipOutputsRespectDirectoryAndChapterSuffixes(t *testing.T) {
	dir := t.TempDir()
	n := NewProjectNaming()
	n.SetDiscTitle("Disc Title")
	n.SetOutputName(1, "Opening")
	job := &queue.Job{OutputFile: filepath.Join(dir, "ignored.mkv"), Config: map[string]interface{}{"titleNumber": 1, "embedChapters": true, "discTitle": n.DiscTitle}}
	ranges, _ := ParseChapterSelection("1-3, 5-6, 8")
	reserved := map[string]bool{}
	jobs := prepareNamedChapterJobs(job, n, ranges, reserved)
	wants := []string{"Opening_Disc Title_Chapters_01-03.mkv", "Opening_Disc Title_Chapters_05-06.mkv", "Opening_Disc Title_Chapter_08.mkv"}
	for i, got := range jobs {
		if got.OutputFile != filepath.Join(dir, wants[i]) || got.Config["outputPath"] != got.OutputFile || got.Config["ripProjectToken"] != n.Token() {
			t.Fatalf("output %d: %+v", i, got)
		}
	}
	before := jobs[0].OutputFile
	n.ResetIfCurrent(n.Token())
	if jobs[0].OutputFile != before || jobs[0].Config["discTitle"] != "Disc Title" {
		t.Fatal("reset changed queued snapshots")
	}
	if got := NamedTitleOutputPath(filepath.Join(dir, "base.mp4"), "Disc/Title", "../Scene:One", 2); filepath.Dir(got) != dir || filepath.Ext(got) != ".mp4" {
		t.Fatal("name escaped output directory or changed format")
	}
	if got := filepath.Base(NamedTitleOutputPath(filepath.Join(dir, "base.mkv"), "", "", 3)); got != "Title_03.mkv" {
		t.Fatalf("empty-name fallback: %s", got)
	}
	path := filepath.Join(dir, "exists.mkv")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	first := allocateNamedRipPath(path, reserved)
	second := allocateNamedRipPath(path, reserved)
	if first == path || second == path || first == second {
		t.Fatal("existing/queued file collision was not avoided")
	}
}
