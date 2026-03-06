package appcfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
)

func LoadModuleJSON(name string, out interface{}) (map[string]json.RawMessage, error) {
	path := configpath.ModuleConfigPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return raw, nil
}

func SaveModuleJSON(name string, in interface{}) error {
	path := configpath.ModuleConfigPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
