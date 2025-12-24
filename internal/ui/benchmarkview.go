package ui

import (
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/stu/VideoTools/internal/sysinfo"
)

// BuildBenchmarkProgressView creates the benchmark progress UI
func BuildBenchmarkProgressView(
	hwInfo sysinfo.HardwareInfo,
	onCancel func(),
	titleColor, bgColor, textColor color.Color,
) *BenchmarkProgressView {
	view := &BenchmarkProgressView{
		hwInfo:     hwInfo,
		titleColor: titleColor,
		bgColor:    bgColor,
		textColor:  textColor,
		onCancel:   onCancel,
	}
	view.build()
	return view
}

// BenchmarkProgressView shows real-time benchmark progress
type BenchmarkProgressView struct {
	hwInfo     sysinfo.HardwareInfo
	titleColor color.Color
	bgColor    color.Color
	textColor  color.Color
	onCancel   func()

	container    *fyne.Container
	statusLabel  *widget.Label
	progressBar  *widget.ProgressBar
	currentLabel *widget.Label
	resultsBox   *fyne.Container
	cancelBtn    *widget.Button
}

func (v *BenchmarkProgressView) build() {
	// Header
	title := canvas.NewText("ENCODER BENCHMARK", v.titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	v.cancelBtn = widget.NewButton("Cancel", v.onCancel)
	v.cancelBtn.Importance = widget.DangerImportance

	header := container.NewBorder(
		nil, nil,
		nil,
		v.cancelBtn,
		container.NewCenter(title),
	)

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

	hwContent := container.NewVBox(
		hwInfoTitle,
		cpuLabel,
		gpuLabel,
		ramLabel,
		driverLabel,
	)

	hwInfoSection := container.NewPadded(
		container.NewMax(hwCard, hwContent),
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

	// Results section
	resultsTitle := widget.NewLabel("Results")
	resultsTitle.TextStyle = fyne.TextStyle{Bold: true}
	resultsTitle.Alignment = fyne.TextAlignCenter

	v.resultsBox = container.NewVBox()
	resultsScroll := container.NewVScroll(v.resultsBox)
	resultsScroll.SetMinSize(fyne.NewSize(0, 300))

	resultsSection := container.NewBorder(
		resultsTitle,
		nil, nil, nil,
		resultsScroll,
	)

	// Main layout
	body := container.NewBorder(
		header,
		nil, nil, nil,
		container.NewVBox(
			hwInfoSection,
			widget.NewSeparator(),
			statusSection,
			widget.NewSeparator(),
			resultsSection,
		),
	)

	v.container = container.NewPadded(body)
}

// GetContainer returns the main container
func (v *BenchmarkProgressView) GetContainer() *fyne.Container {
	return v.container
}

// UpdateProgress updates the progress bar and labels
func (v *BenchmarkProgressView) UpdateProgress(current, total int, encoder, preset string) {
	pct := (float64(current) / float64(total)) * 100 // Convert to 0-100 range
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.progressBar.SetValue(pct)
		v.statusLabel.SetText(fmt.Sprintf("Testing encoder %d of %d", current, total))
		v.currentLabel.SetText(fmt.Sprintf("Testing: %s (preset: %s)", encoder, preset))
		v.progressBar.Refresh()
		v.statusLabel.Refresh()
		v.currentLabel.Refresh()
	}, false)
}

// AddResult adds a completed test result to the display
func (v *BenchmarkProgressView) AddResult(result benchmark.Result) {
	var statusColor color.Color
	var statusText string

	if result.Error != "" {
		statusColor = color.RGBA{R: 255, G: 68, B: 68, A: 255} // Red
		statusText = fmt.Sprintf("FAILED: %s", result.Error)
	} else {
		statusColor = color.RGBA{R: 76, G: 232, B: 112, A: 255} // Green
		statusText = fmt.Sprintf("%.1f FPS | %.1fs encoding time", result.FPS, result.EncodingTime)
	}

	// Status indicator
	statusRect := canvas.NewRectangle(statusColor)
	statusRect.SetMinSize(fyne.NewSize(6, 0))

	// Encoder label
	encoderLabel := widget.NewLabel(fmt.Sprintf("%s (%s)", result.Encoder, result.Preset))
	encoderLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Status label
	statusLabel := widget.NewLabel(statusText)
	statusLabel.Wrapping = fyne.TextWrapWord

	// Card content
	content := container.NewBorder(
		nil, nil,
		statusRect,
		nil,
		container.NewVBox(encoderLabel, statusLabel),
	)

	// Card background
	card := canvas.NewRectangle(v.bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, content),
	)

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.resultsBox.Add(item)
		v.resultsBox.Refresh()
	}, false)
}

// SetComplete marks the benchmark as complete
func (v *BenchmarkProgressView) SetComplete() {
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.statusLabel.SetText("Benchmark complete!")
		v.progressBar.SetValue(100.0)
		v.currentLabel.SetText("")
		v.cancelBtn.SetText("Close")
		v.statusLabel.Refresh()
		v.progressBar.Refresh()
		v.currentLabel.Refresh()
		v.cancelBtn.Refresh()
	}, false)
}

// BuildBenchmarkResultsView creates the final results/recommendation UI
func BuildBenchmarkResultsView(
	results []benchmark.Result,
	recommendation benchmark.Result,
	hwInfo sysinfo.HardwareInfo,
	onApply func(),
	onClose func(),
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Header
	title := canvas.NewText("BENCHMARK RESULTS", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	closeBtn := widget.NewButton("Close", onClose)
	closeBtn.Importance = widget.LowImportance

	header := container.NewBorder(
		nil, nil,
		nil,
		closeBtn,
		container.NewCenter(title),
	)

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

	hwContent := container.NewVBox(
		hwInfoTitle,
		cpuLabel,
		gpuLabel,
		ramLabel,
		driverLabel,
	)

	hwInfoSection := container.NewPadded(
		container.NewMax(hwCard, hwContent),
	)

	// Recommendation section
	if recommendation.Encoder != "" {
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

		recContent := container.NewVBox(
			recTitle,
			recEncoder,
			recStats,
			container.NewCenter(applyBtn),
		)

		recommendationSection := container.NewPadded(
			container.NewMax(recCard, recContent),
		)

		// Top results list
		topResultsTitle := widget.NewLabel("Top Encoders")
		topResultsTitle.TextStyle = fyne.TextStyle{Bold: true}
		topResultsTitle.Alignment = fyne.TextAlignCenter

		var filtered []benchmark.Result
		for _, result := range results {
			if result.Error == "" {
				filtered = append(filtered, result)
			}
		}

		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Score > filtered[j].Score
		})

		var resultItems []fyne.CanvasObject
		for i, result := range filtered {
			rankLabel := widget.NewLabel(fmt.Sprintf("#%d", i+1))
			rankLabel.TextStyle = fyne.TextStyle{Bold: true}

			encoderLabel := widget.NewLabel(fmt.Sprintf("%s (%s)", result.Encoder, result.Preset))

			statsLabel := widget.NewLabel(fmt.Sprintf("%.1f FPS | %.1fs | Score: %.1f",
				result.FPS, result.EncodingTime, result.Score))
			statsLabel.TextStyle = fyne.TextStyle{Italic: true}

			content := container.NewBorder(
				nil, nil,
				rankLabel,
				nil,
				container.NewVBox(encoderLabel, statsLabel),
			)

			card := canvas.NewRectangle(bgColor)
			card.CornerRadius = 4

			item := container.NewPadded(
				container.NewMax(card, content),
			)

			resultItems = append(resultItems, item)
		}

		resultsBox := container.NewVBox(resultItems...)
		resultsScroll := container.NewVScroll(resultsBox)
		resultsScroll.SetMinSize(fyne.NewSize(0, 300))

		resultsSection := container.NewBorder(
			topResultsTitle,
			nil, nil, nil,
			resultsScroll,
		)

		// Main layout
		body := container.NewBorder(
			header,
			nil, nil, nil,
			container.NewVBox(
				hwInfoSection,
				widget.NewSeparator(),
				recommendationSection,
				widget.NewSeparator(),
				resultsSection,
			),
		)

		return container.NewPadded(body)
	}

	// No results case
	emptyMsg := widget.NewLabel("No benchmark results available")
	emptyMsg.Alignment = fyne.TextAlignCenter

	body := container.NewBorder(
		header,
		nil, nil, nil,
		container.NewCenter(emptyMsg),
	)

	return container.NewPadded(body)
}

// BuildBenchmarkHistoryView creates the benchmark history browser UI
func BuildBenchmarkHistoryView(
	history []BenchmarkHistoryRun,
	onSelectRun func(int),
	onClose func(),
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Header
	title := canvas.NewText("BENCHMARK HISTORY", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	closeBtn := widget.NewButton("← Back", onClose)
	closeBtn.Importance = widget.LowImportance

	header := container.NewBorder(
		nil, nil,
		closeBtn,
		nil,
		container.NewCenter(title),
	)

	if len(history) == 0 {
		emptyMsg := widget.NewLabel("No benchmark history yet.\n\nRun your first benchmark to see results here.")
		emptyMsg.Alignment = fyne.TextAlignCenter
		emptyMsg.Wrapping = fyne.TextWrapWord

		body := container.NewBorder(
			header,
			nil, nil, nil,
			container.NewCenter(emptyMsg),
		)

		return container.NewPadded(body)
	}

	// Build list of benchmark runs
	var runItems []fyne.CanvasObject
	for i, run := range history {
		idx := i // Capture for closure
		runItems = append(runItems, buildHistoryRunItem(run, idx, onSelectRun, bgColor, textColor))
	}

	runsList := container.NewVBox(runItems...)
	runsScroll := container.NewVScroll(runsList)
	runsScroll.SetMinSize(fyne.NewSize(0, 400))

	infoLabel := widget.NewLabel("Click on a benchmark run to view detailed results")
	infoLabel.Alignment = fyne.TextAlignCenter
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}

	body := container.NewBorder(
		header,
		container.NewVBox(widget.NewSeparator(), infoLabel),
		nil, nil,
		runsScroll,
	)

	return container.NewPadded(body)
}

// BenchmarkHistoryRun represents a benchmark run in the history view
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
	bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Timestamp label
	timeLabel := widget.NewLabel(run.Timestamp)
	timeLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Recommendation info
	recLabel := widget.NewLabel(fmt.Sprintf("Recommended: %s (%s) - %.1f FPS",
		run.RecommendedEncoder, run.RecommendedPreset, run.RecommendedFPS))

	// Result count
	countLabel := widget.NewLabel(fmt.Sprintf("%d encoders tested", run.ResultCount))
	countLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Content
	content := container.NewVBox(
		timeLabel,
		recLabel,
		countLabel,
	)

	// Card background
	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, content),
	)

	// Make it tappable
	tappable := NewTappable(item, func() {
		onSelect(index)
	})

	return tappable
}
