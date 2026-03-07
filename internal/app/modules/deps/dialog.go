package deps

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

type Dependency struct {
	Name       string
	InstallCmd string
}

// ShowMissingDependencies renders the missing-dependency dialog for a module.
func ShowMissingDependencies(window fyne.Window, missing []Dependency) {
	if len(missing) == 0 {
		return
	}

	var message strings.Builder
	message.WriteString("This module requires the following dependencies:\n\n")

	for _, dep := range missing {
		message.WriteString(fmt.Sprintf(" %s\n", dep.Name))
		if strings.TrimSpace(dep.InstallCmd) != "" {
			message.WriteString(fmt.Sprintf("  Install: %s\n\n", dep.InstallCmd))
		}
	}

	dialog.ShowInformation("Missing Dependencies", message.String(), window)
}
