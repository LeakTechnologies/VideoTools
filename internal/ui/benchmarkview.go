package ui

import (
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/sysinfo"
)

// benchmarkModuleFooter builds the standard module footer used by all benchmark
// views: a dark status strip (32 px) above a tinted action bar (44 px).
func benchmarkModuleFooter(tint color.Color, actionContent fyne.CanvasObject, statsBar *ConversionStatsBar) fyne.CanvasObject {
	statusBg := canvas.NewRectangle(color.NRGBA{R: 34, G: 34, B: 34, A: 255})
	statusBg.SetMinSize(fyne.NewSize(0, 32))
	statusStrip := container.NewMax(statusBg, container.NewPadded(statsBar))

	if actionContent == nil {
		actionContent = layout.NewSpacer()
	}
	actionBg := canvas.NewRectangle(tint)
	actionBg.SetMinSize(fyne.NewSize(0, 44))
	actionBar := container.NewMax(actionBg, container.NewPadded(actionContent))

	return container.NewVBox(statusStrip, actionBar)
}

// BuildBenchmarkProgressView creates the benchmark progress UI.
func BuildBenchmarkProgressView(
	hwInfo sysinfo.HardwareInfo,
	onBack func(),
	headerColor, bgColor color.Color,
	statsBar *ConversionStatsBar,
) *BenchmarkProgressView {
	view := &BenchmarkProgressView{
		hwInfo:      hwInfo,
		headerColor: headerColor,
		bgColor:     bgColor,
		statsBar:    statsBar,
		onBack:      onBack,
	}
	view.build()
	return view
}

// BenchmarkProgressView shows real-time benchmark progress.
type BenchmarkProgressView struct {
	hwInfo      sysinfo.HardwareInfo
	headerColor color.Color
	bgColor     color.Color
	statsBar    *ConversionStatsBar
	onBack      func()

	container    *fyne.Container
	statusLabel  *widget.Label
	progressBar  *widget.ProgressBar
	currentLabel *widget.Label
	resultsBox   *fyne.Container
}

func (v *BenchmarkProgressView) build() {
	// Header bar
	backBtn := DarkTextButton("< BENCHMARK", v.onBack)
	topBar := TintedBar(v.headerColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Hardware info section
	hwInfoTitle := widget.NewLabel("System Hardware")
	hwInfoTitle.TextStyle = fyne.TextStyle{Bold: true}
	hwInfoTitle.Alignment = fyne.TextAlignCenter

	cpuLabel := widget.NewLabel(fmt.Sprintf("CPU: %s (%d cores @ %s)", v.hwInfo.CPU, v.hwInfo.CPUCores, v.hwInfo.CPUMHz))
	cpuLabel.Wrapping = fyne.TextWrapWord

	gpuLabel := widget.NewLabel(fmt.Sprintf("GPU: %s", v.hwInfo.GPU))
	gpuLabel.Wrapping = fyne.TextWrapWord

	ramLabel := widget.NewLabel(fmt.Sprintf("RAM: %s", v.hwInfo.RAM))

	driverLabel := widget.NewLabel(fmt.Sprintf("Driver: %s", v.hwInfo.GPUDriver))
	driverLabel.Wrapping = fyne.TextWrapWord

	hwCard := canvas.NewRectangle(color.RGBA{R: 34, G: 38, B: 48, A: 255})
	hwCard.CornerRadius = 8

	hwInfoSection := container.NewPadded(
		container.NewMax(hwCard, container.NewVBox(
			hwInfoTitle, cpuLabel, gpuLabel, ramLabel, driverLabel,
		)),
	)

	// Status section
	v.statusLabel = widget.NewLabel("Initializing benchmark...")
	v.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	v.statusLabel.Alignment = fyne.TextAlignCenter

	v.progressBar = widget.NewProgressBar()
	v.progressBar.Min = 0
	v.progressBar.Max = 100

	v.currentLabel = widget.NewLabel("")
	v.currentLabel.Alignment = fyne.TextAlignCenter
	v.currentLabel.Wrapping = fyne.TextWrapWord

	statusSection := container.NewVBox(
		v.statusLabel,
		v.progressBar,
		v.currentLabel,
	)

	// Results section — expands to fill remaining space
	resultsTitle := widget.NewLabel("Results")
	resultsTitle.TextStyle = fyne.TextStyle{Bold: true}
	resultsTitle.Alignment = fyne.TextAlignCenter

	v.resultsBox = container.NewVBox()
	resultsScroll := container.NewVScroll(v.resultsBox)

	resultsSection := container.NewBorder(resultsTitle, nil, nil, nil, resultsScroll)

	// Fixed top portion — hw info + status
	topFixed := container.NewVBox(
		hwInfoSection,
		widget.NewSeparator(),
		statusSection,
		widget.NewSeparator(),
	)

	body := container.NewBorder(
		topBar,
		benchmarkModuleFooter(v.headerColor, nil, v.statsBar),
		nil, nil,
		container.NewBorder(topFixed, nil, nil, nil, resultsSection),
	)

	v.container = container.NewMax(body)
}

// GetContainer returns the root container for this view.
func (v *BenchmarkProgressView) GetContainer() *fyne.Container {
	return v.container
}

// UpdateProgress updates the progress bar and status labels.
// label is the human-readable encoder name (e.g. "NVENC H.264").
func (v *BenchmarkProgressView) UpdateProgress(current, total int, label string) {
	pct := (float64(current) / float64(total)) * 100
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.progressBar.SetValue(pct)
		v.statusLabel.SetText(fmt.Sprintf("Testing encoder %d of %d", current, total))
		v.currentLabel.SetText(fmt.Sprintf("Testing: %s", label))
		v.progressBar.Refresh()
		v.statusLabel.Refresh()
		v.currentLabel.Refresh()
	}, false)
}

// AddResult appends a completed test result card to the live results list.
func (v *BenchmarkProgressView) AddResult(result benchmark.Result) {
	var statusColor color.Color
	var statusText string

	if result.Error != "" {
		statusColor = color.RGBA{R: 255, G: 68, B: 68, A: 255}
		statusText = "FAILED"
	} else {
		statusColor = color.RGBA{R: 76, G: 232, B: 112, A: 255}
		statusText = fmt.Sprintf("%.1f FPS | %.1fs", result.FPS, result.EncodingTime)
	}

	statusRect := canvas.NewRectangle(statusColor)

	encoderLabel := widget.NewLabel(benchmark.FriendlyName(result.Encoder))
	encoderLabel.TextStyle = fyne.TextStyle{Bold: true}

	statusLabel := widget.NewLabel(statusText)
	statusLabel.Wrapping = fyne.TextWrapWord

	content := container.NewBorder(nil, nil, statusRect, nil,
		container.NewVBox(encoderLabel, statusLabel))

	card := canvas.NewRectangle(v.bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(container.NewMax(card, content))

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.resultsBox.Add(item)
		v.resultsBox.Refresh()
	}, false)
}

// SetComplete marks the benchmark as finished.
func (v *BenchmarkProgressView) SetComplete() {
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.statusLabel.SetText("Benchmark complete!")
		v.progressBar.SetValue(100.0)
		v.currentLabel.SetText("")
		v.statusLabel.Refresh()
		v.progressBar.Refresh()
		v.currentLabel.Refresh()
	}, false)
}

// BuildBenchmarkResultsView creates the final results/recommendation UI.
// actionContent is placed in the bottom action bar; pass nil for an empty bar.
func BuildBenchmarkResultsView(
	results []benchmark.Result,
	recommendation benchmark.Result,
	hwInfo sysinfo.HardwareInfo,
	onApply func(),
	onBack func(),
	headerColor, bgColor color.Color,
	statsBar *ConversionStatsBar,
	actionContent fyne.CanvasObject,
) fyne.CanvasObject {
	// Header bar
	backBtn := DarkTextButton("< BENCHMARK", onBack)
	topBar := TintedBar(headerColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Footer bar
	footer := benchmarkModuleFooter(headerColor, actionContent, statsBar)

	// Hardware info section
	hwInfoTitle := widget.NewLabel("System Hardware")
	hwInfoTitle.TextStyle = fyne.TextStyle{Bold: true}
	hwInfoTitle.Alignment = fyne.TextAlignCenter

	cpuLabel := widget.NewLabel(fmt.Sprintf("CPU: %s (%d cores @ %s)", hwInfo.CPU, hwInfo.CPUCores, hwInfo.CPUMHz))
	cpuLabel.Wrapping = fyne.TextWrapWord

	gpuLabel := widget.NewLabel(fmt.Sprintf("GPU: %s", hwInfo.GPU))
	gpuLabel.Wrapping = fyne.TextWrapWord

	ramLabel := widget.NewLabel(fmt.Sprintf("RAM: %s", hwInfo.RAM))

	driverLabel := widget.NewLabel(fmt.Sprintf("Driver: %s", hwInfo.GPUDriver))
	driverLabel.Wrapping = fyne.TextWrapWord

	hwCard := canvas.NewRectangle(color.RGBA{R: 34, G: 38, B: 48, A: 255})
	hwCard.CornerRadius = 8

	hwInfoSection := container.NewPadded(
		container.NewMax(hwCard, container.NewVBox(
			hwInfoTitle, cpuLabel, gpuLabel, ramLabel, driverLabel,
		)),
	)

	if recommendation.Encoder == "" {
		// No results case
		emptyMsg := widget.NewLabel("No benchmark results available")
		emptyMsg.Alignment = fyne.TextAlignCenter

		return container.NewBorder(topBar, footer, nil, nil,
			container.NewBorder(hwInfoSection, nil, nil, nil,
				container.NewCenter(emptyMsg)))
	}

	// Recommendation section
	recTitle := widget.NewLabel("RECOMMENDED ACCELERATION")
	recTitle.TextStyle = fyne.TextStyle{Bold: true}
	recTitle.Alignment = fyne.TextAlignCenter

	recHWLabel := widget.NewLabel(benchmark.HWAccelLabel(recommendation.HWAccel))
	recHWLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	recHWLabel.Alignment = fyne.TextAlignCenter

	recDetail := widget.NewLabel(fmt.Sprintf("Best encoder: %s  •  %.1f FPS",
		benchmark.FriendlyName(recommendation.Encoder), recommendation.FPS))
	recDetail.Alignment = fyne.TextAlignCenter

	applyBtn := widget.NewButton("Apply to Settings", onApply)
	applyBtn.Importance = widget.HighImportance

	recCard := canvas.NewRectangle(color.RGBA{R: 68, G: 136, B: 255, A: 50})
	recCard.CornerRadius = 8

	recommendationSection := container.NewPadded(
		container.NewMax(recCard, container.NewVBox(
			recTitle, recHWLabel, recDetail,
			container.NewCenter(applyBtn),
		)),
	)

	// All encoder results, sorted by FPS (successful ones first)
	resultsTitle := widget.NewLabel("Encoder Results")
	resultsTitle.TextStyle = fyne.TextStyle{Bold: true}
	resultsTitle.Alignment = fyne.TextAlignCenter

	// Separate passing and failing results; sort each group by FPS desc
	var passed, failed []benchmark.Result
	for _, r := range results {
		if r.Error == "" {
			passed = append(passed, r)
		} else {
			failed = append(failed, r)
		}
	}
	sort.Slice(passed, func(i, j int) bool {
		return passed[i].Score > passed[j].Score
	})
	allSorted := append(passed, failed...)

	var resultItems []fyne.CanvasObject
	for i, r := range allSorted {
		var statusColor color.Color
		var statsText string
		if r.Error == "" {
			statusColor = color.RGBA{R: 76, G: 232, B: 112, A: 255}
			statsText = fmt.Sprintf("%.1f FPS | %.1fs", r.FPS, r.EncodingTime)
		} else {
			statusColor = color.RGBA{R: 255, G: 68, B: 68, A: 255}
			statsText = "FAILED"
		}

		rankLabel := widget.NewLabel(fmt.Sprintf("#%d", i+1))
		rankLabel.TextStyle = fyne.TextStyle{Bold: true}

		statusDot := canvas.NewRectangle(statusColor)
		statusDot.SetMinSize(fyne.NewSize(6, 0))

		nameLabel := widget.NewLabel(benchmark.FriendlyName(r.Encoder))
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}

		statsLabel := widget.NewLabel(statsText)
		statsLabel.TextStyle = fyne.TextStyle{Italic: true}

		content := container.NewBorder(nil, nil,
			container.NewHBox(rankLabel, statusDot),
			nil,
			container.NewVBox(nameLabel, statsLabel),
		)

		card := canvas.NewRectangle(bgColor)
		card.CornerRadius = 4

		resultItems = append(resultItems, container.NewPadded(container.NewMax(card, content)))
	}

	resultsBox := container.NewVBox(resultItems...)
	resultsScroll := container.NewVScroll(resultsBox)
	resultsSection := container.NewBorder(resultsTitle, nil, nil, nil, resultsScroll)

	// Fixed top: hw info + recommendation; results scroll expands below
	topFixed := container.NewVBox(
		hwInfoSection,
		widget.NewSeparator(),
		recommendationSection,
		widget.NewSeparator(),
	)

	return container.NewBorder(
		topBar,
		footer,
		nil, nil,
		container.NewBorder(topFixed, nil, nil, nil, resultsSection),
	)
}

// BuildBenchmarkHistoryView creates the benchmark history browser UI.
func BuildBenchmarkHistoryView(
	history []BenchmarkHistoryRun,
	onSelectRun func(int),
	onBack func(),
	headerColor, bgColor color.Color,
	statsBar *ConversionStatsBar,
) fyne.CanvasObject {
	// Header bar
	backBtn := DarkTextButton("< BENCHMARK", onBack)
	topBar := TintedBar(headerColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Footer bar
	footer := benchmarkModuleFooter(headerColor, nil, statsBar)

	if len(history) == 0 {
		emptyMsg := widget.NewLabel("No benchmark history yet.\n\nRun your first benchmark to see results here.")
		emptyMsg.Alignment = fyne.TextAlignCenter
		emptyMsg.Wrapping = fyne.TextWrapWord

		return container.NewBorder(topBar, footer, nil, nil,
			container.NewCenter(emptyMsg))
	}

	var runItems []fyne.CanvasObject
	for i, run := range history {
		idx := i
		runItems = append(runItems, buildHistoryRunItem(run, idx, onSelectRun, bgColor))
	}

	runsList := container.NewVBox(runItems...)
	runsScroll := container.NewVScroll(runsList)

	infoLabel := widget.NewLabel("Click on a benchmark run to view detailed results")
	infoLabel.Alignment = fyne.TextAlignCenter
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}

	return container.NewBorder(
		topBar,
		footer,
		nil, nil,
		container.NewBorder(nil, container.NewVBox(widget.NewSeparator(), infoLabel), nil, nil,
			runsScroll),
	)
}

// BenchmarkHistoryRun represents a benchmark run in the history view.
type BenchmarkHistoryRun struct {
	Timestamp          string
	ResultCount        int
	RecommendedHWAccel string // "nvenc", "amf", "qsv", or "none"
	RecommendedFPS     float64
}

func buildHistoryRunItem(
	run BenchmarkHistoryRun,
	index int,
	onSelect func(int),
	bgColor color.Color,
) fyne.CanvasObject {
	timeLabel := widget.NewLabel(run.Timestamp)
	timeLabel.TextStyle = fyne.TextStyle{Bold: true}

	recLabel := widget.NewLabel(fmt.Sprintf("Recommended: %s  •  %.1f FPS",
		benchmark.HWAccelLabel(run.RecommendedHWAccel), run.RecommendedFPS))

	countLabel := widget.NewLabel(fmt.Sprintf("%d encoders tested", run.ResultCount))
	countLabel.TextStyle = fyne.TextStyle{Italic: true}

	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, container.NewVBox(timeLabel, recLabel, countLabel)),
	)

	return NewTappable(item, func() { onSelect(index) })
}
