package deps

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// Dependency describes a single external dependency that a module requires.
type Dependency struct {
	Name       string
	InstallCmd string
}

// ShowMissingDependencies renders the missing-dependency dialog for a module.
func ShowMissingDependencies(window fyne.Window, missing []Dependency) {
	if len(missing) == 0 {
		return
	}

	t := i18n.T()
	logging.Debug(logging.CatModule, "showing missing dependencies dialog: %d missing", len(missing))

	var message strings.Builder
	for _, dep := range missing {
		message.WriteString(fmt.Sprintf(" %s\n", dep.Name))
		if strings.TrimSpace(dep.InstallCmd) != "" {
			message.WriteString(fmt.Sprintf("  Install: %s\n\n", dep.InstallCmd))
		}
	}

	dialog.ShowInformation(t.DependenciesMissing, message.String(), window)
}
