package main

import (
	depsmodule "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/deps"
)

func (s *appState) showMissingDependenciesDialog(moduleID string) {
	missing, _ := getModuleDependencyStatus(moduleID)
	payload := make([]depsmodule.Dependency, 0, len(missing))
	for _, depName := range missing {
		if dep, ok := allDependencies[depName]; ok {
			payload = append(payload, depsmodule.Dependency{
				Name:       dep.Name,
				InstallCmd: dep.InstallCmd,
			})
		}
	}
	depsmodule.ShowMissingDependencies(s.window, payload)
}
