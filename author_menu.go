package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/spu"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/theme"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/vob"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type dvdMenuButton struct {
	ID      string
	Label   string
	Command string
	X0      int
	Y0      int
	X1      int
	Y1      int
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	ChaptersMpg     string
	ChaptersButtons []dvdMenuButton
	ExtrasMpg       string
	ExtrasButtons   []dvdMenuButton
}

func buildDVDMenuAssets(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, extras []extraItem, logFn func(string), template MenuTemplate, backgroundImage, motionBackground string, theme *MenuTheme, logo menuLogoOptions, chapterVideoPath string, chapterThumbOffset float64) (dvdMenuSet, error) {
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
		MainMpg:     mainMpg,
		MainButtons: mainButtons,
	}

	// Generate chapters menu if there are multiple chapters
	if len(chapters) > 1 {
		chaptersMenuMpg, chaptersButtons, err := buildChaptersMenuMPEGSet(ctx, workDir, title, region, aspect, chapters, backgroundImage, motionBackground, theme, logFn, chapterVideoPath, chapterThumbOffset)
		if err != nil {
			return dvdMenuSet{}, err
		}
		result.ChaptersMpg = chaptersMenuMpg
		result.ChaptersButtons = chaptersButtons
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
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
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("Extras menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

func buildChaptersMenuMPEGSet(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage, motionBackground string, theme *MenuTheme, logFn func(string), chapterVideoPath string, chapterThumbOffset float64) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildChapterMenuButtons(chapters, width, height)
	if len(buttons) == 0 {
		return "", nil, nil
	}

	bgPath := filepath.Join(workDir, "chapters_menu_bg.png")
	overlayPath := filepath.Join(workDir, "chapters_menu_overlay.png")
	highlightPath := filepath.Join(workDir, "chapters_menu_highlight.png")
	selectPath := filepath.Join(workDir, "chapters_menu_select.png")
	menuSpu := filepath.Join(workDir, "chapters_menu_spu.mpg")

	if logFn != nil {
		logFn("Building chapters menu assets...")
	}

	// Generate chapter thumbnails if video path is provided
	var chapterThumbPaths []string
	if chapterVideoPath != "" && chapterThumbOffset > 0 {
		chapterThumbPaths = generateChapterThumbnails(ctx, workDir, chapterVideoPath, chapters, chapterThumbOffset, logFn)
	}

	if err := buildChaptersMenuBackground(ctx, bgPath, title, buttons, width, height, resolveMenuTheme(theme), chapterThumbPaths, logFn); err != nil {
		return "", nil, err
	}
	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	// Use native Go SPU encoder (zero-dep)
	if err := buildMenuSPU(ctx, overlayPath, menuSpu, logFn); err != nil {
		return "", nil, fmt.Errorf("native SPU encoder: %w", err)
	}
	if logFn != nil {
		logFn(fmt.Sprintf("Chapters menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// generateChapterThumbnails creates thumbnail images for each chapter.
// Returns slice of thumbnail paths in order of chapters.
func generateChapterThumbnails(ctx context.Context, workDir, videoPath string, chapters []authorChapter, offsetSeconds float64, logFn func(string)) []string {
	var thumbPaths []string
	thumbWidth := 160
	thumbHeight := 90

	for i, chapter := range chapters {
		// Calculate timestamp: chapter start + offset
		timestamp := chapter.Timestamp + offsetSeconds
		thumbPath := filepath.Join(workDir, fmt.Sprintf("chapter_%02d_thumb.png", i+1))

		if logFn != nil {
			logFn(fmt.Sprintf("Generating thumbnail for chapter %d at %.1fs...", i+1, timestamp))
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

func buildChapterMenuButtons(chapters []authorChapter, width, height int) []dvdMenuButton {
	t := i18n.T()
	buttons := []dvdMenuButton{}

	// Add a button for each chapter
	for i, ch := range chapters {
		buttons = append(buttons, dvdMenuButton{
			Label:   ch.Title,
			Command: fmt.Sprintf("jump title 1 chapter %d;", i+1),
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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle title logo and studio logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle title logo and studio logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)

	// Handle title logo and studio logo overlays
	inputIndex := 1
	baseLayer := "[bg]"

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

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorChapterSelection)),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=52:text=%s", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=80:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=14:x=36:y=94:text=%s", fontArg, textColor, escapeDrawtextText(t.AuthorSelectChapterMenu)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		// Truncate long chapter names for display
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

	// Skip thumbnail overlays for now - filter chain complexity is causing issues
	// TODO: Re-enable with simpler filter chain approach

	// Use simple filter without thumbnails
	args = append(args, "-filter_complex", fmt.Sprintf("[0:v]%s", filterChain), "-frames:v", "1", outputPath)

	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildMenuOverlays(ctx context.Context, overlayPath, highlightPath, selectPath string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	accent := theme.AccentColor
	if err := buildMenuOverlay(ctx, overlayPath, buttons, width, height, "0x000000@0.0", logFn); err != nil {
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

	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=black@0.0:s=%dx%d", width, height),
		"-vf", filterChain,
		"-frames:v", "1",
		outputPath,
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildMenuMPEG(ctx context.Context, bgPath, outputPath, region, aspect string, motionBackground string, logFn func(string)) error {
	scale := "720:480"
	if strings.ToLower(region) == "pal" {
		scale = "720:576"
	}

	var args []string

	if motionBackground != "" && strings.Contains(strings.ToLower(motionBackground), ".mpg") {
		// Use motion background video - just transcode to DVD format
		if logFn != nil {
			logFn(fmt.Sprintf("Using motion background: %s", filepath.Base(motionBackground)))
		}
		args = []string{
			"-y",
			"-i", motionBackground,
			"-t", "30",
			"-r", "30000/1001",
			"-vf", fmt.Sprintf("scale=%s:force_original_aspect_ratio=decrease,pad=%s:(ow-iw)/2:(oh-ih)/2,format=yuv420p", scale, scale),
			"-c:v", "mpeg2video",
			"-b:v", "3000k",
			"-maxrate", "5000k",
			"-bufsize", "1835k",
			"-g", "15",
			"-pix_fmt", "yuv420p",
			"-aspect", aspect,
			"-f", "dvd",
			"-loop", "0",
			outputPath,
		}
	} else {
		// Use static background image (looped)
		args = []string{
			"-y",
			"-loop", "1",
			"-i", bgPath,
			"-t", "30",
			"-r", "30000/1001",
			"-vf", fmt.Sprintf("scale=%s,format=yuv420p", scale),
			"-c:v", "mpeg2video",
			"-b:v", "3000k",
			"-maxrate", "5000k",
			"-bufsize", "1835k",
			"-g", "15",
			"-pix_fmt", "yuv420p",
			"-aspect", aspect,
			"-f", "dvd",
			outputPath,
		}
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func writeSpumuxXML(path, overlayPath, highlightPath, selectPath string, buttons []dvdMenuButton) error {
	var b strings.Builder
	b.WriteString("<subpictures>\n")
	b.WriteString("  <stream>\n")
	b.WriteString(fmt.Sprintf("    <spu start=\"00:00:00.00\" end=\"00:00:30.00\" image=\"%s\" highlight=\"%s\" select=\"%s\" force=\"yes\">\n",
		escapeXMLAttr(overlayPath),
		escapeXMLAttr(highlightPath),
		escapeXMLAttr(selectPath),
	))
	for i, btn := range buttons {
		b.WriteString(fmt.Sprintf("      <button name=\"b%d\" x0=\"%d\" y0=\"%d\" x1=\"%d\" y1=\"%d\" />\n",
			i+1, btn.X0, btn.Y0, btn.X1, btn.Y1))
	}
	b.WriteString("    </spu>\n")
	b.WriteString("  </stream>\n")
	b.WriteString("</subpictures>\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// runNativeSpumux creates menu SPU using native Go encoder (zero-dep).
// It takes the background image and generates an SPU-encoded VOB.
func runNativeSpumux(ctx context.Context, overlayPath, outputPath string, logFn func(string)) error {
	// Load overlay image
	overlayFile, err := os.Open(overlayPath)
	if err != nil {
		return fmt.Errorf("open overlay: %w", err)
	}
	defer overlayFile.Close()

	img, _, err := image.Decode(overlayFile)
	if err != nil {
		return fmt.Errorf("decode overlay: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	logging.Info(logging.CatDVD, "Native SPU encoding menu: %dx%d", width, height)

	// Use native Go SPU encoder
	enc := spu.NewMenuEncoder(width, height)
	enc.SetPalette(spu.DefaultPalette())

	spuData, err := enc.EncodeMenuImage(img, spu.DefaultSPUOptions())
	if err != nil {
		return fmt.Errorf("encode SPU: %w", err)
	}

	// Create VOB with SPU
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	mux := vob.NewMuxer(outFile)
	mux.SetFrameRate(29.97)

	// Write SPU at PTS 0 (menu display start)
	if err := mux.WriteSPU(spuData, vob.SubStreamSPUBase, 0); err != nil {
		return fmt.Errorf("write SPU: %w", err)
	}

	// Write a minimal NAV_PCK to mark the end
	if err := mux.WriteNAV_PCK(&vob.PCIPacket{}, &vob.DSIPacket{}); err != nil {
		return fmt.Errorf("write NAV: %w", err)
	}

	logging.Info(logging.CatDVD, "Native SPU complete: %s", outputPath)
	return nil
}

// buildMenuSPU creates the menu SPU file using native Go encoder.
// This replaces the external spumux call for zero-dependency operation.
func buildMenuSPU(ctx context.Context, overlayPath, menuSpuPath string, logFn func(string)) error {
	if logFn != nil {
		logFn(">> Native SPU encoder")
	}
	return runNativeSpumux(ctx, overlayPath, menuSpuPath, logFn)
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
	// Get absolute path to working directory
	wd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(wd, "assets", "fonts", "IBMPlexMono-Regular.ttf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Try executable directory
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		p := filepath.Join(dir, "assets", "fonts", "IBMPlexMono-Regular.ttf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
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

func menuFontArg(theme *MenuTheme) string {
	// Try FontPath first (specific font file)
	if theme != nil && theme.FontPath != "" {
		if _, err := os.Stat(theme.FontPath); err == nil {
			// Escape the font path for FFmpeg filter - replace : and ' with escaped versions
			escapedPath := strings.ReplaceAll(theme.FontPath, ":", "\\:")
			escapedPath = strings.ReplaceAll(escapedPath, "'", "\\'")
			return fmt.Sprintf("fontfile=%s", escapedPath)
		}
	}
	// FontPath doesn't exist or is empty - use system-wide fonts
	// Only use FontName if it's a known universally available font
	if theme != nil && theme.FontName != "" {
		safeFonts := map[string]bool{
			"DejaVu Sans Mono": true,
			"DejaVu Sans":      true,
			"Liberation Mono":  true,
			"Liberation Sans":  true,
			"FreeMono":         true,
			"FreeSans":         true,
		}
		if safeFonts[theme.FontName] {
			return fmt.Sprintf("font=%s", theme.FontName)
		}
	}
	// Fallback to most universally available monospace font on Linux
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

func escapeXMLAttr(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return strings.ReplaceAll(value, "\"", "&quot;")
	}
	escaped := b.String()
	return strings.ReplaceAll(escaped, "\"", "&quot;")
}
