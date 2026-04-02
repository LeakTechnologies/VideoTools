package deps

import (
	"fmt"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

func ShowMissingDependencies(missing []Dependency, window interface{}) {
	t := i18n.T()
	var message strings.Builder
	for _, dep := range missing {
		message.WriteString(fmt.Sprintf(" %s\n", dep.Name))
		if strings.TrimSpace(dep.InstallCmd) != "" {
			message.WriteString(fmt.Sprintf("  Install: %s\n\n", dep.InstallCmd))
		}
	}

	logging.Debug(logging.CatModule, "showing missing dependencies dialog: %d missing", len(missing))

	// Use type assertion for window - could be fyne.Window or interface{}
	switch w := window.(type) {
	case interface {
		ShowInformation(title, message string, parent interface{})
	}:
		w.ShowInformation(t.DependenciesMissing, message.String())
	default:
		// Fallback if window type doesn't match
		fmt.Println(t.DependenciesMissing + ": " + message.String())
	}
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
