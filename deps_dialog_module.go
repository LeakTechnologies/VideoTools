package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2/dialog"
)

func (s *appState) showMissingDependenciesDialog(moduleID string) {
	missing, _ := getModuleDependencyStatus(moduleID)

	if len(missing) == 0 {
		return // No missing dependencies
	}

	// Build message with missing dependencies and install commands
	var message strings.Builder
	message.WriteString("This module requires the following dependencies:\n\n")

	for _, depName := range missing {
		if dep, ok := allDependencies[depName]; ok {
			message.WriteString(fmt.Sprintf(" %s\n", dep.Name))
			if dep.InstallCmd != "" {
				message.WriteString(fmt.Sprintf("  Install: %s\n\n", dep.InstallCmd))
			}
		}
	}

	// Create dialog
	dialog.ShowInformation(
		"Missing Dependencies",
		message.String(),
		s.window,
	)
}
