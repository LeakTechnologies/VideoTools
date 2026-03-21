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
	"git.leaktechnologies.dev/stu/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/stu/VideoTools/internal/sysinfo"
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
	backBtn := widget.NewButton("< BENCHMARK", v.onBack)
	backBtn.Importance = widget.LowImportance
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
func (v *BenchmarkProgressView) UpdateProgress(current, total int, encoder, preset string) {
	pct := (float64(current) / float64(total)) * 100
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.progressBar.SetValue(pct)
		v.statusLabel.SetText(fmt.Sprintf("Testing encoder %d of %d", current, total))
		v.currentLabel.SetText(fmt.Sprintf("Testing: %s (preset: %s)", encoder, preset))
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
		statusText = fmt.Sprintf("FAILED: %s", result.Error)
	} else {
		statusColor = color.RGBA{R: 76, G: 232, B: 112, A: 255}
		statusText = fmt.Sprintf("%.1f FPS | %.1fs encoding time", result.FPS, result.EncodingTime)
	}

	statusRect := canvas.NewRectangle(statusColor)

	encoderLabel := widget.NewLabel(fmt.Sprintf("%s (%s)", result.Encoder, result.Preset))
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
	backBtn := widget.NewButton("< BENCHMARK", onBack)
	backBtn.Importance = widget.LowImportance
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
	recTitle := widget.NewLabel("RECOMMENDED ENCODER")
	recTitle.TextStyle = fyne.TextStyle{Bold: true}
	recTitle.Alignment = fyne.TextAlignCenter

	recEncoder := widget.NewLabel(fmt.Sprintf("%s (preset: %s)", recommendation.Encoder, recommendation.Preset))
	recEncoder.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	recEncoder.Alignment = fyne.TextAlignCenter

	recStats := widget.NewLabel(fmt.Sprintf("%.1f FPS | %.1fs encoding time | Score: %.1f",
		recommendation.FPS, recommendation.EncodingTime, recommendation.Score))
	recStats.Alignment = fyne.TextAlignCenter

	applyBtn := widget.NewButton("Apply to Settings", onApply)
	applyBtn.Importance = widget.HighImportance

	recCard := canvas.NewRectangle(color.RGBA{R: 68, G: 136, B: 255, A: 50})
	recCard.CornerRadius = 8

	recommendationSection := container.NewPadded(
		container.NewMax(recCard, container.NewVBox(
			recTitle, recEncoder, recStats,
			container.NewCenter(applyBtn),
		)),
	)

	// Top encoders list
	topResultsTitle := widget.NewLabel("Top Encoders")
	topResultsTitle.TextStyle = fyne.TextStyle{Bold: true}
	topResultsTitle.Alignment = fyne.TextAlignCenter

	var filtered []benchmark.Result
	for _, r := range results {
		if r.Error == "" {
			filtered = append(filtered, r)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	var resultItems []fyne.CanvasObject
	for i, r := range filtered {
		rankLabel := widget.NewLabel(fmt.Sprintf("#%d", i+1))
		rankLabel.TextStyle = fyne.TextStyle{Bold: true}

		encoderLabel := widget.NewLabel(fmt.Sprintf("%s (%s)", r.Encoder, r.Preset))

		statsLabel := widget.NewLabel(fmt.Sprintf("%.1f FPS | %.1fs | Score: %.1f",
			r.FPS, r.EncodingTime, r.Score))
		statsLabel.TextStyle = fyne.TextStyle{Italic: true}

		content := container.NewBorder(nil, nil, rankLabel, nil,
			container.NewVBox(encoderLabel, statsLabel))

		card := canvas.NewRectangle(bgColor)
		card.CornerRadius = 4

		resultItems = append(resultItems, container.NewPadded(container.NewMax(card, content)))
	}

	resultsBox := container.NewVBox(resultItems...)
	resultsScroll := container.NewVScroll(resultsBox)
	resultsSection := container.NewBorder(topResultsTitle, nil, nil, nil, resultsScroll)

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
	backBtn := widget.NewButton("< BENCHMARK", onBack)
	backBtn.Importance = widget.LowImportance
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
	RecommendedEncoder string
	RecommendedPreset  string
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

	recLabel := widget.NewLabel(fmt.Sprintf("Recommended: %s (%s) - %.1f FPS",
		run.RecommendedEncoder, run.RecommendedPreset, run.RecommendedFPS))

	countLabel := widget.NewLabel(fmt.Sprintf("%d encoders tested", run.ResultCount))
	countLabel.TextStyle = fyne.TextStyle{Italic: true}

	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, container.NewVBox(timeLabel, recLabel, countLabel)),
	)

	return NewTappable(item, func() { onSelect(index) })
}
