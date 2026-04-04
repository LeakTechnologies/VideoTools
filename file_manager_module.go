package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
)

type FileEntry struct {
	Name     string
	Path     string
	Size     int64
	Modified time.Time
	IsDir    bool
	Ext      string
}

func (s *appState) showFileManagerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "filemanager"
	s.maximizeWindow()
	s.setContent(s.buildFileManagerView())
}

type fmState struct {
	s           *appState
	currentPath string
	history     []string
	historyPos  int
	entries     []FileEntry
	fileList    *widget.List
	breadcrumb  *widget.Label
	backBtn     *widget.Button
	forwardBtn  *widget.Button
	upBtn       *widget.Button
	homeBtn     *widget.Button
}

func (s *appState) buildFileManagerView() fyne.CanvasObject {
	t := i18n.T()

	state := &fmState{
		s:           s,
		currentPath: getUserHomeDir(),
		history:     []string{getUserHomeDir()},
		historyPos:  0,
		entries:     []FileEntry{},
	}

	header := widget.NewLabelWithStyle("File Manager", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	state.breadcrumb = widget.NewLabel(state.currentPath)
	state.breadcrumb.TextStyle = fyne.TextStyle{Monospace: true}

	state.backBtn = widget.NewButton("←", state.fmGoBack)
	state.forwardBtn = widget.NewButton("→", state.fmGoForward)
	state.upBtn = widget.NewButton("↑", state.fmGoUp)
	state.homeBtn = widget.NewButton("🏠", state.fmGoHome)

	toolbar := container.NewHBox(state.backBtn, state.forwardBtn, state.upBtn, state.homeBtn)

	state.fileList = widget.NewList(
		func() int { return len(state.entries) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel(""),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			entry := state.entries[id]
			cont := obj.(*fyne.Container)
			icon := getFileIcon(entry)
			cont.Objects[0].(*widget.Label).SetText(icon + " " + entry.Name)
			cont.Objects[1].(*widget.Label).SetText(formatFileSize(entry.Size))
		},
	)

	state.fileList.OnSelected = state.fmOnSelected

	state.fmRefresh()

	cancelBtn := widget.NewButton(t.ActionCancel, func() {
		s.showMainMenu()
	})

	footer := moduleFooter(moduleColor("filemanager"), container.NewHBox(cancelBtn), s.statsBar)

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		state.breadcrumb,
		toolbar,
		state.fileList,
	)
	content = container.NewPadded(content)

	return container.NewBorder(content, footer, nil, nil)
}

func (state *fmState) fmRefresh() {
	newEntries, err := readDir(state.currentPath)
	if err != nil {
		dialog.ShowError(err, state.s.window)
		return
	}
	state.entries = newEntries
	sortFiles(state.entries, "name", true)
	state.breadcrumb.SetText(state.currentPath)
	state.fileList.Refresh()
	state.fmUpdateNav()
}

func (state *fmState) fmUpdateNav() {
	state.backBtn.Disable()
	state.forwardBtn.Disable()
	state.upBtn.Disable()
	if state.historyPos > 0 {
		state.backBtn.Enable()
	}
	if state.historyPos < len(state.history)-1 {
		state.forwardBtn.Enable()
	}
	parent := filepath.Dir(state.currentPath)
	if parent != state.currentPath {
		state.upBtn.Enable()
	}
}

func (state *fmState) fmGoBack() {
	if state.historyPos > 0 {
		state.historyPos--
		state.currentPath = state.history[state.historyPos]
		state.fmRefresh()
	}
}

func (state *fmState) fmGoForward() {
	if state.historyPos < len(state.history)-1 {
		state.historyPos++
		state.currentPath = state.history[state.historyPos]
		state.fmRefresh()
	}
}

func (state *fmState) fmGoUp() {
	parent := filepath.Dir(state.currentPath)
	if parent != state.currentPath {
		state.currentPath = parent
		state.history = append(state.history[:state.historyPos+1], parent)
		state.historyPos++
		state.fmRefresh()
	}
}

func (state *fmState) fmGoHome() {
	state.currentPath = getUserHomeDir()
	state.history = append(state.history[:state.historyPos+1], state.currentPath)
	state.historyPos++
	state.fmRefresh()
}

func (state *fmState) fmOnSelected(id widget.ListItemID) {
	entry := state.entries[id]
	if entry.IsDir {
		state.currentPath = entry.Path
		state.history = append(state.history[:state.historyPos+1], entry.Path)
		state.historyPos++
		state.fmRefresh()
	}
}

func getUserHomeDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		if runtime.GOOS == "windows" {
			home = "C:\\"
		} else {
			home = "/home"
		}
	}
	return home
}

func readDir(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		files = append(files, FileEntry{
			Name:     name,
			Path:     filepath.Join(path, name),
			Size:     info.Size(),
			Modified: info.ModTime(),
			IsDir:    e.IsDir(),
			Ext:      ext,
		})
	}
	return files, nil
}

func sortFiles(entries []FileEntry, by string, asc bool) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		switch by {
		case "name":
			if asc {
				return entries[i].Name < entries[j].Name
			}
			return entries[i].Name > entries[j].Name
		case "size":
			if asc {
				return entries[i].Size < entries[j].Size
			}
			return entries[i].Size > entries[j].Size
		case "modified":
			if asc {
				return entries[i].Modified.Before(entries[j].Modified)
			}
			return entries[i].Modified.After(entries[j].Modified)
		}
		return entries[i].Name < entries[j].Name
	})
}

func getFileIcon(entry FileEntry) string {
	if entry.IsDir {
		return "📁"
	}
	ext := entry.Ext
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm":
		return "🎬"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return "🔊"
	case ".srt", ".ass", ".ssa", ".sub":
		return "📝"
	case ".iso", ".img":
		return "💿"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
		return "🖼️"
	case ".txt", ".md":
		return "📄"
	default:
		return "📄"
	}
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.1f GB", float64(size)/1024/1024/1024)
}
