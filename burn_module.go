package main

import (
	"context"
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
)

func (s *appState) showBurnView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "burn"
	s.maximizeWindow()
	s.setContent(s.buildBurnView())
}

func (s *appState) buildBurnView() fyne.CanvasObject {
	t := i18n.T()

	header := widget.NewLabelWithStyle(t.ModuleBurn, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	sourceLabel := widget.NewLabel(t.BurnSelectISO)
	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder("Drop ISO file or click to browse...")

	browseBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFileOpen(func(uri fyne.URI, err error) {
			if err != nil || uri == nil {
				return
			}
			sourceEntry.SetText(uri.Path())
		}, s.window)
	})

	driveLabel := widget.NewLabel(t.BurnSelectDrive)
	driveSelect := widget.NewSelect([]string{t.BurnNoDrivesFound}, func(val string) {})

	refreshDrivesBtn := widget.NewButton(t.ActionRefresh, func() {
		drives := detectOpticalDrives()
		options := []string{}
		for _, d := range drives {
			options = append(options, d)
		}
		if len(options) == 0 {
			options = append(options, t.BurnNoDrivesFound)
		}
		driveSelect.SetOptions(options)
		driveSelect.SetSelected(options[0])
	})

	speedLabel := widget.NewLabel(t.BurnSpeed)
	speedSelect := widget.NewSelect([]string{"Auto", "1x", "2x", "4x", "8x"}, func(val string) {})
	speedSelect.SetSelected("Auto")

	ejectCheck := widget.NewCheck(t.BurnEject, func(checked bool) {})

	burnBtn := widget.NewButton(t.BurnStart, func() {
		isoPath := sourceEntry.Text()
		drive := driveSelect.Selected
		if isoPath == "" {
			dialog.ShowInformation(t.DialogNoFile, "Please select an ISO file", s.window)
			return
		}
		if drive == "" || drive == t.BurnNoDrivesFound {
			dialog.ShowInformation(t.DialogNoFile, "Please select a drive", s.window)
			return
		}

		logging.Info(logging.CatDisc, "Starting burn: ISO=%s Drive=%s", isoPath, drive)

		job := &queue.Job{
			ID:     queue.NewJobID(),
			Type:   queue.JobTypeBurn,
			Status: queue.JobStatusPending,
			Config: map[string]interface{}{
				"source": isoPath,
				"drive":  drive,
				"speed":  speedSelect.Selected,
				"eject":  ejectCheck.Checked,
			},
		}
		s.jobQueue.Add(job)
		if !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		dialog.ShowInformation(t.DialogQueued, "Burn job added to queue.", s.window)
	})

	cancelBtn := widget.NewButton(t.ActionCancel, func() {
		s.showMainMenu()
	})

	footer := moduleFooter(moduleColor("burn"), container.NewHBox(cancelBtn, burnBtn), s.statsBar)

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		container.NewHBox(sourceLabel, sourceEntry, browseBtn),
		container.NewHBox(driveLabel, driveSelect, refreshDrivesBtn),
		container.NewHBox(speedLabel, speedSelect),
		ejectCheck,
	)
	content = container.NewPadded(content)

	return container.NewBorder(content, footer, nil, nil)
}

func (s *appState) executeBurnJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	isoPath, _ := cfg["source"].(string)
	drive, _ := cfg["drive"].(string)
	speed, _ := cfg["speed"].(string)
	eject, _ := cfg["eject"].(bool)

	logging.Info(logging.CatDisc, "Executing burn job: ID=%s ISO=%s Drive=%s Speed=%s Eject=%v",
		job.ID, isoPath, drive, speed, eject)

	progressCallback(0.1)

	// Validate ISO exists
	if _, err := os.Stat(isoPath); err != nil {
		return fmt.Errorf("ISO file not found: %s", isoPath)
	}

	// Perform the burn
	if err := burnISO(isoPath, drive, speed, eject); err != nil {
		return fmt.Errorf("burn failed: %w", err)
	}

	progressCallback(1.0)
	logging.Info(logging.CatDisc, "Burn completed successfully")
	return nil
}
