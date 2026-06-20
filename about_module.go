package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/app/modules/about"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
)

func (s *appState) showAbout() {
	t := i18n.T()

	var vtLogoRes, ltLogoRes fyne.Resource
	if data, err := logoAssets.ReadFile("assets/logo/VT_logo.png"); err == nil {
		vtLogoRes = fyne.NewStaticResource("VT_logo.png", data)
	}
	if data, err := logoAssets.ReadFile("assets/logo/LT_Logo-26.png"); err == nil {
		ltLogoRes = fyne.NewStaticResource("LT_Logo-26.png", data)
	}

	about.Show(about.Options{
		LogoResource:    vtLogoRes,
		LTLogoResource:  ltLogoRes,
		Window:          s.window,
		Version:         fmt.Sprintf("VideoTools %s", versionWithPlatform()),
		Developer:       "Leak Technologies",
		LogsPath:        getLogsDir(),
		TextColor:       textColor,
		OpenFolder:      openFolder,
		OpenURL:         openURL,
		QRURL:           "https://github.com/LeakTechnologies/VideoTools/releases",
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
