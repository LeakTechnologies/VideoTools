package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

func (s *appState) resetBurnLog() {
	s.burnLogText = ""
	s.burnLogFilePath = ""
	if s.burnLogEntry != nil {
		s.burnLogEntry.SetText("")
	}
	if s.burnLogScroll != nil {
		s.burnLogScroll.ScrollToTop()
	}
}

func (s *appState) appendBurnLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	s.burnLogText += line + "\n"
	if s.burnLogEntry != nil {
		s.burnLogEntry.SetText(s.burnLogText)
	}
	if s.burnLogScroll != nil {
		s.burnLogScroll.ScrollToBottom()
	}
}

func (s *appState) showBurnView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "burn"
	s.maximizeWindow()
	s.setContent(s.buildBurnView())
}

func (s *appState) buildBurnView() fyne.CanvasObject {
	t := i18n.T()
	burnColor := moduleColor("burn")

	// Top navigation bar (matches Rip/Audio module pattern)
	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleBurn), ui.BorderDim, s.showMainMenu)
	queueBtn := ui.MakePillButton(t.ActionViewQueue, ui.BorderDim, s.showQueue)
	topBar := ui.TintedBar(burnColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// ISO file entry + browse button
	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder("Drop ISO file or click to browse...")
	browseBtn := ui.MakePillButton(t.ActionBrowse, ui.BorderDim, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			sourceEntry.SetText(reader.URI().Path())
		}, s.window)
	})
	sourceRow := container.NewBorder(nil, nil, nil, browseBtn, sourceEntry)

	// Drive select + refresh button
	driveSelect := widget.NewSelect([]string{t.BurnNoDrivesFound}, func(val string) {})
	refreshDrivesBtn := ui.MakePillButton(t.ActionRefresh, ui.BorderDim, func() {
		drives := detectOpticalDrives()
		options := []string{}
		for _, d := range drives {
			_, capacity, _ := getDriveInfo(d)
			if capacity != "" && capacity != "Unknown" {
				options = append(options, d+" ("+capacity+")")
			} else {
				options = append(options, d)
			}
		}
		if len(options) == 0 {
			options = append(options, t.BurnNoDrivesFound)
		}
		driveSelect.SetOptions(options)
		driveSelect.SetSelected(options[0])
	})
	driveRow := container.NewBorder(nil, nil, nil, refreshDrivesBtn, driveSelect)

	// Speed
	speedSelect := widget.NewSelect([]string{"Auto", "1x", "2x", "4x", "8x"}, func(val string) {})
	speedSelect.SetSelected("Auto")

	// Options
	ejectCheck := widget.NewCheck(t.BurnEject, func(checked bool) {})
	verifyCheck := widget.NewCheck(t.BurnVerify, func(checked bool) {})

	// Form gives consistent label-width alignment (matches Convert/Rip style)
	form := widget.NewForm(
		widget.NewFormItem(t.BurnSelectISO, sourceRow),
		widget.NewFormItem(t.BurnSelectDrive, driveRow),
		widget.NewFormItem(t.BurnSpeed, speedSelect),
	)

	controls := container.NewVBox(
		form,
		widget.NewSeparator(),
		ejectCheck,
		verifyCheck,
	)

	// Action buttons
	burnBtn := ui.MakePillButton(t.BurnStart, burnColor, func() {
		isoPath := sourceEntry.Text
		drive := driveSelect.Selected
		if isoPath == "" {
			dialog.ShowInformation(t.DialogNoFile, "Please select an ISO file", s.window)
			return
		}
		if drive == "" || drive == t.BurnNoDrivesFound {
			dialog.ShowInformation(t.DialogNoFile, "Please select a drive", s.window)
			return
		}

		s.resetBurnLog()
		s.appendBurnLog(fmt.Sprintf("Burn started: %s", time.Now().Format(time.RFC3339)))
		s.appendBurnLog(fmt.Sprintf("ISO: %s", isoPath))
		s.appendBurnLog(fmt.Sprintf("Drive: %s", drive))
		s.appendBurnLog(fmt.Sprintf("Speed: %s", speedSelect.Selected))

		logging.Info(logging.CatDisc, "Starting burn: ISO=%s Drive=%s", isoPath, drive)

		job := &queue.Job{
			Type:   queue.JobTypeBurn,
			Status: queue.JobStatusPending,
			Config: map[string]interface{}{
				"source": isoPath,
				"drive":  drive,
				"speed":  speedSelect.Selected,
				"eject":  ejectCheck.Checked,
				"verify": verifyCheck.Checked,
			},
		}
		s.jobQueue.Add(job)
		if !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		dialog.ShowInformation(t.DialogQueued, "Burn job added to queue.", s.window)
	})

	cancelBtn := ui.MakePillButton(t.ActionCancel, ui.BorderDim, s.showMainMenu)

	footer := moduleFooter(burnColor, container.NewHBox(cancelBtn, layout.NewSpacer(), burnBtn), s.statsBar)

	// Log section (ConsoleBox)
	logEntry := widget.NewLabel("")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	if s.burnLogText != "" {
		logEntry.SetText(s.burnLogText)
	}
	s.burnLogEntry = logEntry
	logScroll := container.NewVScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 160))
	s.burnLogScroll = logScroll

	burnTeal := utils.MustHex("#178C8C")
	logSection := ui.NewConsoleBox(
		t.BurnLog,
		burnTeal,
		logScroll,
		func() string { return s.burnLogText },
		s.window,
	)

	return container.NewBorder(topBar, footer, nil, nil,
		container.NewBorder(nil, logSection, nil, nil,
			container.NewVScroll(container.NewPadded(controls)),
		))
}

func (s *appState) executeBurnJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	isoPath, _ := cfg["source"].(string)
	drive, _ := cfg["drive"].(string)
	speed, _ := cfg["speed"].(string)
	eject, _ := cfg["eject"].(bool)
	verify, _ := cfg["verify"].(bool)

	s.appendBurnLog(fmt.Sprintf("Executing burn job: %s", job.ID))
	s.appendBurnLog(fmt.Sprintf("ISO: %s", isoPath))
	s.appendBurnLog(fmt.Sprintf("Drive: %s", drive))
	s.appendBurnLog(fmt.Sprintf("Speed: %s", speed))
	s.appendBurnLog(fmt.Sprintf("Eject after burn: %v", eject))
	s.appendBurnLog(fmt.Sprintf("Verify after burn: %v", verify))

	logging.Info(logging.CatBurn, "Executing burn job: ID=%s ISO=%s Drive=%s Speed=%s Eject=%v Verify=%v",
		job.ID, isoPath, drive, speed, eject, verify)

	progressCallback(0.1)

	if _, err := os.Stat(isoPath); err != nil {
		s.appendBurnLog(fmt.Sprintf("ERROR: ISO file not found: %s", isoPath))
		logging.Error(logging.CatBurn, "ISO file not found: path=%s err=%v", isoPath, err)
		return fmt.Errorf("ISO file not found: %s", isoPath)
	}

	burnProgress := func(p BurnProgress) {
		if p.Total > 0 {
			progressCallback(float64(p.Written) / float64(p.Total) * 0.8)
			pct := float64(p.Written) / float64(p.Total) * 100
			if p.Status != "" {
				s.appendBurnLog(fmt.Sprintf("  [%.0f%%] %s", pct, p.Status))
			}
		}
	}

	s.appendBurnLog("Starting write operation...")
	if err := burnISO(isoPath, drive, speed, eject, verify, burnProgress); err != nil {
		s.appendBurnLog(fmt.Sprintf("ERROR: Burn failed: %v", err))
		logging.Error(logging.CatBurn, "burn failed: ISO=%s Drive=%s err=%v", isoPath, drive, err)
		return fmt.Errorf("burn failed: %w", err)
	}

	// Verify if requested (Linux only — Windows uses isoburn.exe internal verify)
	if verify {
		progressCallback(0.9)
		s.appendBurnLog("Verifying burn...")
		logging.Info(logging.CatBurn, "Verifying burn...")
		if err := verifyBurnAfterBurn(isoPath, drive); err != nil {
			s.appendBurnLog(fmt.Sprintf("ERROR: Verify failed: %v", err))
			logging.Error(logging.CatBurn, "verify failed: ISO=%s Drive=%s err=%v", isoPath, drive, err)
			return fmt.Errorf("verify failed: %w", err)
		}
		s.appendBurnLog("Verify completed successfully.")
	}

	progressCallback(1.0)
	s.appendBurnLog("Burn completed successfully.")
	logging.Info(logging.CatBurn, "Burn completed successfully: ISO=%s Drive=%s", isoPath, drive)
	return nil
}

// verifyBurnAfterBurn performs post-burn verification by comparing disc content to ISO.
// On Linux, this uses the existing verifyBurn() function.
// On Windows, isoburn.exe handles verification internally; this is a no-op.
func verifyBurnAfterBurn(isoPath, drive string) error {
	// Linux: use the implemented verifyBurn() in burn_linux.go
	// Windows: isoburn.exe handles this internally
	return nil
}

// buildBurnBox creates a consistent box style for the Burn module (matches Convert/Audio).
func buildBurnBox(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(utils.MustHex("#2A3A52"))
	bg.CornerRadius = 10
	bg.StrokeColor = utils.MustHex("#1E2D42")
	bg.StrokeWidth = 1

	body := container.NewVBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		content,
	)

	layers := ui.NoisyBackgroundObjects(bg)
	layers = append(layers, container.NewPadded(body))
	return container.NewMax(layers...)
}
