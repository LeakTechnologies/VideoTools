package main

import (
	"fmt"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/about"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
)

func (s *appState) showAbout() {
	t := i18n.T()
	about.Show(about.Options{
		Window:          s.window,
		Version:         fmt.Sprintf("VideoTools %s", versionWithPlatform()),
		Developer:       "Leak Technologies",
		LogsPath:        getLogsDir(),
		TextColor:       textColor,
		OpenFolder:      openFolder,
		OpenURL:         openURL,
		DocsURL:         "https://git.leaktechnologies.dev/leak_technologies/VideoTools/wiki",
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
