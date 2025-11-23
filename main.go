package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/player"
	"github.com/hajimehoshi/oto"
)

// Module describes a high level tool surface that gets a tile on the menu.
type Module struct {
	ID     string
	Label  string
	Color  color.Color
	Handle func(files []string)
}

var (
	debugFlag = flag.Bool("debug", false, "enable verbose logging (env: VIDEOTOOLS_DEBUG=1)")

	backgroundColor = mustHex("#0B0F1A")
	gridColor       = mustHex("#171C2A")
	textColor       = mustHex("#E1EEFF")
	queueColor      = mustHex("#5961FF")

	modules = []Module{
		{"convert", "Convert", mustHex("#5E2AE2"), handleConvert},
		{"merge", "Merge", mustHex("#3852F3"), handleMerge},
		{"trim", "Trim", mustHex("#1B87F4"), handleTrim},
		{"filters", "Filters", mustHex("#1FC4D0"), handleFilters},
		{"upscale", "Upscale", mustHex("#3FD777"), handleUpscale},
		{"audio", "Audio", mustHex("#9CE33D"), handleAudio},
		{"thumb", "Thumb", mustHex("#F0C33E"), handleThumb},
		{"inspect", "Inspect", mustHex("#F69A3F"), handleInspect},
	}
)

var (
	logFilePath  string
	logFile      *os.File
	logHistory   []string
	debugEnabled bool
	debugLogger  = log.New(os.Stderr, "[videotools] ", log.LstdFlags|log.Lmicroseconds)
)

const logHistoryMax = 500

type logCategory string

const (
	logCatUI     logCategory = "[UI]"
	logCatCLI    logCategory = "[CLI]"
	logCatFFMPEG logCategory = "[FFMPEG]"
	logCatSystem logCategory = "[SYS]"
	logCatModule logCategory = "[MODULE]"
)

type formatOption struct {
	Label      string
	Ext        string
	VideoCodec string
}

var formatOptions = []formatOption{
	{"MP4 (H.264)", ".mp4", "libx264"},
	{"MKV (H.265)", ".mkv", "libx265"},
	{"MOV (ProRes)", ".mov", "prores_ks"},
}

type convertConfig struct {
	OutputBase       string
	SelectedFormat   formatOption
	Quality          string
	Mode             string
	InverseTelecine  bool
	InverseAutoNotes string
	CoverArtPath     string
	AspectHandling   string
	OutputAspect     string
}

func (c convertConfig) OutputFile() string {
	base := strings.TrimSpace(c.OutputBase)
	if base == "" {
		base = "converted"
	}
	return base + c.SelectedFormat.Ext
}

func (c convertConfig) CoverLabel() string {
	if strings.TrimSpace(c.CoverArtPath) == "" {
		return "Cover: none"
	}
	return fmt.Sprintf("Cover: %s", filepath.Base(c.CoverArtPath))
}

type appState struct {
	window        fyne.Window
	active        string
	source        *videoSource
	anim          *previewAnimator
	convert       convertConfig
	currentFrame  string
	player        player.Controller
	playerReady   bool
	playerVolume  float64
	playerMuted   bool
	lastVolume    float64
	playerPaused  bool
	playerPos     float64
	playerLast    time.Time
	progressQuit  chan struct{}
	convertCancel context.CancelFunc
	playerSurf    *playerSurface
	convertBusy   bool
	convertStatus string
	playSess      *playSession
}

func (s *appState) stopPreview() {
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
}

type playerSurface struct {
	obj           fyne.CanvasObject
	width, height int
}

func (s *appState) setPlayerSurface(obj fyne.CanvasObject, w, h int) {
	s.playerSurf = &playerSurface{obj: obj, width: w, height: h}
	s.syncPlayerWindow()
}

func (s *appState) currentPlayerPos() float64 {
	if s.playerPaused {
		return s.playerPos
	}
	return s.playerPos + time.Since(s.playerLast).Seconds()
}

func (s *appState) stopProgressLoop() {
	if s.progressQuit != nil {
		close(s.progressQuit)
		s.progressQuit = nil
	}
}

func (s *appState) startProgressLoop(maxDur float64, slider *widget.Slider, update func(float64)) {
	s.stopProgressLoop()
	stop := make(chan struct{})
	s.progressQuit = stop
	ticker := time.NewTicker(200 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				pos := s.currentPlayerPos()
				if pos < 0 {
					pos = 0
				}
				if pos > maxDur {
					pos = maxDur
				}
				if update != nil {
					update(pos)
				}
				if slider != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						slider.SetValue(pos)
					}, false)
				}
			}
		}
	}()
}

func (s *appState) syncPlayerWindow() {
	if s.player == nil || s.playerSurf == nil || s.playerSurf.obj == nil {
		return
	}
	driver := fyne.CurrentApp().Driver()
	pos := driver.AbsolutePositionForObject(s.playerSurf.obj)
	width := s.playerSurf.width
	height := s.playerSurf.height
	if width <= 0 || height <= 0 {
		return
	}
	s.player.SetWindow(int(pos.X), int(pos.Y), width, height)
	debugLog(logCatUI, "player window target pos=(%d,%d) size=%dx%d", int(pos.X), int(pos.Y), width, height)
}

func (s *appState) startPreview(frames []string, img *canvas.Image, slider *widget.Slider) {
	if len(frames) == 0 {
		return
	}
	anim := &previewAnimator{frames: frames, img: img, slider: slider, stop: make(chan struct{}), playing: true, state: s}
	s.anim = anim
	anim.Start()
}

func (s *appState) hasSource() bool {
	return s.source != nil
}

func (s *appState) applyInverseDefaults(src *videoSource) {
	if src == nil {
		return
	}
	if src.IsProgressive() {
		s.convert.InverseTelecine = false
		s.convert.InverseAutoNotes = "Progressive source detected; inverse telecine disabled."
	} else {
		s.convert.InverseTelecine = true
		s.convert.InverseAutoNotes = "Interlaced source detected; smoothing enabled."
	}
}

func (s *appState) setContent(body fyne.CanvasObject) {
	bg := canvas.NewRectangle(backgroundColor)
	bg.SetMinSize(fyne.NewSize(920, 540))
	if body == nil {
		s.window.SetContent(bg)
		return
	}
	s.window.SetContent(container.NewMax(bg, body))
}

func (s *appState) showMainMenu() {
	s.stopPreview()
	s.stopPlayer()
	s.active = ""
	s.setContent(container.NewPadded(buildMainMenu(s)))
}

func (s *appState) showModule(id string) {
	switch id {
	case "convert":
		s.showConvertView(nil)
	default:
		debugLog(logCatUI, "UI module %s not wired yet", id)
	}
}

func (s *appState) showConvertView(file *videoSource) {
	s.stopPreview()
	s.active = "convert"
	if file != nil {
		s.source = file
	}
	if s.source == nil {
		s.convert.OutputBase = "converted"
		s.convert.CoverArtPath = ""
		s.convert.AspectHandling = "Auto"
	}
	s.setContent(buildConvertView(s, s.source))
}

func (s *appState) shutdown() {
	s.stopPlayer()
	if s.player != nil {
		s.player.Close()
	}
}

func (s *appState) stopPlayer() {
	if s.playSess != nil {
		s.playSess.Stop()
		s.playSess = nil
	}
	if s.player != nil {
		s.player.Stop()
	}
	s.stopProgressLoop()
	s.playerReady = false
	s.playerPaused = true
}

func main() {
	initLogging()
	defer closeLogs()

	flag.Parse()
	setDebug(*debugFlag || os.Getenv("VIDEOTOOLS_DEBUG") != "")
	debugLog(logCatSystem, "starting VideoTools prototype at %s", time.Now().Format(time.RFC3339))

	args := flag.Args()
	if len(args) > 0 {
		if err := runCLI(args); err != nil {
			fmt.Fprintln(os.Stderr, "videotools:", err)
			fmt.Fprintln(os.Stderr)
			printUsage()
			os.Exit(1)
		}
		return
	}

	if display := os.Getenv("DISPLAY"); display == "" {
		debugLog(logCatUI, "DISPLAY environment variable is empty; GUI may not be visible in headless mode")
	} else {
		debugLog(logCatUI, "DISPLAY=%s", display)
	}
	runGUI()
}

func runGUI() {
	a := app.NewWithID("com.leaktechnologies.videotools")
	a.Settings().SetTheme(&monoTheme{})
	debugLog(logCatUI, "created fyne app: %#v", a)
	w := a.NewWindow("VideoTools")
	if icon := loadAppIcon(); icon != nil {
		a.SetIcon(icon)
		w.SetIcon(icon)
		debugLog(logCatUI, "app icon loaded and applied")
	} else {
		debugLog(logCatUI, "app icon not found; continuing without custom icon")
	}
	w.Resize(fyne.NewSize(920, 540))
	debugLog(logCatUI, "window initialized (size 920x540)")

	state := &appState{
		window: w,
		convert: convertConfig{
			OutputBase:       "converted",
			SelectedFormat:   formatOptions[0],
			Quality:          "Standard (CRF 23)",
			Mode:             "Simple",
			InverseTelecine:  true,
			InverseAutoNotes: "Default smoothing for interlaced footage.",
			OutputAspect:     "Source",
			AspectHandling:   "Auto",
		},
		player:       player.New(),
		playerVolume: 100,
		lastVolume:   100,
		playerMuted:  false,
		playerPaused: true,
	}
	defer state.shutdown()
	w.SetOnDropped(func(pos fyne.Position, items []fyne.URI) {
		state.handleDrop(items)
	})
	state.showMainMenu()
	debugLog(logCatUI, "main menu rendered with %d modules", len(modules))
	w.ShowAndRun()
}

func runCLI(args []string) error {
	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]
	debugLog(logCatCLI, "command=%s args=%v", cmd, cmdArgs)

	switch cmd {
	case "convert":
		return runConvertCLI(cmdArgs)
	case "combine", "merge":
		return runCombineCLI(cmdArgs)
	case "trim":
		handleTrim(cmdArgs)
	case "filters":
		handleFilters(cmdArgs)
	case "upscale":
		handleUpscale(cmdArgs)
	case "audio":
		handleAudio(cmdArgs)
	case "thumb":
		handleThumb(cmdArgs)
	case "inspect":
		handleInspect(cmdArgs)
	case "logs":
		return runLogsCLI()
	case "help":
		printUsage()
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	return nil
}

func runConvertCLI(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("convert requires input and output files (e.g. videotools convert input.avi output.mp4)")
	}
	in, out := args[0], args[1]
	debugLog(logCatFFMPEG, "convert input=%s output=%s", in, out)
	handleConvert([]string{in, out})
	return nil
}

func runCombineCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("combine requires input files and an output (e.g. videotools combine clip1.mov clip2.wav / final.mp4)")
	}
	inputs, outputs, err := splitIOArgs(args)
	if err != nil {
		return err
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return fmt.Errorf("combine expects one or more inputs, '/', then an output file")
	}
	debugLog(logCatFFMPEG, "combine inputs=%v output=%v", inputs, outputs)
	// For now feed inputs followed by outputs to the merge handler.
	handleMerge(append(inputs, outputs...))
	return nil
}

func splitIOArgs(args []string) (inputs []string, outputs []string, err error) {
	sep := -1
	for i, a := range args {
		if a == "/" {
			sep = i
			break
		}
	}
	if sep == -1 {
		return nil, nil, fmt.Errorf("missing '/' separator between inputs and outputs")
	}
	inputs = append(inputs, args[:sep]...)
	outputs = append(outputs, args[sep+1:]...)
	return inputs, outputs, nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  videotools convert <input> <output>")
	fmt.Println("  videotools combine <in1> <in2> ... / <output>")
	fmt.Println("  videotools trim <args>")
	fmt.Println("  videotools filters <args>")
	fmt.Println("  videotools upscale <args>")
	fmt.Println("  videotools audio <args>")
	fmt.Println("  videotools thumb <args>")
	fmt.Println("  videotools inspect <args>")
	fmt.Println("  videotools logs                 # tail recent log lines")
	fmt.Println("  videotools            # launch GUI")
	fmt.Println()
	fmt.Println("Set VIDEOTOOLS_DEBUG=1 or pass -debug for verbose logs.")
	fmt.Println("Logs are written to", logFilePath, "or set VIDEOTOOLS_LOG_FILE to override.")
}

func runLogsCLI() error {
	path := logFilePath
	if path == "" {
		return fmt.Errorf("log file unavailable")
	}
	debugLog(logCatCLI, "reading logs from %s", path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxLines = 200
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	fmt.Printf("--- showing last %d log lines from %s ---\n", len(lines), path)
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func buildMainMenu(state *appState) fyne.CanvasObject {
	title := canvas.NewText("VIDEOTOOLS", mustHex("#4CE870"))
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 28

	queueTile := buildQueueTile(0, 0)

	header := container.New(layout.NewHBoxLayout(),
		title,
		layout.NewSpacer(),
		queueTile,
	)

	var tileObjects []fyne.CanvasObject
	for _, mod := range modules {
		modCopy := mod
		tileObjects = append(tileObjects, buildModuleTile(modCopy, func() {
			state.showModule(modCopy.ID)
		}))
	}

	grid := container.NewGridWithColumns(3, tileObjects...)

	padding := canvas.NewRectangle(color.Transparent)
	padding.SetMinSize(fyne.NewSize(0, 14))

	body := container.New(layout.NewVBoxLayout(),
		header,
		padding,
		grid,
	)

	return body
}

func buildModuleTile(mod Module, tapped func()) fyne.CanvasObject {
	debugLog(logCatUI, "building tile %s color=%v", mod.ID, mod.Color)
	return container.NewPadded(newModuleTile(mod.Label, mod.Color, tapped))
}

func buildQueueTile(done, total int) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 8
	rect.SetMinSize(fyne.NewSize(160, 60))

	text := canvas.NewText(fmt.Sprintf("QUEUE: %d/%d", done, total), textColor)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 18

	return container.NewMax(rect, container.NewCenter(text))
}

func buildConvertView(state *appState, src *videoSource) fyne.CanvasObject {
	convertColor := moduleColor("convert")

	back := widget.NewButton("< CONVERT", func() {
		state.showMainMenu()
	})
	back.Importance = widget.LowImportance
	backBar := tintedBar(convertColor, container.NewHBox(back, layout.NewSpacer()))

	var updateCover func(string)
	coverLabel := widget.NewLabel(state.convert.CoverLabel())
	updateCover = func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		state.convert.CoverArtPath = path
		coverLabel.SetText(state.convert.CoverLabel())
	}

	videoPanel := buildVideoPane(state, fyne.NewSize(520, 300), src, updateCover)
	metaPanel := buildMetadataPanel(state, src, fyne.NewSize(520, 160))

	modeToggle := widget.NewRadioGroup([]string{"Simple", "Advanced"}, func(value string) {
		debugLog(logCatUI, "convert mode selected: %s", value)
		state.convert.Mode = value
	})
	modeToggle.Horizontal = true
	modeToggle.SetSelected(state.convert.Mode)

	var formatLabels []string
	for _, opt := range formatOptions {
		formatLabels = append(formatLabels, opt.Label)
	}
	outputHint := widget.NewLabel(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	formatSelect := widget.NewSelect(formatLabels, func(value string) {
		for _, opt := range formatOptions {
			if opt.Label == value {
				debugLog(logCatUI, "format set to %s", value)
				state.convert.SelectedFormat = opt
				outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
				break
			}
		}
	})
	formatSelect.SetSelected(state.convert.SelectedFormat.Label)

	qualitySelect := widget.NewSelect([]string{"Draft (CRF 28)", "Standard (CRF 23)", "High (CRF 18)", "Lossless"}, func(value string) {
		debugLog(logCatUI, "quality preset %s", value)
		state.convert.Quality = value
	})
	qualitySelect.SetSelected(state.convert.Quality)

	outputEntry := widget.NewEntry()
	outputEntry.SetText(state.convert.OutputBase)
	outputEntry.OnChanged = func(val string) {
		state.convert.OutputBase = val
		outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	}

	inverseCheck := widget.NewCheck("Smart Inverse Telecine", func(checked bool) {
		state.convert.InverseTelecine = checked
	})
	inverseCheck.Checked = state.convert.InverseTelecine
	inverseHint := widget.NewLabel(state.convert.InverseAutoNotes)

	aspectTargets := []string{"Source", "16:9", "4:3", "1:1", "9:16", "21:9"}
	targetAspectSelect := widget.NewSelect(aspectTargets, func(value string) {
		debugLog(logCatUI, "target aspect set to %s", value)
		state.convert.OutputAspect = value
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelect.SetSelected(state.convert.OutputAspect)
	targetAspectHint := widget.NewLabel("Pick desired output aspect (default Source).")

	aspectOptions := widget.NewRadioGroup([]string{"Auto", "Letterbox", "Pillarbox", "Blur Fill"}, func(value string) {
		debugLog(logCatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	})
	aspectOptions.Horizontal = false
	aspectOptions.Required = true
	aspectOptions.SetSelected(state.convert.AspectHandling)

	aspectOptions.SetSelected(state.convert.AspectHandling)

	backgroundHint := widget.NewLabel("Shown when aspect differs; choose padding/fill style.")
	aspectBox := container.NewVBox(
		widget.NewLabelWithStyle("Aspect Handling", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		aspectOptions,
		backgroundHint,
	)

	updateAspectBoxVisibility := func() {
		if src == nil {
			aspectBox.Hide()
			return
		}
		target := resolveTargetAspect(state.convert.OutputAspect, src)
		srcAspect := aspectRatioFloat(src.Width, src.Height)
		if target == 0 || srcAspect == 0 || ratiosApproxEqual(target, srcAspect, 0.01) {
			aspectBox.Hide()
		} else {
			aspectBox.Show()
		}
	}
	updateAspectBoxVisibility()
	targetAspectSelect.OnChanged = func(value string) {
		debugLog(logCatUI, "target aspect set to %s", value)
		state.convert.OutputAspect = value
		updateAspectBoxVisibility()
	}
	aspectOptions.OnChanged = func(value string) {
		debugLog(logCatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	}

	optionsBody := container.NewVBox(
		widget.NewLabelWithStyle("Mode", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		modeToggle,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Output Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		widget.NewLabelWithStyle("Output Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		outputHint,
		widget.NewLabelWithStyle("Cover Art", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		coverLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Quality", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		qualitySelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Inverse Telecine", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		inverseCheck,
		inverseHint,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Output Aspect", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelect,
		targetAspectHint,
		aspectBox,
		layout.NewSpacer(),
	)

	optionsRect := canvas.NewRectangle(mustHex("#13182B"))
	optionsRect.CornerRadius = 8
	optionsRect.StrokeColor = gridColor
	optionsRect.StrokeWidth = 1
	optionsPanel := container.NewMax(optionsRect, container.NewPadded(optionsBody))

	snippetBtn := widget.NewButton("Generate Snippet", func() {
		if state.source == nil {
			dialog.ShowInformation("Snippet", "Load a video first.", state.window)
			return
		}
		go state.generateSnippet()
	})
	snippetBtn.Importance = widget.MediumImportance
	if src == nil {
		snippetBtn.Disable()
	}
	snippetHint := widget.NewLabel("Creates a 20s clip centred on the timeline midpoint.")
	snippetRow := container.NewHBox(snippetBtn, layout.NewSpacer(), snippetHint)
	leftColumn := container.NewVBox(
		videoPanel,
		container.NewMax(metaPanel),
	)
	grid := container.NewGridWithColumns(2, leftColumn, optionsPanel)
	mainArea := container.NewPadded(container.NewVBox(
		grid,
		snippetRow,
	))

	resetBtn := widget.NewButton("Reset", func() {
		modeToggle.SetSelected("Simple")
		formatSelect.SetSelected("MP4 (H.264)")
		qualitySelect.SetSelected("Standard (CRF 23)")
		aspectOptions.SetSelected("Auto")
		targetAspectSelect.SetSelected("Source")
		updateAspectBoxVisibility()
		debugLog(logCatUI, "convert settings reset to defaults")
	})
	statusLabel := widget.NewLabel("")
	if state.convertBusy {
		statusLabel.SetText(state.convertStatus)
	} else if src != nil {
		statusLabel.SetText("Ready to convert")
	} else {
		statusLabel.SetText("Load a video to convert")
	}
	activity := widget.NewProgressBarInfinite()
	activity.Stop()
	activity.Hide()
	if state.convertBusy {
		activity.Show()
		activity.Start()
	}
	var convertBtn *widget.Button
	var cancelBtn *widget.Button
	cancelBtn = widget.NewButton("Cancel", func() {
		state.cancelConvert(cancelBtn, convertBtn, activity, statusLabel)
	})
	cancelBtn.Importance = widget.DangerImportance
	cancelBtn.Disable()
	convertBtn = widget.NewButton("CONVERT", func() {
		state.startConvert(statusLabel, convertBtn, cancelBtn, activity)
	})
	convertBtn.Importance = widget.HighImportance
	if src == nil {
		convertBtn.Disable()
	}
	if state.convertBusy {
		convertBtn.Disable()
		cancelBtn.Enable()
	}

	actionInner := container.NewHBox(resetBtn, activity, statusLabel, layout.NewSpacer(), cancelBtn, convertBtn)
	actionBar := tintedBar(convertColor, actionInner)

	return container.NewBorder(
		backBar,
		container.NewVBox(widget.NewSeparator(), actionBar),
		nil,
		nil,
		mainArea,
	)
}

func makeLabeledPanel(title, body string, min fyne.Size) *fyne.Container {
	rect := canvas.NewRectangle(mustHex("#191F35"))
	rect.CornerRadius = 8
	rect.StrokeColor = gridColor
	rect.StrokeWidth = 1
	rect.SetMinSize(min)

	header := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(body)
	desc.Wrapping = fyne.TextWrapWord

	box := container.NewVBox(header, desc, layout.NewSpacer())
	return container.NewMax(rect, container.NewPadded(box))
}

type moduleTile struct {
	widget.BaseWidget
	label    string
	color    color.Color
	onTapped func()
}

func newModuleTile(label string, col color.Color, tapped func()) *moduleTile {
	m := &moduleTile{
		label:    strings.ToUpper(label),
		color:    col,
		onTapped: tapped,
	}
	m.ExtendBaseWidget(m)
	return m
}

func (m *moduleTile) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(m.color)
	bg.CornerRadius = 8
	bg.StrokeColor = gridColor
	bg.StrokeWidth = 1

	txt := canvas.NewText(m.label, textColor)
	txt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	txt.Alignment = fyne.TextAlignCenter

	return &moduleTileRenderer{
		tile:  m,
		bg:    bg,
		label: txt,
	}
}

func (m *moduleTile) Tapped(*fyne.PointEvent) {
	if m.onTapped != nil {
		m.onTapped()
	}
}

type moduleTileRenderer struct {
	tile  *moduleTile
	bg    *canvas.Rectangle
	label *canvas.Text
}

func (r *moduleTileRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	labelSize := r.label.MinSize()
	r.label.Move(fyne.NewPos(
		(size.Width-labelSize.Width)/2,
		(size.Height-labelSize.Height)/2,
	))
}

func (r *moduleTileRenderer) MinSize() fyne.Size {
	return fyne.NewSize(220, 110)
}

func (r *moduleTileRenderer) Refresh() {
	r.bg.FillColor = r.tile.color
	r.bg.Refresh()
	r.label.Text = r.tile.label
	r.label.Refresh()
}

func (r *moduleTileRenderer) Destroy() {}

func (r *moduleTileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label}
}

func tintedBar(col color.Color, body fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(col)
	rect.SetMinSize(fyne.NewSize(0, 48))
	padded := container.NewPadded(body)
	return container.NewMax(rect, padded)
}

func buildMetadataPanel(state *appState, src *videoSource, min fyne.Size) fyne.CanvasObject {
	outer := canvas.NewRectangle(mustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	outer.SetMinSize(min)

	header := widget.NewLabelWithStyle("Metadata", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var top fyne.CanvasObject = header

	if src == nil {
		body := container.NewVBox(
			top,
			widget.NewSeparator(),
			widget.NewLabel("Load a clip to inspect its technical details."),
			layout.NewSpacer(),
		)
		return container.NewMax(outer, container.NewPadded(body))
	}

	bitrate := "--"
	if src.Bitrate > 0 {
		bitrate = fmt.Sprintf("%d kbps", src.Bitrate/1000)
	}

	info := widget.NewForm(
		widget.NewFormItem("File", widget.NewLabel(src.DisplayName)),
		widget.NewFormItem("Format", widget.NewLabel(firstNonEmpty(src.Format, "Unknown"))),
		widget.NewFormItem("Resolution", widget.NewLabel(fmt.Sprintf("%dx%d", src.Width, src.Height))),
		widget.NewFormItem("Aspect Ratio", widget.NewLabel(src.AspectRatioString())),
		widget.NewFormItem("Duration", widget.NewLabel(src.DurationString())),
		widget.NewFormItem("Video Codec", widget.NewLabel(firstNonEmpty(src.VideoCodec, "Unknown"))),
		widget.NewFormItem("Video Bitrate", widget.NewLabel(bitrate)),
		widget.NewFormItem("Frame Rate", widget.NewLabel(fmt.Sprintf("%.2f fps", src.FrameRate))),
		widget.NewFormItem("Pixel Format", widget.NewLabel(firstNonEmpty(src.PixelFormat, "Unknown"))),
		widget.NewFormItem("Field Order", widget.NewLabel(firstNonEmpty(src.FieldOrder, "Unknown"))),
		widget.NewFormItem("Audio Codec", widget.NewLabel(firstNonEmpty(src.AudioCodec, "Unknown"))),
		widget.NewFormItem("Audio Rate", widget.NewLabel(fmt.Sprintf("%d Hz", src.AudioRate))),
		widget.NewFormItem("Channels", widget.NewLabel(channelLabel(src.Channels))),
	)
	for _, item := range info.Items {
		if lbl, ok := item.Widget.(*widget.Label); ok {
			lbl.Wrapping = fyne.TextWrapWord
		}
	}

	// Clear button to remove the loaded video and reset UI.
	clearBtn := widget.NewButton("Clear Video", func() {
		if state != nil {
			state.clearVideo()
		}
	})
	clearBtn.Importance = widget.LowImportance
	top = container.NewBorder(nil, nil, nil, clearBtn, header)

	body := container.NewVBox(
		top,
		widget.NewSeparator(),
		info,
	)
	return container.NewMax(outer, container.NewPadded(body))
}

func buildVideoPane(state *appState, min fyne.Size, src *videoSource, onCover func(string)) fyne.CanvasObject {
	outer := canvas.NewRectangle(mustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	defaultAspect := 9.0 / 16.0
	if src != nil && src.Width > 0 && src.Height > 0 {
		defaultAspect = float64(src.Height) / float64(src.Width)
	}
	baseWidth := float64(min.Width)
	if baseWidth < 500 {
		baseWidth = 500
	}
	targetWidth := float32(baseWidth)
	_ = defaultAspect
	targetHeight := float32(min.Height)
	outer.SetMinSize(fyne.NewSize(targetWidth, targetHeight))

	if src == nil {
		icon := canvas.NewText("▶", mustHex("#4CE870"))
		icon.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		icon.TextSize = 42
		hintMain := widget.NewLabelWithStyle("Drop a video or open one to start playback", fyne.TextAlignCenter, fyne.TextStyle{Monospace: true, Bold: true})
		hintSub := widget.NewLabel("MP4, MOV, MKV and more")
		hintSub.Alignment = fyne.TextAlignCenter

		open := widget.NewButton("Open File…", func() {
			debugLog(logCatUI, "convert open file dialog requested")
			dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					debugLog(logCatUI, "file open error: %v", err)
					return
				}
				if r == nil {
					return
				}
				path := r.URI().Path()
				r.Close()
				go state.loadVideo(path)
			}, state.window)
			dlg.Resize(fyne.NewSize(600, 400))
			dlg.Show()
		})

		placeholder := container.NewVBox(
			container.NewCenter(icon),
			container.NewCenter(hintMain),
			container.NewCenter(hintSub),
			container.NewCenter(open),
		)
		return container.NewMax(outer, container.NewCenter(container.NewPadded(placeholder)))
	}

	state.stopPreview()

	sourceFrame := ""
	if len(src.PreviewFrames) == 0 {
		if thumb, err := capturePreviewFrames(src.Path, src.Duration); err == nil && len(thumb) > 0 {
			sourceFrame = thumb[0]
			src.PreviewFrames = thumb
		}
	} else {
		sourceFrame = src.PreviewFrames[0]
	}
	if sourceFrame != "" {
		state.currentFrame = sourceFrame
	}

	var img *canvas.Image
	if sourceFrame != "" {
		img = canvas.NewImageFromFile(sourceFrame)
	} else {
		img = canvas.NewImageFromResource(nil)
	}
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(targetWidth-28, targetHeight-40))
	stage := canvas.NewRectangle(mustHex("#0F1529"))
	stage.CornerRadius = 6
	stage.SetMinSize(fyne.NewSize(targetWidth-12, targetHeight-12))
	videoStage := container.NewMax(stage, container.NewPadded(container.NewCenter(img)))

	coverBtn := makeIconButton("⌾", "Set current frame as cover art", func() {
		path, err := state.captureCoverFromCurrent()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if onCover != nil {
			onCover(path)
		}
	})

	importBtn := makeIconButton("⬆", "Import cover art file", func() {
		dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			if dest, err := state.importCoverImage(path); err == nil {
				if onCover != nil {
					onCover(dest)
				}
			} else {
				dialog.ShowError(err, state.window)
			}
		}, state.window)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		dlg.Show()
	})

	usePlayer := true

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(src.DurationString())
	totalTime.Alignment = fyne.TextAlignTrailing
	var updatingProgress bool
	slider := widget.NewSlider(0, math.Max(1, src.Duration))
	slider.Step = 0.5
	updateProgress := func(val float64) {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			updatingProgress = true
			currentTime.SetText(formatClock(val))
			slider.SetValue(val)
			updatingProgress = false
		}, false)
	}

	var controls fyne.CanvasObject
	if usePlayer {
		var volIcon *widget.Button
		var updatingVolume bool
		ensureSession := func() bool {
			if state.playSess == nil {
				state.playSess = newPlaySession(src.Path, src.Width, src.Height, src.FrameRate, int(targetWidth-28), int(targetHeight-40), updateProgress, img)
				state.playSess.SetVolume(state.playerVolume)
				state.playerPaused = true
			}
			return state.playSess != nil
		}
		slider.OnChanged = func(val float64) {
			if updatingProgress {
				return
			}
			updateProgress(val)
			if ensureSession() {
				state.playSess.Seek(val)
			}
		}
		updateVolIcon := func() {
			if volIcon == nil {
				return
			}
			if state.playerMuted || state.playerVolume <= 0 {
				volIcon.SetText("🔇")
			} else {
				volIcon.SetText("🔊")
			}
		}
		volIcon = makeIconButton("🔊", "Mute/Unmute", func() {
			if !ensureSession() {
				return
			}
			if state.playerMuted {
				target := state.lastVolume
				if target <= 0 {
					target = 50
				}
				state.playerVolume = target
				state.playerMuted = false
				state.playSess.SetVolume(target)
			} else {
				state.lastVolume = state.playerVolume
				state.playerVolume = 0
				state.playerMuted = true
				state.playSess.SetVolume(0)
			}
			updateVolIcon()
		})
		volSlider := widget.NewSlider(0, 100)
		volSlider.Step = 1
		volSlider.Value = state.playerVolume
		volSlider.OnChanged = func(val float64) {
			if updatingVolume {
				return
			}
			state.playerVolume = val
			if val > 0 {
				state.lastVolume = val
				state.playerMuted = false
			} else {
				state.playerMuted = true
			}
			if ensureSession() {
				state.playSess.SetVolume(val)
			}
			updateVolIcon()
		}
		updateVolIcon()
		volSlider.Refresh()
		playBtn := makeIconButton("▶/⏸", "Play/Pause", func() {
			if !ensureSession() {
				return
			}
			if state.playerPaused {
				state.playSess.Play()
				state.playerPaused = false
			} else {
				state.playSess.Pause()
				state.playerPaused = true
			}
		})
		fullBtn := makeIconButton("⛶", "Toggle fullscreen", func() {
			// Placeholder: embed fullscreen toggle into playback surface later.
		})
		volBox := container.NewHBox(volIcon, container.NewMax(volSlider))
		progress := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		controls = container.NewVBox(
			container.NewHBox(playBtn, fullBtn, coverBtn, importBtn, layout.NewSpacer(), volBox),
			progress,
		)
	} else {
		slider := widget.NewSlider(0, math.Max(1, float64(len(src.PreviewFrames)-1)))
		slider.Step = 1
		slider.OnChanged = func(val float64) {
			if state.anim != nil && state.anim.playing {
				state.anim.Pause()
			}
			idx := int(val)
			if idx >= 0 && idx < len(src.PreviewFrames) {
				state.showFrameManual(src.PreviewFrames[idx], img)
				if slider.Max > 0 {
					approx := (val / slider.Max) * src.Duration
					currentTime.SetText(formatClock(approx))
				}
			}
		}
		playBtn := makeIconButton("▶/⏸", "Play/Pause", func() {
			if len(src.PreviewFrames) == 0 {
				return
			}
			if state.anim == nil {
				state.startPreview(src.PreviewFrames, img, slider)
				return
			}
			if state.anim.playing {
				state.anim.Pause()
			} else {
				state.anim.Play()
			}
		})
		volSlider := widget.NewSlider(0, 100)
		volSlider.Disable()
		progress := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		controls = container.NewVBox(
			container.NewHBox(playBtn, coverBtn, importBtn, layout.NewSpacer(), widget.NewLabel("🔇"), container.NewMax(volSlider)),
			progress,
		)
		if len(src.PreviewFrames) > 1 {
			state.startPreview(src.PreviewFrames, img, slider)
		} else {
			playBtn.Disable()
		}
	}

	barBg := canvas.NewRectangle(color.NRGBA{R: 12, G: 17, B: 31, A: 180})
	barBg.SetMinSize(fyne.NewSize(targetWidth-32, 72))
	overlayBar := container.NewMax(barBg, container.NewPadded(controls))

	overlay := container.NewVBox(layout.NewSpacer(), overlayBar)
	videoWithOverlay := container.NewMax(videoStage, overlay)
	state.setPlayerSurface(videoStage, int(targetWidth-12), int(targetHeight-12))

	stack := container.NewVBox(
		container.NewPadded(videoWithOverlay),
	)
	return container.NewMax(outer, container.NewCenter(container.NewPadded(stack)))
}

func moduleColor(id string) color.Color {
	for _, m := range modules {
		if m.ID == id {
			return m.Color
		}
	}
	return queueColor
}

type playSession struct {
	path     string
	fps      float64
	width    int
	height   int
	targetW  int
	targetH  int
	volume   float64
	muted    bool
	paused   bool
	current  float64
	stop     chan struct{}
	done     chan struct{}
	prog     func(float64)
	img      *canvas.Image
	mu       sync.Mutex
	videoCmd *exec.Cmd
	audioCmd *exec.Cmd
	frameN   int
}

var audioCtxGlobal struct {
	once sync.Once
	ctx  *oto.Context
	err  error
}

func getAudioContext(sampleRate, channels, bytesPerSample int) (*oto.Context, error) {
	audioCtxGlobal.once.Do(func() {
		audioCtxGlobal.ctx, audioCtxGlobal.err = oto.NewContext(sampleRate, channels, bytesPerSample, 2048)
	})
	return audioCtxGlobal.ctx, audioCtxGlobal.err
}

func newPlaySession(path string, w, h int, fps float64, targetW, targetH int, prog func(float64), img *canvas.Image) *playSession {
	if fps <= 0 {
		fps = 24
	}
	if targetW <= 0 {
		targetW = 640
	}
	if targetH <= 0 {
		targetH = int(float64(targetW) * (float64(h) / float64(maxInt(w, 1))))
	}
	return &playSession{
		path:    path,
		fps:     fps,
		width:   w,
		height:  h,
		targetW: targetW,
		targetH: targetH,
		volume:  100,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		prog:    prog,
		img:     img,
	}
}

func (p *playSession) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.videoCmd == nil && p.audioCmd == nil {
		p.startLocked(p.current)
		return
	}
	p.paused = false
}

func (p *playSession) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

func (p *playSession) Seek(offset float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if offset < 0 {
		offset = 0
	}
	paused := p.paused
	p.current = offset
	p.stopLocked()
	p.startLocked(p.current)
	p.paused = paused
	if p.paused {
		// Ensure loops honor paused right after restart.
		time.AfterFunc(30*time.Millisecond, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.paused = true
		})
	}
	if p.prog != nil {
		p.prog(p.current)
	}
}

func (p *playSession) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.volume = v
	if v > 0 {
		p.muted = false
	} else {
		p.muted = true
	}
}

func (p *playSession) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *playSession) stopLocked() {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	if p.videoCmd != nil && p.videoCmd.Process != nil {
		_ = p.videoCmd.Process.Kill()
		_ = p.videoCmd.Wait()
	}
	if p.audioCmd != nil && p.audioCmd.Process != nil {
		_ = p.audioCmd.Process.Kill()
		_ = p.audioCmd.Wait()
	}
	p.videoCmd = nil
	p.audioCmd = nil
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
}

func (p *playSession) startLocked(offset float64) {
	p.paused = false
	p.current = offset
	p.frameN = 0
	debugLog(logCatFFMPEG, "playSession start path=%s offset=%.3f fps=%.3f target=%dx%d", p.path, offset, p.fps, p.targetW, p.targetH)
	p.runVideo(offset)
	p.runAudio(offset)
}

func (p *playSession) runVideo(offset float64) {
	var stderr bytes.Buffer
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vf", fmt.Sprintf("scale=%d:%d", p.targetW, p.targetH),
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-r", fmt.Sprintf("%.3f", p.fps),
		"-",
	}
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		debugLog(logCatFFMPEG, "video pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		debugLog(logCatFFMPEG, "video start failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
		return
	}
	// Pace frames to the source frame rate instead of hammering refreshes as fast as possible.
	frameDur := time.Second
	if p.fps > 0 {
		frameDur = time.Duration(float64(time.Second) / math.Max(p.fps, 0.1))
	}
	nextFrameAt := time.Now()
	p.videoCmd = cmd
	frameSize := p.targetW * p.targetH * 3
	buf := make([]byte, frameSize)
	go func() {
		defer cmd.Process.Kill()
		for {
			select {
			case <-p.stop:
				debugLog(logCatFFMPEG, "video loop stop")
				return
			default:
			}
			if p.paused {
				time.Sleep(30 * time.Millisecond)
				nextFrameAt = time.Now().Add(frameDur)
				continue
			}
			_, err := io.ReadFull(stdout, buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				msg := strings.TrimSpace(stderr.String())
				debugLog(logCatFFMPEG, "video read failed: %v (%s)", err, msg)
				return
			}
			if delay := time.Until(nextFrameAt); delay > 0 {
				time.Sleep(delay)
			}
			nextFrameAt = nextFrameAt.Add(frameDur)
			// Allocate a fresh frame to avoid concurrent texture reuse issues.
			frame := image.NewRGBA(image.Rect(0, 0, p.targetW, p.targetH))
			copyRGBToRGBA(frame.Pix, buf)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				if p.img != nil {
					// Ensure we render the live frame, not a stale resource preview.
					p.img.Resource = nil
					p.img.File = ""
					p.img.Image = frame
					p.img.Refresh()
				}
			}, false)
			if p.frameN < 3 {
				debugLog(logCatFFMPEG, "video frame %d drawn (%.2fs)", p.frameN+1, p.current)
			}
			p.frameN++
			if p.fps > 0 {
				p.current = offset + (float64(p.frameN) / p.fps)
			}
			if p.prog != nil {
				p.prog(p.current)
			}
		}
	}()
}

func (p *playSession) runAudio(offset float64) {
	const sampleRate = 48000
	const channels = 2
	const bytesPerSample = 2
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-f", "s16le",
		"-",
	)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		debugLog(logCatFFMPEG, "audio pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		debugLog(logCatFFMPEG, "audio start failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
		return
	}
	p.audioCmd = cmd
	ctx, err := getAudioContext(sampleRate, channels, bytesPerSample)
	if err != nil {
		debugLog(logCatFFMPEG, "audio context error: %v", err)
		return
	}
	player := ctx.NewPlayer()
	if player == nil {
		debugLog(logCatFFMPEG, "audio player creation failed")
		return
	}
	localPlayer := player
	go func() {
		defer cmd.Process.Kill()
		defer localPlayer.Close()
		chunk := make([]byte, 4096)
		tmp := make([]byte, 4096)
		loggedFirst := false
		for {
			select {
			case <-p.stop:
				debugLog(logCatFFMPEG, "audio loop stop")
				return
			default:
			}
			if p.paused {
				time.Sleep(30 * time.Millisecond)
				continue
			}
			n, err := stdout.Read(chunk)
			if n > 0 {
				if !loggedFirst {
					debugLog(logCatFFMPEG, "audio stream delivering bytes")
					loggedFirst = true
				}
				gain := p.volume / 100.0
				if gain < 0 {
					gain = 0
				}
				if gain > 2 {
					gain = 2
				}
				copy(tmp, chunk[:n])
				if p.muted || gain <= 0 {
					for i := 0; i < n; i++ {
						tmp[i] = 0
					}
				} else if math.Abs(1-gain) > 0.001 {
					for i := 0; i+1 < n; i += 2 {
						sample := int16(binary.LittleEndian.Uint16(tmp[i:]))
						amp := int(float64(sample) * gain)
						if amp > math.MaxInt16 {
							amp = math.MaxInt16
						}
						if amp < math.MinInt16 {
							amp = math.MinInt16
						}
						binary.LittleEndian.PutUint16(tmp[i:], uint16(int16(amp)))
					}
				}
				localPlayer.Write(tmp[:n])
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					debugLog(logCatFFMPEG, "audio read failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
				}
				return
			}
		}
	}()
}

type previewAnimator struct {
	frames  []string
	img     *canvas.Image
	slider  *widget.Slider
	stop    chan struct{}
	playing bool
	state   *appState
	index   int
}

func (a *previewAnimator) Start() {
	if len(a.frames) == 0 {
		return
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		idx := 0
		for {
			select {
			case <-a.stop:
				return
			case <-ticker.C:
				if !a.playing {
					continue
				}
				idx = (idx + 1) % len(a.frames)
				a.index = idx
				frame := a.frames[idx]
				a.showFrame(frame)
				if a.slider != nil {
					cur := float64(idx)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						a.slider.SetValue(cur)
					}, false)
				}
			}
		}
	}()
}

func (a *previewAnimator) Pause() { a.playing = false }
func (a *previewAnimator) Play()  { a.playing = true }

func (a *previewAnimator) showFrame(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	frame, err := png.Decode(f)
	if err != nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		a.img.Image = frame
		a.img.Refresh()
		if a.state != nil {
			a.state.currentFrame = path
		}
	}, false)
}

func (a *previewAnimator) Stop() {
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
}

func (s *appState) showFrameManual(path string, img *canvas.Image) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	frame, err := png.Decode(f)
	if err != nil {
		return
	}
	img.Image = frame
	img.Refresh()
	s.currentFrame = path
}

func (s *appState) captureCoverFromCurrent() (string, error) {
	if s.currentFrame == "" {
		return "", fmt.Errorf("no frame available")
	}
	data, err := os.ReadFile(s.currentFrame)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-cover-%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *appState) importCoverImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-cover-import-%d%s", time.Now().UnixNano(), filepath.Ext(path)))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *appState) handleDrop(items []fyne.URI) {
	if len(items) == 0 {
		return
	}
	for _, uri := range items {
		if uri.Scheme() != "file" {
			continue
		}
		path := uri.Path()
		debugLog(logCatModule, "drop received path=%s active=%s", path, s.active)
		switch s.active {
		case "convert":
			go s.loadVideo(path)
		default:
			debugLog(logCatUI, "drop ignored; no module active to handle file")
		}
		break
	}
}

func (s *appState) loadVideo(path string) {
	win := s.window
	if s.playSess != nil {
		s.playSess.Stop()
		s.playSess = nil
	}
	s.stopProgressLoop()
	src, err := probeVideo(path)
	if err != nil {
		debugLog(logCatFFMPEG, "ffprobe failed for %s: %v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			dialog.ShowError(fmt.Errorf("failed to analyze %s: %w", filepath.Base(path), err), win)
		}, false)
		return
	}
	if frames, err := capturePreviewFrames(src.Path, src.Duration); err == nil {
		src.PreviewFrames = frames
		if len(frames) > 0 {
			s.currentFrame = frames[0]
		}
	} else {
		debugLog(logCatFFMPEG, "preview generation failed: %v", err)
		s.currentFrame = ""
	}
	s.applyInverseDefaults(src)
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	s.convert.OutputBase = base + "-convert"
	s.convert.CoverArtPath = ""
	s.convert.AspectHandling = "Auto"
	s.playerReady = false
	s.playerPos = 0
	s.playerPaused = true
	debugLog(logCatModule, "video loaded %+v", src)
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(src)
	}, false)
}

func (s *appState) clearVideo() {
	debugLog(logCatModule, "clearing loaded video")
	s.stopPlayer()
	s.source = nil
	s.currentFrame = ""
	s.convertBusy = false
	s.convertStatus = ""
	s.convert.OutputBase = "converted"
	s.convert.CoverArtPath = ""
	s.convert.AspectHandling = "Auto"
	s.convert.OutputAspect = "Source"
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(nil)
	}, false)
}

func crfForQuality(q string) string {
	switch q {
	case "Draft (CRF 28)":
		return "28"
	case "High (CRF 18)":
		return "18"
	case "Lossless":
		return "0"
	default:
		return "23"
	}
}

func (s *appState) cancelConvert(cancelBtn, btn *widget.Button, spinner *widget.ProgressBarInfinite, status *widget.Label) {
	if s.convertCancel == nil {
		return
	}
	if cancelBtn != nil {
		cancelBtn.Disable()
	}
	s.convertStatus = "Cancelling…"
	if status != nil {
		status.SetText(s.convertStatus)
	}
	s.convertCancel()
}

func (s *appState) startConvert(status *widget.Label, btn, cancelBtn *widget.Button, spinner *widget.ProgressBarInfinite) {
	setStatus := func(msg string) {
		s.convertStatus = msg
		debugLog(logCatFFMPEG, "convert status: %s", msg)
		if status != nil {
			status.SetText(msg)
		}
	}
	if s.source == nil {
		dialog.ShowInformation("Convert", "Load a video first.", s.window)
		return
	}
	if s.convertBusy {
		return
	}
	src := s.source
	cfg := s.convert
	outDir := filepath.Dir(src.Path)
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", src.Path,
	}
	// Video filters.
	var vf []string
	if cfg.InverseTelecine {
		vf = append(vf, "yadif")
	}
	srcAspect := aspectRatioFloat(src.Width, src.Height)
	targetAspect := resolveTargetAspect(cfg.OutputAspect, src)
	if targetAspect > 0 && srcAspect > 0 && !ratiosApproxEqual(targetAspect, srcAspect, 0.01) {
		vf = append(vf, aspectFilters(targetAspect, cfg.AspectHandling)...)
	}
	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}
	// Video codec and quality.
	args = append(args, "-c:v", cfg.SelectedFormat.VideoCodec)
	crf := crfForQuality(cfg.Quality)
	if cfg.SelectedFormat.VideoCodec == "libx264" || cfg.SelectedFormat.VideoCodec == "libx265" {
		args = append(args, "-crf", crf, "-preset", "medium")
	}
	// Audio: copy if present.
	args = append(args, "-c:a", "copy")
	// Ensure quickstart for MP4/MOV outputs.
	if strings.EqualFold(cfg.SelectedFormat.Ext, ".mp4") || strings.EqualFold(cfg.SelectedFormat.Ext, ".mov") {
		args = append(args, "-movflags", "+faststart")
	}
	// Progress feed to stdout for live updates.
	args = append(args, "-progress", "pipe:1", "-nostats")
	args = append(args, outPath)

	debugLog(logCatFFMPEG, "convert command: ffmpeg %s", strings.Join(args, " "))
	s.convertBusy = true
	setStatus("Preparing conversion…")
	if btn != nil {
		btn.Disable()
	}
	if spinner != nil {
		spinner.Show()
		spinner.Start()
	}
	if cancelBtn != nil {
		cancelBtn.Enable()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.convertCancel = cancel

	go func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Running ffmpeg…")
		}, false)

		started := time.Now()
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			debugLog(logCatFFMPEG, "convert stdout pipe failed: %v", err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				dialog.ShowError(fmt.Errorf("convert failed: %w", err), s.window)
				s.convertBusy = false
				setStatus("Failed")
				if btn != nil {
					btn.Enable()
				}
				if cancelBtn != nil {
					cancelBtn.Disable()
				}
				if spinner != nil {
					spinner.Stop()
					spinner.Hide()
				}
			}, false)
			s.convertCancel = nil
			return
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		progressQuit := make(chan struct{})
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case <-progressQuit:
					return
				default:
				}
				line := scanner.Text()
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, val := parts[0], parts[1]
				if key != "out_time_ms" && key != "progress" {
					continue
				}
				if key == "out_time_ms" {
					ms, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					elapsedProc := ms / 1000000.0
					total := src.Duration
					var pct float64
					if total > 0 {
						pct = math.Min(100, math.Max(0, (elapsedProc/total)*100))
					}
					elapsedWall := time.Since(started).Seconds()
					var eta string
					if pct > 0 && elapsedWall > 0 && pct < 100 {
						remaining := elapsedWall * (100 - pct) / pct
						eta = formatShortDuration(remaining)
					}
					speed := 0.0
					if elapsedWall > 0 {
						speed = elapsedProc / elapsedWall
					}
					lbl := fmt.Sprintf("Converting… %.0f%% | elapsed %s | ETA %s | %.2fx", pct, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						setStatus(lbl)
					}, false)
				}
				if key == "progress" && val == "end" {
					return
				}
			}
		}()

		if err := cmd.Start(); err != nil {
			close(progressQuit)
			debugLog(logCatFFMPEG, "convert failed to start: %v", err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				dialog.ShowError(fmt.Errorf("convert failed: %w", err), s.window)
				s.convertBusy = false
				setStatus("Failed")
				if btn != nil {
					btn.Enable()
				}
				if cancelBtn != nil {
					cancelBtn.Disable()
				}
				if spinner != nil {
					spinner.Stop()
					spinner.Hide()
				}
			}, false)
			s.convertCancel = nil
			return
		}

		err = cmd.Wait()
		close(progressQuit)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				debugLog(logCatFFMPEG, "convert cancelled")
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.convertBusy = false
					setStatus("Cancelled")
					if btn != nil {
						btn.Enable()
					}
					if cancelBtn != nil {
						cancelBtn.Disable()
					}
					if spinner != nil {
						spinner.Stop()
						spinner.Hide()
					}
				}, false)
				s.convertCancel = nil
				return
			}
			debugLog(logCatFFMPEG, "convert failed: %v stderr=%s", err, strings.TrimSpace(stderr.String()))
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				dialog.ShowError(fmt.Errorf("convert failed: %w", err), s.window)
				s.convertBusy = false
				setStatus("Failed")
				if btn != nil {
					btn.Enable()
				}
				if cancelBtn != nil {
					cancelBtn.Disable()
				}
				if spinner != nil {
					spinner.Stop()
					spinner.Hide()
				}
			}, false)
			s.convertCancel = nil
			return
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Validating output…")
		}, false)
		if _, probeErr := probeVideo(outPath); probeErr != nil {
			debugLog(logCatFFMPEG, "convert probe failed: %v", probeErr)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				dialog.ShowError(fmt.Errorf("conversion output is invalid: %w", probeErr), s.window)
				s.convertBusy = false
				setStatus("Failed")
				if btn != nil {
					btn.Enable()
				}
				if cancelBtn != nil {
					cancelBtn.Disable()
				}
				if spinner != nil {
					spinner.Stop()
					spinner.Hide()
				}
			}, false)
			s.convertCancel = nil
			return
		}
		debugLog(logCatFFMPEG, "convert completed: %s", outPath)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			dialog.ShowInformation("Convert", fmt.Sprintf("Saved %s", outPath), s.window)
			s.convertBusy = false
			setStatus("Done")
			if btn != nil {
				btn.Enable()
			}
			if cancelBtn != nil {
				cancelBtn.Disable()
			}
			if spinner != nil {
				spinner.Stop()
				spinner.Hide()
			}
		}, false)
		s.convertCancel = nil
	}()
}

func formatShortDuration(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func etaOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "--"
	}
	return s
}

func aspectFilters(target float64, mode string) []string {
	if target <= 0 {
		return nil
	}
	ar := fmt.Sprintf("%.6f", target)
	scale := fmt.Sprintf("scale=w='if(gt(a,%[1]s),round(ih*%[1]s/2)*2,iw)':h='if(gt(a,%[1]s),ih,round(iw/%[1]s/2)*2)'", ar)
	padColor := "black"
	if strings.EqualFold(mode, "Blur Fill") {
		padColor = "black"
	}
	// Future: expand blur fill to use blurred edges; currently shares padding approach.
	pad := fmt.Sprintf("pad=w='max(iw,round(ih*%[1]s/2)*2)':h='max(round(iw/%[1]s/2)*2,ih)':x='(ow-iw)/2':y='(oh-ih)/2':color=%s", ar, padColor)
	return []string{scale, pad, "setsar=1"}
}

func loadAppIcon() fyne.Resource {
	search := []string{
		filepath.Join("assets", "logo", "VT_Icon.svg"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		search = append(search, filepath.Join(dir, "assets", "logo", "VT_Icon.svg"))
	}
	for _, p := range search {
		if _, err := os.Stat(p); err == nil {
			res, err := fyne.LoadResourceFromPath(p)
			if err != nil {
				debugLog(logCatUI, "failed to load icon %s: %v", p, err)
				continue
			}
			return res
		}
	}
	return nil
}
func (s *appState) generateSnippet() {
	if s.source == nil {
		return
	}
	src := s.source
	center := math.Max(0, src.Duration/2-10)
	start := fmt.Sprintf("%.2f", center)
	outName := fmt.Sprintf("%s-snippet-%d.mp4", strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)), time.Now().Unix())
	outPath := filepath.Join(filepath.Dir(src.Path), outName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", start,
		"-i", src.Path,
		"-t", "20",
		"-c", "copy",
		outPath,
	)
	debugLog(logCatFFMPEG, "snippet command: %s", strings.Join(cmd.Args, " "))
	if out, err := cmd.CombinedOutput(); err != nil {
		debugLog(logCatFFMPEG, "snippet stderr: %s", string(out))
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			dialog.ShowError(fmt.Errorf("snippet failed: %w", err), s.window)
		}, false)
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		dialog.ShowInformation("Snippet Created", fmt.Sprintf("Saved %s", outPath), s.window)
	}, false)
}

func capturePreviewFrames(path string, duration float64) ([]string, error) {
	center := math.Max(0, duration/2-1)
	start := fmt.Sprintf("%.2f", center)
	dir, err := os.MkdirTemp("", "videotools-frames-*")
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(dir, "frame-%03d.png")
	cmd := exec.Command("ffmpeg",
		"-y",
		"-ss", start,
		"-i", path,
		"-t", "3",
		"-vf", "scale=640:-1:flags=lanczos,fps=8",
		pattern,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("preview capture failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	files, err := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no preview frames generated")
	}
	slices.Sort(files)
	return files, nil
}

type monoTheme struct{}

func (m *monoTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m *monoTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Monospace = true
	return theme.DefaultTheme().Font(style)
}

func (m *monoTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *monoTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

type videoSource struct {
	Path          string
	DisplayName   string
	Format        string
	Width         int
	Height        int
	Duration      float64
	VideoCodec    string
	AudioCodec    string
	Bitrate       int
	FrameRate     float64
	PixelFormat   string
	AudioRate     int
	Channels      int
	FieldOrder    string
	PreviewFrames []string
}

func (v *videoSource) DurationString() string {
	if v.Duration <= 0 {
		return "--"
	}
	d := time.Duration(v.Duration * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (v *videoSource) AspectRatioString() string {
	if v.Width <= 0 || v.Height <= 0 {
		return "--"
	}
	num, den := simplifyRatio(v.Width, v.Height)
	if num == 0 || den == 0 {
		return "--"
	}
	ratio := float64(num) / float64(den)
	return fmt.Sprintf("%d:%d (%.2f:1)", num, den, ratio)
}

func formatClock(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (v *videoSource) IsProgressive() bool {
	order := strings.ToLower(v.FieldOrder)
	if strings.Contains(order, "progressive") {
		return true
	}
	if strings.Contains(order, "unknown") && strings.Contains(strings.ToLower(v.PixelFormat), "p") {
		return true
	}
	return false
}

func probeVideo(path string) (*videoSource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Format struct {
			Filename   string `json:"filename"`
			Format     string `json:"format_long_name"`
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			Duration     string `json:"duration"`
			BitRate      string `json:"bit_rate"`
			PixFmt       string `json:"pix_fmt"`
			SampleRate   string `json:"sample_rate"`
			Channels     int    `json:"channels"`
			AvgFrameRate string `json:"avg_frame_rate"`
			FieldOrder   string `json:"field_order"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	src := &videoSource{
		Path:        path,
		DisplayName: filepath.Base(path),
		Format:      firstNonEmpty(result.Format.Format, result.Format.FormatName),
	}
	if rate, err := parseInt(result.Format.BitRate); err == nil {
		src.Bitrate = rate
	}
	if durStr := result.Format.Duration; durStr != "" {
		if val, err := parseFloat(durStr); err == nil {
			src.Duration = val
		}
	}
	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			if src.VideoCodec == "" {
				src.VideoCodec = stream.CodecName
				src.FieldOrder = stream.FieldOrder
				if stream.Width > 0 {
					src.Width = stream.Width
				}
				if stream.Height > 0 {
					src.Height = stream.Height
				}
				if dur, err := parseFloat(stream.Duration); err == nil && dur > 0 {
					src.Duration = dur
				}
				if fr := parseFraction(stream.AvgFrameRate); fr > 0 {
					src.FrameRate = fr
				}
				if stream.PixFmt != "" {
					src.PixelFormat = stream.PixFmt
				}
			}
			if src.Bitrate == 0 {
				if br, err := parseInt(stream.BitRate); err == nil {
					src.Bitrate = br
				}
			}
		case "audio":
			if src.AudioCodec == "" {
				src.AudioCodec = stream.CodecName
				if rate, err := parseInt(stream.SampleRate); err == nil {
					src.AudioRate = rate
				}
				if stream.Channels > 0 {
					src.Channels = stream.Channels
				}
			}
		}
	}
	return src, nil
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.Atoi(s)
}

func parseFraction(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0
	}
	parts := strings.Split(s, "/")
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	if len(parts) == 1 {
		return num
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0
	}
	return num / den
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "--"
}

func channelLabel(ch int) string {
	switch ch {
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		if ch <= 0 {
			return ""
		}
		return fmt.Sprintf("%d ch", ch)
	}
}

func gcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func simplifyRatio(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	g := gcd(w, h)
	return w / g, h / g
}

func aspectRatioFloat(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(w) / float64(h)
}

func parseAspectValue(val string) float64 {
	val = strings.TrimSpace(val)
	switch val {
	case "16:9":
		return 16.0 / 9.0
	case "4:3":
		return 4.0 / 3.0
	case "1:1":
		return 1
	case "9:16":
		return 9.0 / 16.0
	case "21:9":
		return 21.0 / 9.0
	}
	parts := strings.Split(val, ":")
	if len(parts) == 2 {
		n, err1 := strconv.ParseFloat(parts[0], 64)
		d, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && d != 0 {
			return n / d
		}
	}
	return 0
}

func resolveTargetAspect(val string, src *videoSource) float64 {
	if strings.EqualFold(val, "source") {
		if src != nil {
			return aspectRatioFloat(src.Width, src.Height)
		}
		return 0
	}
	if r := parseAspectValue(val); r > 0 {
		return r
	}
	return 0
}

func ratiosApproxEqual(a, b, tol float64) bool {
	if a == 0 || b == 0 {
		return false
	}
	diff := math.Abs(a - b)
	if b != 0 {
		diff = diff / b
	}
	return diff <= tol
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// copyRGBToRGBA expands packed RGB bytes into RGBA while forcing opaque alpha.
func copyRGBToRGBA(dst, src []byte) {
	di := 0
	for si := 0; si+2 < len(src) && di+3 < len(dst); si, di = si+3, di+4 {
		dst[di] = src[si]
		dst[di+1] = src[si+1]
		dst[di+2] = src[si+2]
		dst[di+3] = 0xff
	}
}

func makeIconButton(symbol, tooltip string, tapped func()) *widget.Button {
	btn := widget.NewButton(symbol, tapped)
	btn.Importance = widget.LowImportance
	return btn
}

func mustHex(h string) color.NRGBA {
	c, err := parseHexColor(h)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid color %q: %v\n", h, err)
		os.Exit(1)
	}
	return c
}

func parseHexColor(s string) (color.NRGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.NRGBA{}, fmt.Errorf("want 6 digits, got %q", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}, nil
}

// Placeholder handlers keep the prototype compiling while we wire modules.
func handleConvert(files []string) {
	debugLog(logCatFFMPEG, "convert handler invoked with %v", files)
	fmt.Println("convert", files)
}

func handleMerge(files []string) {
	debugLog(logCatFFMPEG, "merge handler invoked with %v", files)
	fmt.Println("merge", files)
}

func handleTrim(files []string) {
	debugLog(logCatModule, "trim handler invoked with %v", files)
	fmt.Println("trim", files)
}

func handleFilters(files []string) {
	debugLog(logCatModule, "filters handler invoked with %v", files)
	fmt.Println("filters", files)
}

func handleUpscale(files []string) {
	debugLog(logCatModule, "upscale handler invoked with %v", files)
	fmt.Println("upscale", files)
}

func handleAudio(files []string) {
	debugLog(logCatModule, "audio handler invoked with %v", files)
	fmt.Println("audio", files)
}

func handleThumb(files []string) {
	debugLog(logCatModule, "thumb handler invoked with %v", files)
	fmt.Println("thumb", files)
}

func handleInspect(files []string) {
	debugLog(logCatModule, "inspect handler invoked with %v", files)
	fmt.Println("inspect", files)
}

func initLogging() {
	logFilePath = os.Getenv("VIDEOTOOLS_LOG_FILE")
	if logFilePath == "" {
		logFilePath = "videotools.log"
	}
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot open log file %s: %v\n", logFilePath, err)
		return
	}
	logFile = f
}

func closeLogs() {
	if logFile != nil {
		logFile.Close()
	}
}

func setDebug(on bool) {
	debugEnabled = on
	debugLog(logCatSystem, "debug logging toggled -> %v (VIDEOTOOLS_DEBUG=%s)", on, os.Getenv("VIDEOTOOLS_DEBUG"))
}

func debugLog(cat logCategory, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if logFile != nil {
		fmt.Fprintf(logFile, "%s %s\n", timestamp, msg)
	}
	logHistory = append(logHistory, fmt.Sprintf("%s %s", timestamp, msg))
	if len(logHistory) > logHistoryMax {
		logHistory = logHistory[len(logHistory)-logHistoryMax:]
	}
	if debugEnabled {
		debugLogger.Printf("%s %s", timestamp, msg)
	}
}
