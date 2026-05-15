package theme

import (
	"encoding/json"
	"fmt"
	"os"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// ScriptableTheme defines the structure of a moddable DVD menu theme.
type ScriptableTheme struct {
	Name       string      `json:"name"`
	Background Background  `json:"background"`
	Elements   []Element   `json:"elements"`
	Resolution Resolution  `json:"resolution"`
}

type Background struct {
	Type   string `json:"type"` // "video" or "image"
	Source string `json:"source"`
	Loop   bool   `json:"loop"`
}

type Element struct {
	Type    string `json:"type"` // "text", "button", "image"
	ID      string `json:"id"`
	Content string `json:"content"` // Text content or image path
	Action  string `json:"action"`  // "play_all", "chapters", "extras", "play_title:N"
	Style   Style  `json:"style"`
	Hover   *Style `json:"hover,omitempty"`
}

type Style struct {
	Top             string `json:"top"`
	Left            string `json:"left"`
	Width           string `json:"width"`
	Height          string `json:"height"`
	FontSize        string `json:"font-size"`
	Color           string `json:"color"`
	BackgroundColor string `json:"background-color"`
	Border          string `json:"border"`
}

type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LoadTheme loads a theme from a JSON file.
func LoadTheme(path string) (*ScriptableTheme, error) {
	logging.Info(logging.CatDVD, "Loading scriptable theme from %s", path)
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theme file: %w", err)
	}
	
	var theme ScriptableTheme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("unmarshal theme json: %w", err)
	}
	
	logging.Debug(logging.CatDVD, "Theme '%s' loaded with %d elements", theme.Name, len(theme.Elements))
	return &theme, nil
}
