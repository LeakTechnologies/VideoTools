package subtitles

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const ModuleColor = "#9C27B0"

func BuildView(cb ViewCallbacks) fyne.CanvasObject {
	t := i18n.T()

	backBtn := widget.NewButton("< "+t.ModuleSubtitles, func() {
		cb.ShowMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		cb.ShowQueue()
	})
	cb.SetQueueBtn(queueBtn)
	cb.UpdateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		cb.ClearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	subtitlesColor := utils.MustHex(ModuleColor)
	topBar := ui.TintedBar(subtitlesColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	content := container.NewVBox()
	content.Add(widget.NewLabel(t.ModuleSubtitles))

	var bottomBar fyne.CanvasObject
	if cb.StatsBar() != nil {
		bottomBar = container.NewHBox(layout.NewSpacer(), cb.StatsBar())
	} else {
		bottomBar = container.NewHBox(layout.NewSpacer())
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
