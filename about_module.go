package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/about"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
)

func (s *appState) showAbout() {
	t := i18n.T()

	var logoRes fyne.Resource
	if data, err := logoAssets.ReadFile("assets/logo/VT_logo.svg"); err == nil {
		logoRes = fyne.NewStaticResource("VT_logo.svg", data)
	}

	about.Show(about.Options{
		LogoResource:    logoRes,
		Window:          s.window,
		Version:         fmt.Sprintf("VideoTools %s", versionWithPlatform()),
		Developer:       "Leak Technologies",
		LogsPath:        getLogsDir(),
		TextColor:       textColor,
		OpenFolder:      openFolder,
		OpenURL:         openURL,
		QRURL:           "https://git.leaktechnologies.dev/leak_technologies/VideoTools/releases",
		WebsiteURL:      "https://leaktechnologies.dev",
		XProfileURL:     "https://x.com/VT_VideoTools",
		XLabel:          "X: @VT_VideoTools",
		TitleLabel:      t.MenuAbout,
		LogsFolderLabel: t.AboutLogsFolder,
		ScanDocsLabel:   t.AboutScanForDocs,
		FeedbackLabel:   t.AboutFeedback,
		CloseLabel:      t.AboutClose,
		OpenLabel:       t.ActionOpen,
	})
}
