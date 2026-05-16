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
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
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
	backBtn     *ui.PillButton
	forwardBtn  *ui.PillButton
	upBtn       *ui.PillButton
	homeBtn     *ui.PillButton
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

	header := widget.NewLabelWithStyle(t.ModuleFileManager, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	state.breadcrumb = widget.NewLabel(state.currentPath)
	state.breadcrumb.TextStyle = fyne.TextStyle{Monospace: true}

	state.backBtn = ui.MakePillButton("←", ui.BorderDim, state.fmGoBack)
	state.forwardBtn = ui.MakePillButton("→", ui.BorderDim, state.fmGoForward)
	state.upBtn = ui.MakePillButton("↑", ui.BorderDim, state.fmGoUp)
	state.homeBtn = ui.MakePillButton("🏠", ui.BorderDim, state.fmGoHome)

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

	cancelBtn := ui.MakePillButton(t.ActionCancel, ui.BorderDim, func() {
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
		return
	}
	state.fmShowContextMenu(entry, id)
}

func (state *fmState) fmShowContextMenu(entry FileEntry, id widget.ListItemID) {
	t := i18n.T()
	menu := fyne.NewMenu("")

	ext := entry.Ext
	isVideo := isVideoFile(ext)
	isAudio := isAudioFile(ext)
	isSubtitle := isSubtitleFile(ext)
	isDVD := isDVDFile(ext)

	if isVideo {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label: t.FileManagerOpenConvert,
			Action: func() {
				state.openInModule("convert", entry.Path)
			},
		})
	}

	if isAudio {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label: t.FileManagerOpenAudio,
			Action: func() {
				state.openInModule("audio", entry.Path)
			},
		})
	}

	if isSubtitle {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label: t.FileManagerOpenSubtitles,
			Action: func() {
				state.openInModule("subtitles", entry.Path)
			},
		})
	}

	if isDVD {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label: t.FileManagerOpenAuthor,
			Action: func() {
				state.openInModule("author", entry.Path)
			},
		})
	}

	menu.Items = append(menu.Items, &fyne.MenuItem{
		Label: t.FileManagerOpenInspect,
		Action: func() {
			state.openInModule("inspect", entry.Path)
		},
	})

	pop := widget.NewPopUpMenu(menu, state.s.window.Canvas())
	pop.Show()
}

func (state *fmState) openInModule(module, path string) {
	switch module {
	case "convert":
		state.s.source = &videoSource{Path: path, DisplayName: filepath.Base(path)}
		state.s.showConvertView(state.s.source)
	case "audio":
		state.s.showAudioView()
	case "subtitles":
		state.s.showSubtitlesView()
	case "author":
		state.s.showAuthorView()
	case "inspect":
		state.s.showInspectView()
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

var videoExts = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true, ".webm": true, ".ts": true, ".m2ts": true, ".vob": true, ".mpg": true}
var audioExts = map[string]bool{".mp3": true, ".aac": true, ".flac": true, ".wav": true, ".ogg": true, ".m4a": true, ".opus": true}
var subtitleExts = map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true}
var dvdExts = map[string]bool{".iso": true, ".img": true}

func isVideoFile(ext string) bool {
	return videoExts[ext]
}

func isAudioFile(ext string) bool {
	return audioExts[ext]
}

func isSubtitleFile(ext string) bool {
	return subtitleExts[ext]
}

func isDVDFile(ext string) bool {
	return dvdExts[ext]
}
