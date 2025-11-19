package main

import (
	"bufio"
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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
	a := app.New()
	debugLog(logCatUI, "created fyne app: %#v", a)
	w := a.NewWindow("VideoTools")
	w.Resize(fyne.NewSize(920, 540))
	debugLog(logCatUI, "window initialized (size 920x540)")

	menu := buildMainMenu()
	bg := canvas.NewRectangle(backgroundColor)
	bg.SetMinSize(fyne.NewSize(920, 540))
	w.SetContent(container.NewMax(bg, menu))
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

func buildMainMenu() fyne.CanvasObject {
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
		tileObjects = append(tileObjects, buildModuleTile(mod))
	}

	grid := container.NewGridWithColumns(3, tileObjects...)

	padding := canvas.NewRectangle(color.Transparent)
	padding.SetMinSize(fyne.NewSize(0, 14))

	body := container.New(layout.NewVBoxLayout(),
		header,
		padding,
		grid,
	)

	return container.NewPadded(body)
}

func buildModuleTile(mod Module) fyne.CanvasObject {
	debugLog(logCatUI, "building tile %s color=%v", mod.ID, mod.Color)
	rect := canvas.NewRectangle(mod.Color)
	rect.CornerRadius = 8
	rect.StrokeColor = gridColor
	rect.StrokeWidth = 1
	rect.SetMinSize(fyne.NewSize(220, 110))

	label := canvas.NewText(strings.ToUpper(mod.Label), textColor)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	label.TextSize = 20

	tile := container.NewMax(rect, container.NewCenter(label))
	return container.NewPadded(tile)
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
