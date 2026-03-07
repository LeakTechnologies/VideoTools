package main

import (
	"fmt"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/about"
)

func (s *appState) showAbout() {
	about.Show(about.Options{
		Window:      s.window,
		Version:     fmt.Sprintf("VideoTools %s", versionWithPlatform()),
		Developer:   "Leak Technologies",
		LogsPath:    getLogsDir(),
		TextColor:   textColor,
		OpenFolder:  openFolder,
		OpenURL:     openURL,
		DocsURL:     "https://docs.leaktechnologies.dev/VideoTools",
		XProfileURL: "https://x.com/VT_VideoTools",
		XLabel:      "X: @VT_VideoTools",
	})
}
