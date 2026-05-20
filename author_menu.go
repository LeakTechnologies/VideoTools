package main

import (
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/spu"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/theme"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/vob"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

//go:embed assets/fonts/IBMPlexMono-Regular.ttf
var ibmPlexMonoTTF []byte

type dvdMenuButton struct {
	ID      string
	Label   string
	Command string
	X0      int
	Y0      int
	X1      int
	Y1      int
	IsNav   bool   // True if this is a navigation button (Next/Prev)
	NavType string // "next" or "prev"
}

type MenuTheme struct {
	Name            string
	BackgroundColor string
	HeaderColor     string
	TextColor       string
	AccentColor     string
	FontName        string
	FontPath        string
	IsCustom        bool
}

type menuLogoOptions struct {
	TitleLogo  menuLogo
	StudioLogo menuLogo
}

type menuLogo struct {
	Enabled  bool
	Path     string
	Position string
	Scale    float64
	Margin   int
}

// MenuTemplate defines the interface for a DVD menu generator.
type MenuTemplate interface {
	Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error)
}

var menuTemplates = map[string]MenuTemplate{
	"Minimal":    &MinimalMenu{},
	"Simple":     &SimpleMenu{},
	"Classic":    &ClassicMenu{},
	"Grid":       &GridMenu{},
	"Filmstrip":  &FilmstripMenu{},
	"Dark":       &DarkMenu{},
	"Poster":     &PosterMenu{},
	"Scriptable": &ScriptableMenu{},
}

type ScriptableMenu struct{}

func (t *ScriptableMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, mTheme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)

	// 1. Load Theme JSON
	themePath := filepath.Join("assets", "dvd_themes", "default", "theme.json")
	sTheme, err := theme.LoadTheme(themePath)
	if err != nil {
		logging.Error(logging.CatDVD, "Failed to load scriptable theme: %v. Falling back to Minimal.", err)
		return (&MinimalMenu{}).Generate(ctx, workDir, title, region, aspect, chapters, backgroundImage, motionBackground, mTheme, logo, logFn)
	}
	sTheme.Resolution.Width = width
	sTheme.Resolution.Height = height

	// 2. Render Assets Natively
	renderer := theme.NewRenderer(sTheme)
	fontData, _ := os.ReadFile(mTheme.FontPath)
	renderer.SetFont(fontData)

	menuAssets, err := renderer.RenderMenu()
	if err != nil {
		return "", nil, fmt.Errorf("native render failed: %w", err)
	}

	// 3. Save rendered PNGs
	bgPath := filepath.Join(workDir, "menu_bg.png")
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")

	savePNG := func(path string, img image.Image) error {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return png.Encode(f, img)
	}

	if err := savePNG(bgPath, menuAssets.Background); err != nil {
		return "", nil, err
	}
	if err := savePNG(overlayPath, menuAssets.Highlight); err != nil {
		return "", nil, err
	}
	// Highlight and Select are currently the same for scriptable
	if err := savePNG(highlightPath, menuAssets.Highlight); err != nil {
		return "", nil, err
	}
	if err := savePNG(selectPath, menuAssets.Highlight); err != nil {
		return "", nil, err
	}

	// 4. Continue with Muxing
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	// Convert ButtonRect to dvdMenuButton
	var buttons []dvdMenuButton
	for _, br := range menuAssets.Buttons {
		// Assign commands based on Action
		cmd := "jump title 1;" // Default
		if br.ID == "play-btn" {
			cmd = "jump title 1;"
		}
		buttons = append(buttons, dvdMenuButton{
			ID: br.ID, Label: br.ID, Command: cmd, X0: br.X0, Y0: br.Y0, X1: br.X1, Y1: br.Y1,
		})
	}

	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}

	return menuSpu, buttons, nil
}

var menuThemes = map[string]*MenuTheme{
	"VideoTools": {
		Name:            "VideoTools",
		BackgroundColor: "0x0f172a",
		HeaderColor:     "0x1f2937",
		TextColor:       "0xE1EEFF",
		AccentColor:     "0x7c3aed",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Minimal": {
		Name:            "Minimal",
		BackgroundColor: "0x000000",
		HeaderColor:     "0x1a1a1a",
		TextColor:       "0xFFFFFF",
		AccentColor:     "0xAAAAAA",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Western": {
		Name:            "Western",
		BackgroundColor: "0x1a1408",
		HeaderColor:     "0x2d2310",
		TextColor:       "0xF5DEB3",
		AccentColor:     "0x8B4513",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Film Noir": {
		Name:            "Film Noir",
		BackgroundColor: "0x1a1a1a",
		HeaderColor:     "0x2d2d2d",
		TextColor:       "0xE0E0E0",
		AccentColor:     "0x808080",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Classic Hollywood": {
		Name:            "Classic Hollywood",
		BackgroundColor: "0x000000",
		HeaderColor:     "0x1a1a1a",
		TextColor:       "0xF5F5DC",
		AccentColor:     "0xD4AF37",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Warm Cinema": {
		Name:            "Warm Cinema",
		BackgroundColor: "0x1a0f0a",
		HeaderColor:     "0x2d1a10",
		TextColor:       "0xFFF5E6",
		AccentColor:     "0xE67E22",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Ocean": {
		Name:            "Ocean",
		BackgroundColor: "0x0a1a2a",
		HeaderColor:     "0x142d40",
		TextColor:       "0xE0F0FF",
		AccentColor:     "0x00CED1",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
	"Nature": {
		Name:            "Nature",
		BackgroundColor: "0x0a1a0a",
		HeaderColor:     "0x142d14",
		TextColor:       "0xE6FFE6",
		AccentColor:     "0xDAA520",
		FontName:        "IBM Plex Mono",
		FontPath:        findMenuFontPath(),
	},
}

// MinimalMenu is a clean, minimal menu template with black background and white text buttons.
// Inspired by classic DVD menus like "The Anniversary Party".
type MinimalMenu struct{}

// Generate creates a minimal DVD menu with black background and clean white text.
func (t *MinimalMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with MinimalMenu template...")
	}

	// For Minimal menu, we use pure black background (theme-aware via resolveMenuTheme)
	if backgroundImage == "" {
		if err := buildMinimalMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// SimpleMenu is a basic menu template.
type SimpleMenu struct{}

// Generate creates a simple DVD menu.
func (t *SimpleMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height) // hasExtras=false for template compatibility
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with SimpleMenu template...")
	}

	if backgroundImage == "" {
		if err := buildMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// ClassicMenu is a traditional DVD menu template with centered title and buttons.
type ClassicMenu struct{}

// Generate creates a classic DVD menu with centered title and buttons.
func (t *ClassicMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with ClassicMenu template...")
	}

	if backgroundImage == "" {
		if err := buildClassicMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// DarkMenu is a dark-themed menu template.
type DarkMenu struct{}

// Generate creates a dark-themed DVD menu.
func (t *DarkMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height) // hasExtras=false for template compatibility
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with DarkMenu template...")
	}

	if backgroundImage == "" {
		if err := buildDarkMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// GridMenu is a template with buttons arranged in a grid (2x2 or 3x2).
type GridMenu struct{}

// Generate creates a grid-based DVD menu with buttons in a matrix layout.
func (t *GridMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with GridMenu template...")
	}

	if backgroundImage == "" {
		if err := buildGridMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// FilmstripMenu is a template with wide horizontal buttons like a filmstrip.
type FilmstripMenu struct{}

// Generate creates a filmstrip-style DVD menu with wide horizontal buttons.
func (t *FilmstripMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with FilmstripMenu template...")
	}

	if backgroundImage == "" {
		if err := buildFilmstripMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// PosterMenu is a template that uses a poster image as a background.
type PosterMenu struct{}

// Generate creates a poster-themed DVD menu.
func (t *PosterMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, false, width, height) // hasExtras=false for template compatibility
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage == "" {
		return "", nil, fmt.Errorf("poster menu requires a background image")
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with PosterMenu template...")
	}

	if err := buildPosterMenuBackground(ctx, bgPath, title, buttons, width, height, backgroundImage, resolveMenuTheme(theme), logo, logFn); err != nil {
		return "", nil, err
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

type dvdMenuSet struct {
	MainMpg         string
	MainButtons     []dvdMenuButton
	ChaptersMpgs    []string          // Multiple menu pages
	ChaptersButtons [][]dvdMenuButton // Multiple pages of buttons
	ChaptersPages   int               // Number of chapter menu pages
	ExtrasMpg       string
	ExtrasButtons   []dvdMenuButton
}

func buildDVDMenuAssets(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, extras []extraItem, logFn func(string), template MenuTemplate, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, chapterVideoPath string, chapterThumbMode ChapterThumbMode, chapterThumbOffset float64) (dvdMenuSet, error) {
	if template == nil {
		template = &SimpleMenu{}
	}

	// Determine main menu buttons based on chapters and extras
	width, height := dvdMenuDimensions(region)
	hasExtras := len(extras) > 0
	mainButtons := buildDVDMenuButtons(chapters, hasExtras, width, height)

	// Generate main menu MPEG set
	mainMpg, err := buildMainMenuMPEGSet(ctx, workDir, title, region, aspect, mainButtons, backgroundImage, motionBackground, theme, logo, logFn)
	if err != nil {
		return dvdMenuSet{}, err
	}

	result := dvdMenuSet{
		MainMpg:      mainMpg,
		MainButtons:  mainButtons,
		ChaptersMpgs: []string{},
	}

	// Generate chapters menu if there are multiple chapters
	if len(chapters) > 1 {
		chaptersMpgs, chaptersButtons, err := buildChaptersMenuMPEGSet(ctx, workDir, title, region, aspect, chapters, backgroundImage, motionBackground, theme, logFn, chapterVideoPath, chapterThumbMode, chapterThumbOffset)
		if err != nil {
			return dvdMenuSet{}, err
		}
		result.ChaptersMpgs = chaptersMpgs
		result.ChaptersButtons = chaptersButtons
		result.ChaptersPages = len(chaptersMpgs)
	}

	// Generate extras menu if there are extras
	if len(extras) > 0 {
		extrasMenuMpg, extrasButtons, err := buildExtrasMenuMPEGSet(ctx, workDir, title, region, aspect, extras, backgroundImage, motionBackground, theme, logFn)
		if err != nil {
			return dvdMenuSet{}, err
		}
		result.ExtrasMpg = extrasMenuMpg
		result.ExtrasButtons = extrasButtons
	}

	return result, nil
}

func buildMainMenuMPEGSet(ctx context.Context, workDir, title, region, aspect string, buttons []dvdMenuButton, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, error) {
	width, height := dvdMenuDimensions(region)

	bgPath := filepath.Join(workDir, "menu_bg.png")
	if backgroundImage != "" {
		bgPath = backgroundImage
	}
	overlayPath := filepath.Join(workDir, "menu_overlay.png")
	highlightPath := filepath.Join(workDir, "menu_highlight.png")
	selectPath := filepath.Join(workDir, "menu_select.png")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")

	if logFn != nil {
		logFn("Building DVD menu assets with SimpleMenu template...")
	}

	if backgroundImage == "" {
		if err := buildMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logo, logFn); err != nil {
			return "", err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, nil
}

func buildExtrasMenuMPEGSet(ctx context.Context, workDir, title, region, aspect string, extras []extraItem, backgroundImage, motionBackground string, theme *MenuTheme, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildExtrasMenuButtons(extras, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "extras_menu_bg.png")
	overlayPath := filepath.Join(workDir, "extras_menu_overlay.png")
	highlightPath := filepath.Join(workDir, "extras_menu_highlight.png")
	selectPath := filepath.Join(workDir, "extras_menu_select.png")
	menuSpu := filepath.Join(workDir, "extras_menu_spu.mpg")

	if logFn != nil {
		logFn("Building extras menu assets...")
	}

	if err := buildExtrasMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("Extras menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

func buildChaptersMenuMPEGSet(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logFn func(string), chapterVideoPath string, chapterThumbMode ChapterThumbMode, chapterThumbOffset float64) ([]string, [][]dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)

	// Calculate optimal paging based on menu dimensions
	// Header takes ~100px, navigation buttons at bottom if needed
	headerHeight := 100
	buttonHeight := 28
	buttonSpacing := 4
	chaptersPerPage := CalculateOptimalPaging(len(chapters), height, headerHeight, buttonHeight, buttonSpacing)

	if len(chaptersPerPage) == 0 || chaptersPerPage[0] == 0 {
		return nil, nil, nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Building chapters menu: %d chapters across %d page(s)...", len(chapters), len(chaptersPerPage)))
	}

	// Generate thumbnails for ALL chapters (we'll slice per page)
	var allThumbPaths []string
	if chapterVideoPath != "" && chapterThumbOffset > 0 {
		allThumbPaths = generateChapterThumbnails(ctx, workDir, chapterVideoPath, chapters, chapterThumbMode, chapterThumbOffset, logFn)
	}

	var mpgPaths []string
	var allButtons [][]dvdMenuButton

	// Build each page
	for pageIndex, chapterCount := range chaptersPerPage {
		chapterStart, _ := GetChapterRangeForPage(chapters, chaptersPerPage, pageIndex)

		hasPrevPage := pageIndex > 0
		hasNextPage := pageIndex < len(chaptersPerPage)-1

		buttons := buildChapterMenuButtonsForPage(
			chapters,
			chapterStart,
			chapterCount,
			width,
			height,
			hasPrevPage,
			hasNextPage,
			pageIndex,
		)
		allButtons = append(allButtons, buttons)

		// Get thumbnails for this page's chapters
		pageThumbPaths := allThumbPaths[chapterStart : chapterStart+chapterCount]

		// Pad to match button count (in case thumbnail count is less)
		for len(pageThumbPaths) < chapterCount {
			pageThumbPaths = append(pageThumbPaths, "")
		}

		// Build paths for this page
		bgPath := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_bg.png", pageIndex+1))
		overlayPath := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_overlay.png", pageIndex+1))
		highlightPath := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_highlight.png", pageIndex+1))
		selectPath := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_select.png", pageIndex+1))
		menuSpu := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_spu.mpg", pageIndex+1))

		// Build background with thumbnails
		if err := buildChaptersMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), pageThumbPaths, logFn); err != nil {
			return nil, nil, err
		}

		// Build overlays
		if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
			return nil, nil, err
		}

		// Encode SPU
		if err := buildMenuSPU(ctx, overlayPath, menuSpu, bgPath, region, buttons, logFn); err != nil {
			return nil, nil, fmt.Errorf("native SPU encoder: %w", err)
		}

		mpgPaths = append(mpgPaths, menuSpu)

		if logFn != nil {
			logStr := fmt.Sprintf("Chapters page %d created: %d chapters", pageIndex+1, chapterCount)
			if hasPrevPage || hasNextPage {
				logStr += " (with navigation)"
			}
			logFn(logStr)
		}
	}

	return mpgPaths, allButtons, nil
}

// generateChapterThumbnails creates thumbnail images for each chapter.
// Returns slice of thumbnail paths in order of chapters.
// ChapterThumbMode defines how to position chapter thumbnails
type ChapterThumbMode string

const (
	ChapterThumbModeStart    ChapterThumbMode = "start"    // At chapter start
	ChapterThumbModeMidpoint ChapterThumbMode = "midpoint" // At chapter midpoint
	ChapterThumbModeCustom   ChapterThumbMode = "custom"   // At custom offset from chapter start
)

// generateChapterThumbnails creates thumbnail images for each chapter.
// mode determines where in the chapter to capture: start, midpoint, or custom offset.
// offsetSeconds is used for custom mode (and as additional offset for start mode).
func generateChapterThumbnails(ctx context.Context, workDir, videoPath string, chapters []authorChapter, mode ChapterThumbMode, offsetSeconds float64, logFn func(string)) []string {
	var thumbPaths []string
	thumbWidth := 160
	thumbHeight := 90

	for i, chapter := range chapters {
		var timestamp float64

		switch mode {
		case ChapterThumbModeMidpoint:
			// Calculate midpoint: chapter start + half of assumed 5-minute duration
			// In practice we'd need chapter duration, but we'll use a reasonable default
			chapterDuration := 300.0 // 5 minutes default
			if i < len(chapters)-1 {
				chapterDuration = chapters[i+1].Timestamp - chapter.Timestamp
			}
			timestamp = chapter.Timestamp + (chapterDuration / 2)
		case ChapterThumbModeCustom:
			// Custom offset from chapter start
			timestamp = chapter.Timestamp + offsetSeconds
		case ChapterThumbModeStart:
		default:
			// Default: start with optional offset
			timestamp = chapter.Timestamp + offsetSeconds
		}

		thumbPath := filepath.Join(workDir, fmt.Sprintf("chapter_%02d_thumb.png", i+1))

		if logFn != nil {
			modeDesc := string(mode)
			if mode == ChapterThumbModeStart && offsetSeconds > 0 {
				modeDesc = fmt.Sprintf("start+%.1fs", offsetSeconds)
			}
			logFn(fmt.Sprintf("Generating thumbnail for chapter %d at %.1fs (%s)...", i+1, timestamp, modeDesc))
		}

		args := []string{
			"-ss", fmt.Sprintf("%.2f", timestamp),
			"-i", videoPath,
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:%d", thumbWidth, thumbHeight),
			"-y",
			thumbPath,
		}

		cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
		if err := cmd.Run(); err == nil {
			// Verify the file was created
			if _, err := os.Stat(thumbPath); err == nil {
				thumbPaths = append(thumbPaths, thumbPath)
				continue
			}
		}
		// If failed, add empty path
		thumbPaths = append(thumbPaths, "")
	}

	return thumbPaths
}

// CalculateOptimalPaging calculates how to distribute chapters across menu pages.
// Returns a slice where each element is the number of chapters for that page.
// Earlier pages get extra chapters if distribution is uneven for better visual balance.
func CalculateOptimalPaging(totalChapters, maxHeight, headerHeight, buttonHeight, buttonSpacing int) []int {
	// Calculate max that fits in available space
	availableHeight := maxHeight - headerHeight
	maxPerPage := availableHeight / (buttonHeight + buttonSpacing)

	if maxPerPage <= 0 {
		maxPerPage = 1
	}

	// If all chapters fit on one page, just return that
	if totalChapters <= maxPerPage {
		return []int{totalChapters}
	}

	// Calculate number of pages needed
	numPages := (totalChapters + maxPerPage - 1) / maxPerPage

	// Distribute chapters as evenly as possible, but put extra chapters on EARLIER pages
	// (more balanced visually - later pages are shorter, not empty-looking)
	baseChapters := totalChapters / numPages
	remainder := totalChapters % numPages

	var chaptersPerPage []int
	for i := 0; i < numPages; i++ {
		count := baseChapters
		if i < remainder {
			count++ // Earlier pages get extra chapters
		}
		chaptersPerPage = append(chaptersPerPage, count)
	}

	return chaptersPerPage
}

// GetChapterRangeForPage returns the start index and count of chapters for a given page.
// pageIndex is 0-based. Returns (startIndex, chapterCount).
func GetChapterRangeForPage(chapters []authorChapter, chaptersPerPage []int, pageIndex int) (int, int) {
	if pageIndex < 0 || pageIndex >= len(chaptersPerPage) {
		return 0, 0
	}

	startIndex := 0
	for i := 0; i < pageIndex; i++ {
		startIndex += chaptersPerPage[i]
	}

	return startIndex, chaptersPerPage[pageIndex]
}

func dvdMenuDimensions(region string) (int, int) {
	if strings.ToLower(region) == "pal" {
		return 720, 576
	}
	return 720, 480
}

func buildDVDMenuButtons(chapters []authorChapter, hasExtras bool, width, height int) []dvdMenuButton {
	t := i18n.T()
	buttons := []dvdMenuButton{
		{
			Label:   t.AuthorPlay,
			Command: "jump title 1;",
		},
	}

	// Add Chapters button if there are multiple chapters
	if len(chapters) > 1 {
		buttons = append(buttons, dvdMenuButton{
			Label:   t.AuthorChapters,
			Command: "jump menu 2;", // Jump to chapters menu (second PGC)
		})
	}

	// Add Extras button if extras are present
	if hasExtras {
		extrasMenuIndex := 2
		if len(chapters) > 1 {
			extrasMenuIndex = 3 // Chapters menu is PGC 2, so extras is PGC 3
		}
		buttons = append(buttons, dvdMenuButton{
			Label:   t.AuthorExtrasMenu,
			Command: fmt.Sprintf("jump menu %d;", extrasMenuIndex),
		})
	}

	// Position buttons
	startY := 180
	rowHeight := 34
	boxHeight := 28
	x0 := 86
	x1 := width - 86
	for i := range buttons {
		y0 := startY + i*rowHeight
		buttons[i].X0 = x0
		buttons[i].X1 = x1
		buttons[i].Y0 = y0
		buttons[i].Y1 = y0 + boxHeight
	}
	return buttons
}

// countValidThumbs returns the number of non-empty, existing thumbnail paths.
func countValidThumbs(thumbPaths []string) int {
	n := 0
	for _, p := range thumbPaths {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				n++
			}
		}
	}
	return n
}

// buildChapterMenuButtons creates buttons for ALL chapters (legacy single-page version)
func buildChapterMenuButtons(chapters []authorChapter, width, height int) []dvdMenuButton {
	// Use dynamic paging with default layout parameters
	chaptersPerPage := CalculateOptimalPaging(len(chapters), height, 100, 32, 6)
	if len(chaptersPerPage) == 1 {
		// Single page - use original logic
		return buildChapterMenuButtonsForPage(chapters, 0, len(chapters), width, height, false, false, 0)
	}
	// Multiple pages - build first page with navigation
	return buildChapterMenuButtonsForPage(chapters, 0, chaptersPerPage[0], width, height, false, len(chaptersPerPage) > 1, 0)
}

// buildChapterMenuButtonsForPage creates buttons for a specific page of chapters.
// chapterStart is the 0-based index of the first chapter on this page.
// chapterCount is the number of chapters on this page.
// totalPages is the total number of pages (0-based index).
// currentPage is the 0-based index of this page.
func buildChapterMenuButtonsForPage(chapters []authorChapter, chapterStart, chapterCount, width, height int, hasPrevPage, hasNextPage bool, currentPage int) []dvdMenuButton {
	t := i18n.T()
	buttons := []dvdMenuButton{}

	// Add chapter buttons for this page
	for i := 0; i < chapterCount; i++ {
		chapterIndex := chapterStart + i
		if chapterIndex >= len(chapters) {
			break
		}
		ch := chapters[chapterIndex]
		buttons = append(buttons, dvdMenuButton{
			Label:   ch.Title,
			Command: fmt.Sprintf("jump title 1 chapter %d;", chapterIndex+1),
		})
	}

	// Calculate button positions - leaving room for navigation if needed
	navButtons := 0
	if hasPrevPage {
		navButtons++
	}
	if hasNextPage {
		navButtons++
	}

	// Position buttons
	startY := 120
	rowHeight := 32
	boxHeight := 26
	x0 := 60
	x1 := width - 60

	// If we have navigation, shift chapter buttons up slightly
	if navButtons > 0 {
		startY = 100
		rowHeight = 28
	}

	// Position chapter buttons
	for i := range buttons {
		y0 := startY + i*rowHeight
		buttons[i].X0 = x0
		buttons[i].X1 = x1
		buttons[i].Y0 = y0
		buttons[i].Y1 = y0 + boxHeight
	}

	// Add navigation buttons at the bottom if needed
	navStartY := startY + chapterCount*rowHeight + 10

	// PGC numbering: 1=Main, 2=ChaptersPage1, 3=ChaptersPage2, etc.
	// currentPage is 0-based, so PGC = currentPage + 2
	thisPGC := currentPage + 2

	if hasPrevPage {
		// Previous page: jump to PGC (currentPage + 1), or back to main if first chapter page
		prevPGC := thisPGC - 1
		if prevPGC < 2 {
			prevPGC = 1 // Go back to main menu
		}
		prevButton := dvdMenuButton{
			Label:   t.AuthorPrevPage,
			Command: fmt.Sprintf("jump menu %d;", prevPGC),
			IsNav:   true,
			NavType: "prev",
			X0:      x0,
			X1:      x0 + 120,
			Y0:      navStartY,
			Y1:      navStartY + boxHeight,
		}
		buttons = append(buttons, prevButton)
	}

	if hasNextPage {
		// Next page: jump to PGC (currentPage + 3)
		nextPGC := thisPGC + 1
		nextButton := dvdMenuButton{
			Label:   t.AuthorNextPage,
			Command: fmt.Sprintf("jump menu %d;", nextPGC),
			IsNav:   true,
			NavType: "next",
			X0:      x1 - 120,
			X1:      x1,
			Y0:      navStartY,
			Y1:      navStartY + boxHeight,
		}
		buttons = append(buttons, nextButton)
	}

	// Add Back button
	backButton := dvdMenuButton{
		Label:   t.AuthorBack,
		Command: "jump menu 1;", // Jump back to main menu
	}
	backY := navStartY
	if navButtons > 0 {
		backY = navStartY + rowHeight
	}
	backButton.X0 = x0
	backButton.X1 = x1
	backButton.Y0 = backY
	backButton.Y1 = backY + boxHeight
	buttons = append(buttons, backButton)

	return buttons
}

type extraItem struct {
	Title    string
	TitleNum int // DVD title number for this extra
}

func buildExtrasMenuButtons(extras []extraItem, width, height int) []dvdMenuButton {
	t := i18n.T()
	buttons := []dvdMenuButton{}

	// Add a button for each extra
	for _, extra := range extras {
		buttons = append(buttons, dvdMenuButton{
			Label:   extra.Title,
			Command: fmt.Sprintf("jump title %d;", extra.TitleNum),
		})
	}

	// Add Back button at the end
	buttons = append(buttons, dvdMenuButton{
		Label:   t.AuthorBack,
		Command: "jump menu 1;", // Jump back to main menu (first PGC)
	})

	// Position buttons - allow more buttons to fit
	startY := 120
	rowHeight := 32
	boxHeight := 26
	x0 := 60
	x1 := width - 60

	for i := range buttons {
		y0 := startY + i*rowHeight
		buttons[i].X0 = x0
		buttons[i].X1 = x1
		buttons[i].Y0 = y0
		buttons[i].Y1 = y0 + boxHeight
	}

	return buttons
}

func buildMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = t.AuthorDVDMenu
	}

	bgColor := theme.BackgroundColor
	headerColor := theme.HeaderColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorVideoToolsDVD)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=122:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorSelectTitleChapter)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text=%s", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle title logo and studio logo overlays
	inputIndex := 1

	// Add title logo if enabled
	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	// Add studio logo if enabled
	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

// buildMinimalMenuBackground creates a minimal menu with clean layout:
// Title at top, buttons on left side, simple and elegant.
func buildMinimalMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := strings.ToUpper(utils.ShortenMiddle(strings.TrimSpace(title), 40))
	if safeTitle == "" {
		safeTitle = strings.ToUpper(t.AuthorDVDMenu)
	}

	// Minimal theme uses pure black background with theme's text colors
	bgColor := "0x000000"
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	// Title centered at top, buttons on left side
	filterParts := []string{
		// Title centered at top
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=36:x=(w-text_w)/2:y=40:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		// Separator line below title
		fmt.Sprintf("drawbox=x=100:y=95:w=%d:h=2:color=%s:t=fill", width-200, accentColor),
	}

	// Buttons on left side with good spacing
	for i, btn := range buttons {
		label := strings.ToUpper(escapeDrawtextText(btn.Label))
		y := 150 + i*40
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=22:x=120:y=%d:text=%s", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle title logo and studio logo overlays
	inputIndex := 1

	// Add title logo if enabled
	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	// Add studio logo if enabled
	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

// buildClassicMenuBackground creates a classic DVD menu with:
// - Centered title at top
// - Centered buttons below
// - Decorative border lines
func buildClassicMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := strings.ToUpper(utils.ShortenMiddle(strings.TrimSpace(title), 40))
	if safeTitle == "" {
		safeTitle = strings.ToUpper(t.AuthorDVDMenu)
	}

	bgColor := theme.BackgroundColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	// Classic layout: centered title, centered buttons
	filterParts := []string{
		// Top decorative line
		fmt.Sprintf("drawbox=x=100:y=20:w=%d:h=2:color=%s:t=fill", width-200, accentColor),
		// Centered title
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=32:x=(w-text_w)/2:y=35:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		// Middle decorative line
		fmt.Sprintf("drawbox=x=100:y=90:w=%d:h=2:color=%s:t=fill", width-200, accentColor),
	}

	// Centered buttons
	buttonStartY := 140
	buttonSpacing := 36
	for i, btn := range buttons {
		label := strings.ToUpper(escapeDrawtextText(btn.Label))
		y := buttonStartY + i*buttonSpacing
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=(w-text_w)/2:y=%d:text=%s", fontArg, textColor, y, label))
	}

	// Bottom decorative line
	filterParts = append(filterParts, fmt.Sprintf("drawbox=x=100:y=%d:w=%d:h=2:color=%s:t=fill", buttonStartY+len(buttons)*buttonSpacing+10, width-200, accentColor))

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle logo overlays
	inputIndex := 1

	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

// buildGridMenuBackground creates a grid menu with buttons arranged in a 2x2 or 3x2 matrix.
func buildGridMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := strings.ToUpper(utils.ShortenMiddle(strings.TrimSpace(title), 40))
	if safeTitle == "" {
		safeTitle = strings.ToUpper(t.AuthorDVDMenu)
	}

	bgColor := theme.BackgroundColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	// Calculate grid layout
	cols := 2
	if len(buttons) > 4 {
		cols = 3
	}
	buttonWidth := (width - 200) / cols
	buttonHeight := 50
	startX := 100
	startY := 120

	filterParts := []string{
		// Title centered at top
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=(w-text_w)/2:y=30:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		// Decorative line under title
		fmt.Sprintf("drawbox=x=80:y=70:w=%d:h=2:color=%s:t=fill", width-160, accentColor),
	}

	// Grid buttons
	for i, btn := range buttons {
		col := i % cols
		row := i / cols
		x := startX + col*buttonWidth + 20
		y := startY + row*(buttonHeight+20)
		label := strings.ToUpper(escapeDrawtextText(btn.Label))
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=%d:y=%d:text=%s", fontArg, textColor, x, y+15, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle logo overlays
	inputIndex := 1

	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

// buildFilmstripMenuBackground creates a filmstrip-style menu with wide horizontal buttons.
func buildFilmstripMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := strings.ToUpper(utils.ShortenMiddle(strings.TrimSpace(title), 40))
	if safeTitle == "" {
		safeTitle = strings.ToUpper(t.AuthorDVDMenu)
	}

	bgColor := theme.BackgroundColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	// Filmstrip layout: title at top, wide horizontal buttons below
	filterParts := []string{
		// Title centered
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=(w-text_w)/2:y=25:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		// Decorative line
		fmt.Sprintf("drawbox=x=80:y=65:w=%d:h=2:color=%s:t=fill", width-160, accentColor),
	}

	// Wide horizontal buttons stacked vertically
	buttonHeight := 40
	startX := 100
	startY := 90
	spacing := 15

	for i, btn := range buttons {
		label := strings.ToUpper(escapeDrawtextText(btn.Label))
		y := startY + i*(buttonHeight+spacing)
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=%d:y=%d:text=%s", fontArg, textColor, startX+10, y+12, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle logo overlays
	inputIndex := 1

	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildDarkMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = t.AuthorDVDMenu
	}

	bgColor := "0x000000"
	headerColor := "0x111111"
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorVideoToolsDVD)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=122:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorSelectTitleChapter)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text=%s", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]%s[bg]", filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]%s", filterChain)
		baseLayer = "[0:v]"
	}

	// Handle title logo and studio logo overlays
	inputIndex := 1

	// Add title logo if enabled
	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	// Add studio logo if enabled
	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildPosterMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()
	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = t.AuthorDVDMenu
	}

	textColor := theme.TextColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorVideoToolsDVD)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text=%s", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-i", backgroundImage}

	// Only add output label if logos are enabled; otherwise FFmpeg will complain about unconnected output
	hasLogos := logo.TitleLogo.Enabled || logo.StudioLogo.Enabled
	var filterExpr string
	var baseLayer string
	if hasLogos {
		filterExpr = fmt.Sprintf("[0:v]scale=%d:%d,%s[bg]", width, height, filterChain)
		baseLayer = "[bg]"
	} else {
		filterExpr = fmt.Sprintf("[0:v]scale=%d:%d,%s", width, height, filterChain)
		baseLayer = "[0:v]"
	}

	// Handle title logo and studio logo overlays
	inputIndex := 1

	// Add title logo if enabled
	if logo.TitleLogo.Enabled {
		titleLogoPath := resolveMenuLogoPath(logo.TitleLogo)
		if titleLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.TitleLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.TitleLogo, width, height)
			args = append(args, "-i", titleLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[titlelogo];%s[titlelogo]overlay=%s[tmp%d]", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr, inputIndex)
			baseLayer = fmt.Sprintf("[tmp%d]", inputIndex)
			inputIndex++
		}
	}

	// Add studio logo if enabled
	if logo.StudioLogo.Enabled {
		studioLogoPath := resolveMenuLogoPath(logo.StudioLogo)
		if studioLogoPath != "" {
			posExpr := resolveMenuLogoPosition(logo.StudioLogo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo.StudioLogo, width, height)
			args = append(args, "-i", studioLogoPath)
			filterExpr = fmt.Sprintf("%s;[%d:v]%s[studiologo];%s[studiologo]overlay=%s", filterExpr, inputIndex, scaleExpr, baseLayer, posExpr)
		}
	}

	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildExtrasMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = t.AuthorDVDMenu
	}

	bgColor := theme.BackgroundColor
	headerColor := theme.HeaderColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorExtrasMenu)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=52:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=80:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=14:x=36:y=94:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorSelectExtra)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		// Truncate long names for display
		if len(label) > 50 {
			label = label[:47] + "..."
		}
		y := 120 + i*32
		fontSize := 18
		if btn.Label == t.AuthorBack {
			fontSize = 20 // Make Back button slightly larger
		}
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=%d:x=80:y=%d:text=%s", fontArg, textColor, fontSize, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}
	args = append(args, "-filter_complex", fmt.Sprintf("[0:v]%s", filterChain), "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildChaptersMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, thumbPaths []string, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	t := i18n.T()

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = t.AuthorDVDMenu
	}

	bgColor := theme.BackgroundColor
	headerColor := theme.HeaderColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	// Check if we have navigation buttons to adjust layout
	hasNavButtons := false
	navButtonCount := 0
	for _, btn := range buttons {
		if btn.IsNav {
			hasNavButtons = true
			navButtonCount++
		}
	}

	// Determine layout based on navigation buttons
	startY := 120
	rowHeight := 32
	thumbnailX := 450 // Position thumbnails on the right side

	if hasNavButtons {
		startY = 100
		rowHeight = 28
		thumbnailX = 420
	}

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorChapterSelection)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=52:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=80:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=14:x=36:y=94:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorSelectChapterMenu)),
	}

	// Add chapter button text and track thumbnail positions
	thumbnailYPositions := make(map[int]int) // button index -> y position
	chapterButtonIndex := 0

	for _, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		// Skip navigation buttons in text rendering
		if btn.IsNav {
			continue
		}
		// Truncate long chapter names for display
		if len(label) > 30 {
			label = label[:27] + "..."
		}
		y := startY + chapterButtonIndex*rowHeight
		fontSize := 18

		// Skip Back button in y calculation for thumbnails
		if btn.Label == t.AuthorBack {
			fontSize = 20
		} else if chapterButtonIndex < len(thumbPaths) && thumbPaths[chapterButtonIndex] != "" {
			// This button has a thumbnail
			thumbnailYPositions[chapterButtonIndex] = y
		}

		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=%d:x=80:y=%d:text=%s", fontArg, textColor, fontSize, y, label))
		chapterButtonIndex++
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}

	// Add thumbnail overlays if available.
	// Build a proper chain: each overlay consumes the previous overlay's output.
	// The final overlay is left unlabelled so FFmpeg auto-maps it to the output.
	if len(thumbPaths) > 0 {
		filterExpr := fmt.Sprintf("[0:v]%s[bg0]", filterChain)

		inputIndex := 1
		validThumbs := 0
		currentBase := "bg0"
		for i, thumbPath := range thumbPaths {
			if thumbPath == "" {
				continue
			}
			if _, err := os.Stat(thumbPath); err != nil {
				continue
			}

			// Get y position for this thumbnail
			y, ok := thumbnailYPositions[i]
			if !ok {
				y = startY + i*rowHeight
			}
			// Adjust y for thumbnail (center in button box, thumb is 80x45)
			y = y + 2

			args = append(args, "-i", thumbPath)

			scaleFilter := fmt.Sprintf("[%d:v]scale=80:-2[thumb%d]", inputIndex, inputIndex)
			isLast := inputIndex == countValidThumbs(thumbPaths)
			var overlayFilter string
			if isLast {
				// Final overlay: no output label — FFmpeg auto-maps to output
				overlayFilter = fmt.Sprintf("[%s][thumb%d]overlay=%d:%d", currentBase, inputIndex, thumbnailX, y)
			} else {
				nextBase := fmt.Sprintf("bg%d", inputIndex)
				overlayFilter = fmt.Sprintf("[%s][thumb%d]overlay=%d:%d[%s]", currentBase, inputIndex, thumbnailX, y, nextBase)
				currentBase = nextBase
			}

			filterExpr = filterExpr + ";" + scaleFilter + ";" + overlayFilter
			validThumbs++
			inputIndex++
		}

		if validThumbs > 0 {
			args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
		} else {
			args = append(args, "-filter_complex", fmt.Sprintf("[0:v]%s", filterChain), "-frames:v", "1", outputPath)
		}
	} else {
		// No thumbnails
		args = append(args, "-filter_complex", fmt.Sprintf("[0:v]%s", filterChain), "-frames:v", "1", outputPath)
	}

	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildMenuOverlays(ctx context.Context, overlayPath, highlightPath, selectPath string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	accent := theme.AccentColor
	// Use opaque white so button areas produce SPU pixel value 1 (white).
	// BTTN_GXCOL_NS Group 0 (Normal) has alpha=0 → invisible at rest.
	// BTTN_GXCOL_NS Group 1 (Selected) gives a semi-transparent white highlight.
	if err := buildMenuOverlay(ctx, overlayPath, buttons, width, height, "white", logFn); err != nil {
		return err
	}
	if err := buildMenuOverlay(ctx, highlightPath, buttons, width, height, fmt.Sprintf("%s@0.35", accent), logFn); err != nil {
		return err
	}
	if err := buildMenuOverlay(ctx, selectPath, buttons, width, height, fmt.Sprintf("%s@0.65", accent), logFn); err != nil {
		return err
	}
	return nil
}

func buildMenuOverlay(ctx context.Context, outputPath string, buttons []dvdMenuButton, width, height int, boxColor string, logFn func(string)) error {
	filterParts := []string{}
	for _, btn := range buttons {
		filterParts = append(filterParts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s:t=fill",
			btn.X0, btn.Y0, btn.X1-btn.X0, btn.Y1-btn.Y0, boxColor))
	}
	filterChain := strings.Join(filterParts, ",")
	if filterChain == "" {
		filterChain = "null"
	}

	// Use format=rgba so the transparent background is preserved as alpha=0 in
	// the output PNG. Without explicit rgba, FFmpeg may downgrade to yuv and
	// lose the alpha channel, making non-button areas map to an opaque SPU color.
	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=black@0:s=%dx%d,format=rgba", width, height),
		"-vf", filterChain,
		"-frames:v", "1",
		outputPath,
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

// runNativeSpumux creates a proper DVD menu VOB containing:
//  1. A NAV_PCK at sector 0 (with PCI button coordinates for M3)
//  2. An MPEG-2 still video encoded from bgImagePath (M1/M2)
//  3. An SPU subpicture packet from overlayPath (button highlights)
//
// The video is generated by invoking ffmpeg with the background PNG.
// region must be "pal" for PAL (720×576, 25 fps) or anything else for NTSC (720×480, 29.97 fps).
// duration is the still-image hold time in seconds (0 → defaults to 10).
func runNativeSpumux(ctx context.Context, overlayPath, bgImagePath, outputPath, region string, duration float64, buttons []dvdMenuButton, logFn func(string)) error {
	// ── Determine video parameters from region ────────────────────────────────
	width, height := dvdMenuDimensions(region)
	fps := "30000/1001" // NTSC default
	fpsVal := 29.97
	if strings.ToLower(region) == "pal" {
		fps = "25"
		fpsVal = 25.0
	}
	if duration <= 0 {
		duration = 10.0
	}

	// ── Encode background PNG as MPEG-2 still video via ffmpeg ────────────────
	// Use -f mpeg2video to produce a raw MPEG-2 elementary stream (no PS container).
	// The vob.Muxer wraps ES chunks in PS packs; feeding it an already-PS-muxed
	// file from ffmpeg would double-wrap the data and corrupt the VOB.
	workDir := filepath.Dir(outputPath)
	videoTemp := filepath.Join(workDir, "menu_video_temp.m2v")
	videoArgs := []string{
		"-y",
		"-loop", "1",
		"-i", bgImagePath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-vcodec", "mpeg2video",
		"-b:v", "4000k",
		"-maxrate", "9000k",
		"-bufsize", "1835k",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-r", fps,
		"-pix_fmt", "yuv420p",
		"-an",
		"-f", "mpeg2video",
		"-y",
		videoTemp,
	}
	if logFn != nil {
		logFn(fmt.Sprintf(">> Encoding MPEG-2 background: %dx%d @ %s fps", width, height, fps))
	}
	if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), videoArgs, logFn); err != nil {
		return fmt.Errorf("encode menu background video: %w", err)
	}

	// ── Encode SPU (button highlights) using native Go encoder ────────────────
	overlayFile, err := os.Open(overlayPath)
	if err != nil {
		return fmt.Errorf("open overlay: %w", err)
	}
	img, _, err := image.Decode(overlayFile)
	overlayFile.Close()
	if err != nil {
		return fmt.Errorf("decode overlay: %w", err)
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	logging.Info(logging.CatDVD, "Native SPU encoding menu: %dx%d", imgW, imgH)

	enc := spu.NewMenuEncoder(imgW, imgH)
	enc.SetPalette(spu.DefaultPalette())
	spuData, err := enc.EncodeMenuImage(img, spu.DefaultSPUOptions())
	if err != nil {
		return fmt.Errorf("encode SPU: %w", err)
	}

	// ── Write the final DVD VOB natively (NAV_PCK + video + SPU) ─────────────
	// ffmpeg's -f dvd muxer does NOT produce proper NAV packs at sector boundaries,
	// which causes VLC/libdvdnav to crash. We write the VOB directly using the
	// native vob.Muxer which produces spec-compliant NAV_PCKs.
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output VOB: %w", err)
	}

	m := vob.NewMuxer(outFile)
	m.SetFrameRate(fpsVal)

	// Sector 0: NAV_PCK with button table (M3 compliance)
	if len(buttons) > 0 {
		pciBtns := make([]vob.PCIButton, len(buttons))
		for i, b := range buttons {
			prev := uint8(i)
			next := uint8(i + 2)
			if i == 0 {
				prev = uint8(len(buttons))
			}
			if i == len(buttons)-1 {
				next = 1
			}
			pciBtns[i] = vob.PCIButton{
				X0: b.X0, Y0: b.Y0, X1: b.X1, Y1: b.Y1,
				Up: prev, Down: next, Left: uint8(i + 1), Right: uint8(i + 1),
				CmdNr: uint8(i + 1),
			}
		}
		pci := &vob.PCIPacket{
			Buttons: pciBtns,
			HL_GI: vob.HL_GI{
				BTN_SL_NS: 1,
				BTN_NS:    uint8(len(buttons)),
			},
		}
		if err := m.WriteNAV_PCK(pci, &vob.DSIPacket{}); err != nil {
			outFile.Close()
			return fmt.Errorf("write NAV_PCK: %w", err)
		}
		if logFn != nil {
			logFn(fmt.Sprintf(">> NAV_PCK written at sector 0 (%d buttons)", len(buttons)))
		}
	} else {
		if err := m.WriteNAV_PCK(&vob.PCIPacket{}, &vob.DSIPacket{}); err != nil {
			outFile.Close()
			return fmt.Errorf("write NAV_PCK: %w", err)
		}
	}

	// Read the MPEG-2 video elementary stream and write it as video PES packets
	videoData, err := os.ReadFile(videoTemp)
	if err != nil {
		outFile.Close()
		return fmt.Errorf("read video temp: %w", err)
	}

	// Split video ES into VOBU-sized chunks (each VOBU = ~0.4-0.5s of video)
	// For a still image, we write one VOBU per GOP (group of pictures).
	// Find GOP boundaries by looking for sequence headers or picture start codes.
	frameCount := 0
	pts90 := uint64(0)

	ticksPerFrame90 := m.VideoFrameTicks() / 300

	// Write video in chunks, inserting NAV_PCKs at regular intervals
	chunkSize := 2000 // Max PES payload per pack (leaves room for headers)
	for offset := 0; offset < len(videoData); {
		// Determine chunk size
		end := offset + chunkSize
		if end > len(videoData) {
			end = len(videoData)
		}
		chunk := videoData[offset:end]

		if err := m.WriteVideo(chunk, pts90); err != nil {
			outFile.Close()
			return fmt.Errorf("write video: %w", err)
		}

		frameCount++
		pts90 += ticksPerFrame90

		// Insert NAV_PCK at regular intervals
		if frameCount%15 == 0 {
			if len(buttons) > 0 {
				pciBtns := make([]vob.PCIButton, len(buttons))
				for i, b := range buttons {
					prev := uint8(i)
					next := uint8(i + 2)
					if i == 0 {
						prev = uint8(len(buttons))
					}
					if i == len(buttons)-1 {
						next = 1
					}
					pciBtns[i] = vob.PCIButton{
						X0: b.X0, Y0: b.Y0, X1: b.X1, Y1: b.Y1,
						Up: prev, Down: next, Left: uint8(i + 1), Right: uint8(i + 1),
						CmdNr: uint8(i + 1),
					}
				}
				pci := &vob.PCIPacket{
					Buttons:     pciBtns,
					LVOBU_S_PTM: uint32(pts90),
					LVOBU_E_PTM: uint32(pts90 + ticksPerFrame90*15),
					HL_GI: vob.HL_GI{
						BTN_SL_NS: 1,
						BTN_NS:    uint8(len(buttons)),
					},
				}
				if err := m.WriteNAV_PCK(pci, &vob.DSIPacket{}); err != nil {
					outFile.Close()
					return fmt.Errorf("write NAV_PCK: %w", err)
				}
			} else {
				if err := m.WriteNAV_PCK(&vob.PCIPacket{}, &vob.DSIPacket{}); err != nil {
					outFile.Close()
					return fmt.Errorf("write NAV_PCK: %w", err)
				}
			}
		}

		offset = end
	}

	// Write SPU packet
	if err := m.WriteSPU(spuData, vob.SubStreamSPUBase, 0); err != nil {
		outFile.Close()
		return fmt.Errorf("write SPU: %w", err)
	}

	// Final NAV_PCK
	if len(buttons) > 0 {
		pciBtns := make([]vob.PCIButton, len(buttons))
		for i, b := range buttons {
			prev := uint8(i)
			next := uint8(i + 2)
			if i == 0 {
				prev = uint8(len(buttons))
			}
			if i == len(buttons)-1 {
				next = 1
			}
			pciBtns[i] = vob.PCIButton{
				X0: b.X0, Y0: b.Y0, X1: b.X1, Y1: b.Y1,
				Up: prev, Down: next, Left: uint8(i + 1), Right: uint8(i + 1),
				CmdNr: uint8(i + 1),
			}
		}
		if err := m.WriteNAV_PCK(&vob.PCIPacket{
			Buttons: pciBtns,
			HL_GI: vob.HL_GI{
				BTN_SL_NS: 1,
				BTN_NS:    uint8(len(buttons)),
			},
		}, &vob.DSIPacket{}); err != nil {
			outFile.Close()
			return fmt.Errorf("write final NAV_PCK: %w", err)
		}
	} else {
		if err := m.WriteNAV_PCK(&vob.PCIPacket{}, &vob.DSIPacket{}); err != nil {
			outFile.Close()
			return fmt.Errorf("write final NAV_PCK: %w", err)
		}
	}

	if err := outFile.Close(); err != nil {
		return fmt.Errorf("close output VOB: %w", err)
	}

	if logFn != nil {
		logFn(fmt.Sprintf(">> Menu VOB written: %d sectors, %d NAV_PCKs", m.CurrentSector(), len(m.NAVPCKSectors)))
	}

	// ── Clean up temporary files ──────────────────────────────────────────────
	_ = os.Remove(videoTemp)

	logging.Info(logging.CatDVD, "Menu VOB complete: %s (%dx%d, %s fps, %d buttons)", outputPath, width, height, fps, len(buttons))
	return nil
}

// buildMenuSPU creates a proper DVD menu VOB using native Go SPU encoder + ffmpeg MPEG-2 video.
// bgPath is the background PNG image to encode as MPEG-2 still video.
// region is "pal" or "ntsc" (controls dimensions and frame rate).
func buildMenuSPU(ctx context.Context, overlayPath, menuSpuPath, bgPath, region string, buttons []dvdMenuButton, logFn func(string)) error {
	if logFn != nil {
		logFn(">> Native SPU encoder + MPEG-2 background")
	}
	return runNativeSpumux(ctx, overlayPath, bgPath, menuSpuPath, region, 10.0, buttons, logFn)
}

func findVTLogoPath() string {
	search := []string{
		filepath.Join("assets", "logo", "VT_Icon.png"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		search = append(search, filepath.Join(dir, "assets", "logo", "VT_Icon.png"))
	}
	for _, p := range search {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findMenuFontPath() string {
	wd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(wd, "assets", "fonts", "IBMPlexMono-Regular.ttf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		p := filepath.Join(dir, "assets", "fonts", "IBMPlexMono-Regular.ttf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return embeddedFontPath()
}

var cachedEmbeddedFontPath string

func embeddedFontPath() string {
	if cachedEmbeddedFontPath != "" {
		if _, err := os.Stat(cachedEmbeddedFontPath); err == nil {
			return cachedEmbeddedFontPath
		}
	}
	// Use a fixed name so repeated runs overwrite the same file instead of
	// accumulating hundreds of temp files that are never cleaned up.
	fixed := filepath.Join(os.TempDir(), "videotools-font.ttf")
	if err := os.WriteFile(fixed, ibmPlexMonoTTF, 0o644); err != nil {
		logging.Error(logging.CatDVD, "failed to write embedded font: %v", err)
		return ""
	}
	cachedEmbeddedFontPath = fixed
	return fixed
}

func resolveMenuTheme(theme *MenuTheme) *MenuTheme {
	if theme == nil {
		return menuThemes["VideoTools"]
	}
	if theme.Name == "" {
		return menuThemes["VideoTools"]
	}
	// Handle custom theme
	if theme.IsCustom || theme.Name == "Custom" {
		// Create a custom theme with user-specified colors
		customTheme := &MenuTheme{
			Name:            "Custom",
			BackgroundColor: theme.BackgroundColor,
			HeaderColor:     theme.BackgroundColor,
			TextColor:       theme.TextColor,
			AccentColor:     theme.AccentColor,
			IsCustom:        true,
		}
		// Use defaults for empty colors
		if customTheme.BackgroundColor == "" {
			customTheme.BackgroundColor = "0x000000"
		}
		if customTheme.TextColor == "" {
			customTheme.TextColor = "0xFFFFFF"
		}
		if customTheme.AccentColor == "" {
			customTheme.AccentColor = "0xAAAAAA"
		}
		return customTheme
	}
	if resolved, ok := menuThemes[theme.Name]; ok {
		return resolved
	}
	return menuThemes["VideoTools"]
}

func escapeFontPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.ReplaceAll(p, ":", "\\:")
	p = strings.ReplaceAll(p, "'", "'\\''")
	return "'" + p + "'"
}

func menuFontArg(theme *MenuTheme) string {
	if theme != nil && theme.FontPath != "" {
		if _, err := os.Stat(theme.FontPath); err == nil {
			return fmt.Sprintf("fontfile=%s", escapeFontPath(theme.FontPath))
		}
	}
	if p := embeddedFontPath(); p != "" {
		return fmt.Sprintf("fontfile=%s", escapeFontPath(p))
	}
	return "font=monospace"
}

func resolveMenuLogoPath(logo menuLogo) string {
	if strings.TrimSpace(logo.Path) != "" {
		if _, err := os.Stat(logo.Path); err == nil {
			return logo.Path
		}
		logging.Debug(logging.CatModule, "custom logo not found: %s", logo.Path)
		return ""
	}
	defaultPath := filepath.Join("assets", "logo", "VT_Logo.png")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	logging.Debug(logging.CatModule, "default logo not found: %s", defaultPath)
	return ""
}

func resolveMenuLogoScale(logo menuLogo) float64 {
	if logo.Scale <= 0 {
		return 1.0
	}
	if logo.Scale < 0.2 {
		return 0.2
	}
	if logo.Scale > 2.0 {
		return 2.0
	}
	return logo.Scale
}

func resolveMenuLogoScaleExpr(logo menuLogo, width, height int) string {
	scale := resolveMenuLogoScale(logo)
	maxW := float64(width) * 0.25
	maxH := float64(height) * 0.25
	// Use simpler scale syntax without w=/h= named parameters
	return fmt.Sprintf("scale='min(iw*%.2f,%.0f)':'min(ih*%.2f,%.0f)':force_original_aspect_ratio=decrease", scale, maxW, scale, maxH)
}

func resolveMenuLogoPosition(logo menuLogo, width, height int) string {
	margin := logo.Margin
	if margin < 0 {
		margin = 0
	}
	switch logo.Position {
	case "Top Left":
		return fmt.Sprintf("%d:%d", margin, margin)
	case "Bottom Left":
		return fmt.Sprintf("%d:H-h-%d", margin, margin)
	case "Bottom Right":
		return fmt.Sprintf("W-w-%d:H-h-%d", margin, margin)
	case "Center":
		return "(W-w)/2:(H-h)/2"
	default:
		return fmt.Sprintf("W-w-%d:%d", margin, margin)
	}
}

func escapeDrawtextText(text string) string {
	// Strip ALL special characters - only keep letters, numbers, and spaces
	var result strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}
	// Clean up multiple spaces
	cleaned := strings.Join(strings.Fields(result.String()), " ")
	return cleaned
}

