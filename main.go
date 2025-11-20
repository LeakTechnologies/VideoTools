package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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
	window       fyne.Window
	active       string
	source       *videoSource
	anim         *previewAnimator
	convert      convertConfig
	currentFrame string
	player       player.Controller
	playerReady  bool
	playerVolume float64
	playerPaused bool
}

func (s *appState) stopPreview() {
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
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
	if s.player != nil {
		s.player.Stop()
	}
	s.playerReady = false
	s.playerPaused = false
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
		},
		player:       player.New(),
		playerVolume: 100,
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
	metaPanel := buildMetadataPanel(src, fyne.NewSize(520, 160))

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

	aspectOptions := widget.NewRadioGroup([]string{"Auto", "Letterbox", "Pillarbox", "Blur Fill"}, func(value string) {
		debugLog(logCatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	})
	aspectOptions.Horizontal = false
	aspectOptions.Required = true
	aspectOptions.SetSelected(state.convert.AspectHandling)

	aspectOptions.SetSelected(state.convert.AspectHandling)

	backgroundHint := widget.NewLabel("Choose how 4:3 or 9:16 footage fits into 16:9 exports.")

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
		widget.NewLabelWithStyle("Aspect Handling", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		aspectOptions,
		backgroundHint,
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
		debugLog(logCatUI, "convert settings reset to defaults")
	})
	convertBtn := widget.NewButton("CONVERT", func() {
		debugLog(logCatModule, "convert action triggered -> %s", state.convert.OutputFile())
	})
	convertBtn.Importance = widget.HighImportance

	actionInner := container.NewHBox(resetBtn, layout.NewSpacer(), convertBtn)
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

func buildMetadataPanel(src *videoSource, min fyne.Size) fyne.CanvasObject {
	outer := canvas.NewRectangle(mustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	outer.SetMinSize(min)

	header := widget.NewLabelWithStyle("Metadata", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	if src == nil {
		body := container.NewVBox(
			header,
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

	body := container.NewVBox(
		header,
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
	targetHeight := float32(math.Max(float64(min.Height), baseWidth*defaultAspect))
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

	usePlayer := state.playerReady && state.player != nil

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(src.DurationString())
	totalTime.Alignment = fyne.TextAlignTrailing

	var controls fyne.CanvasObject
	if usePlayer {
		slider := widget.NewSlider(0, math.Max(1, src.Duration))
		slider.Step = 0.5
		slider.OnChanged = func(val float64) {
			currentTime.SetText(formatClock(val))
			if state.player != nil && state.playerReady {
				if err := state.player.Seek(val); err != nil {
					debugLog(logCatFFMPEG, "player seek failed: %v", err)
				}
			}
		}
		volSlider := widget.NewSlider(0, 100)
		volSlider.Step = 1
		volSlider.Value = state.playerVolume
		volSlider.OnChanged = func(val float64) {
			state.playerVolume = val
			if state.player != nil && state.playerReady {
				if err := state.player.SetVolume(val); err != nil {
					debugLog(logCatFFMPEG, "player volume failed: %v", err)
				}
			}
		}
		volSlider.Refresh()
		playBtn := makeIconButton("▶/⏸", "Play/Pause", func() {
			if state.player == nil {
				return
			}
			if state.playerPaused {
				if err := state.player.Play(); err != nil {
					debugLog(logCatFFMPEG, "player play failed: %v", err)
					return
				}
				state.playerPaused = false
			} else {
				if err := state.player.Pause(); err != nil {
					debugLog(logCatFFMPEG, "player pause failed: %v", err)
					return
				}
				state.playerPaused = true
			}
		})
		fullBtn := makeIconButton("⛶", "Toggle fullscreen", func() {
			if state.player != nil {
				if err := state.player.FullScreen(); err != nil {
					debugLog(logCatFFMPEG, "player fullscreen failed: %v", err)
				}
			}
		})
		volBox := container.NewHBox(widget.NewLabel("🔊"), container.NewMax(volSlider))
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
	s.convert.OutputBase = strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	s.convert.CoverArtPath = ""
	s.convert.AspectHandling = "Auto"
	if s.player != nil {
		if err := s.player.Load(src.Path, 0); err != nil {
			debugLog(logCatFFMPEG, "player load failed: %v", err)
			s.playerReady = false
		} else {
			s.playerReady = true
			s.playerPaused = false
			// Apply remembered volume for new loads.
			if err := s.player.SetVolume(s.playerVolume); err != nil {
				debugLog(logCatFFMPEG, "player set volume failed: %v", err)
			}
		}
	}
	debugLog(logCatModule, "video loaded %+v", src)
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(src)
	}, false)
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
