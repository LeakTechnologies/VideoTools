package rip

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/LeakTechnologies/VideoTools/internal/queue"
)

var namingSequence atomic.Uint64

// ProjectNaming is UI-thread-owned state, shared across Rip view rebuilds.
type ProjectNaming struct {
	Source       string
	DiscTitle    string
	OutputNames  map[int]string
	TitleCleared bool
	revision     uint64
	scope        string
	onReset      func()
}

func NewProjectNaming() *ProjectNaming {
	return &ProjectNaming{OutputNames: make(map[int]string), revision: namingSequence.Add(1), scope: rand.Text()}
}

func (n *ProjectNaming) SetSource(source string) {
	if n.Source == source {
		return
	}
	n.Source = source
	n.DiscTitle = ""
	n.OutputNames = make(map[int]string)
	n.TitleCleared = false
	n.revision = namingSequence.Add(1)
	if n.onReset != nil {
		n.onReset()
	}
}

func (n *ProjectNaming) SetDiscTitle(title string) {
	title = strings.TrimSpace(title)
	if n.DiscTitle == title {
		return
	}
	n.DiscTitle = title
	n.TitleCleared = false
	n.revision = namingSequence.Add(1)
}

func (n *ProjectNaming) SetOutputName(number int, name string) {
	if n.OutputNames[number] == name {
		return
	}
	n.OutputNames[number] = name
	n.revision = namingSequence.Add(1)
}

func (n *ProjectNaming) Token() string        { return fmt.Sprintf("%s-%d", n.scope, n.revision) }
func (n *ProjectNaming) SetOnReset(fn func()) { n.onReset = fn }

// Called only for a successful verified rip, dispatched on the UI thread.
func (n *ProjectNaming) ResetIfCurrent(token string) bool {
	if token == "" || token != n.Token() {
		return false
	}
	n.DiscTitle = ""
	n.TitleCleared = true
	n.revision = namingSequence.Add(1)
	if n.onReset != nil {
		n.onReset()
	}
	return true
}

func NamedTitleOutputPath(outputPath, discTitle, titleName string, titleNumber int) string {
	if titleNumber < 1 {
		titleNumber = 1
	}
	titleName = SanitizeForPath(titleName)
	if titleName == "" {
		titleName = fmt.Sprintf("Title_%02d", titleNumber)
	}
	if disc := SanitizeForPath(discTitle); disc != "" {
		titleName += "_" + disc
	}
	return filepath.Join(filepath.Dir(outputPath), titleName+filepath.Ext(outputPath))
}

func allocateNamedRipPath(path string, reserved map[string]bool) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for suffix := 1; ; suffix++ {
		candidate := path
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d%s", base, suffix, ext)
		}
		candidate = UniqueFilePath(candidate)
		key := strings.ToLower(filepath.Clean(candidate))
		if !reserved[key] {
			reserved[key] = true
			return candidate
		}
	}
}

func prepareNamedChapterJobs(job *queue.Job, naming *ProjectNaming, ranges []ChapterSelectionRange, reserved map[string]bool) []*queue.Job {
	number, _ := job.Config["titleNumber"].(int)
	job.OutputFile = NamedTitleOutputPath(job.OutputFile, naming.DiscTitle, naming.OutputNames[number], number)
	job.Config["outputPath"] = job.OutputFile
	job.Title = strings.TrimSuffix(filepath.Base(job.OutputFile), filepath.Ext(job.OutputFile))
	job.Config["ripProjectToken"] = naming.Token()
	jobs := chapterOutputJobs(job, ranges)
	for _, output := range jobs {
		output.OutputFile = allocateNamedRipPath(output.OutputFile, reserved)
		output.Config["outputPath"] = output.OutputFile
		output.Description = output.OutputFile
	}
	return jobs
}
