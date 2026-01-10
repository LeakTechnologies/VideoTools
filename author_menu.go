package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type dvdMenuButton struct {
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
}

type menuLogoOptions struct {
	Enabled  bool
	Path     string
	Position string
	Scale    float64
	Margin   int
}

// MenuTemplate defines the interface for a DVD menu generator.
type MenuTemplate interface {
	Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error)
}

var menuTemplates = map[string]MenuTemplate{
	"Simple": &SimpleMenu{},
	"Dark":   &DarkMenu{},
	"Poster": &PosterMenu{},
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
}

// SimpleMenu is a basic menu template.
type SimpleMenu struct{}

// Generate creates a simple DVD menu.
func (t *SimpleMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, width, height)
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
	menuMpg := filepath.Join(workDir, "menu.mpg")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")
	spumuxXML := filepath.Join(workDir, "menu_spu.xml")

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
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect, logFn); err != nil {
		return "", nil, err
	}
	if err := writeSpumuxXML(spumuxXML, overlayPath, highlightPath, selectPath, buttons); err != nil {
		return "", nil, err
	}
	if err := runSpumux(ctx, spumuxXML, menuMpg, menuSpu, logFn); err != nil {
		return "", nil, err
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// DarkMenu is a dark-themed menu template.
type DarkMenu struct{}

// Generate creates a dark-themed DVD menu.
func (t *DarkMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, width, height)
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
	menuMpg := filepath.Join(workDir, "menu.mpg")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")
	spumuxXML := filepath.Join(workDir, "menu_spu.xml")

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
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect, logFn); err != nil {
		return "", nil, err
	}
	if err := writeSpumuxXML(spumuxXML, overlayPath, highlightPath, selectPath, buttons); err != nil {
		return "", nil, err
	}
	if err := runSpumux(ctx, spumuxXML, menuMpg, menuSpu, logFn); err != nil {
		return "", nil, err
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

// PosterMenu is a template that uses a poster image as a background.
type PosterMenu struct{}

// Generate creates a poster-themed DVD menu.
func (t *PosterMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) (string, []dvdMenuButton, error) {
	width, height := dvdMenuDimensions(region)
	buttons := buildDVDMenuButtons(chapters, width, height)
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
	menuMpg := filepath.Join(workDir, "menu.mpg")
	menuSpu := filepath.Join(workDir, "menu_spu.mpg")
	spumuxXML := filepath.Join(workDir, "menu_spu.xml")

	if logFn != nil {
		logFn("Building DVD menu assets with PosterMenu template...")
	}

	if err := buildPosterMenuBackground(ctx, bgPath, title, buttons, width, height, backgroundImage, resolveMenuTheme(theme), logo, logFn); err != nil {
		return "", nil, err
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height, resolveMenuTheme(theme), logFn); err != nil {
		return "", nil, err
	}
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect, logFn); err != nil {
		return "", nil, err
	}
	if err := writeSpumuxXML(spumuxXML, overlayPath, highlightPath, selectPath, buttons); err != nil {
		return "", nil, err
	}
	if err := runSpumux(ctx, spumuxXML, menuMpg, menuSpu, logFn); err != nil {
		return "", nil, err
	}
	if logFn != nil {
		logFn(fmt.Sprintf("DVD menu created: %s", filepath.Base(menuSpu)))
	}
	return menuSpu, buttons, nil
}

func buildDVDMenuAssets(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, logFn func(string), template MenuTemplate, backgroundImage string, theme *MenuTheme, logo menuLogoOptions) (string, []dvdMenuButton, error) {
	if template == nil {
		template = &SimpleMenu{}
	}
	return template.Generate(ctx, workDir, title, region, aspect, chapters, backgroundImage, resolveMenuTheme(theme), logo, logFn)
}

func dvdMenuDimensions(region string) (int, int) {
	if strings.ToLower(region) == "pal" {
		return 720, 576
	}
	return 720, 480
}

func buildDVDMenuButtons(chapters []authorChapter, width, height int) []dvdMenuButton {
	buttons := []dvdMenuButton{
		{
			Label:   "Play",
			Command: "jump title 1;",
		},
	}

	maxChapters := 8
	if len(chapters) < maxChapters {
		maxChapters = len(chapters)
	}
	for i := 0; i < maxChapters; i++ {
		label := fmt.Sprintf("Chapter %d", i+1)
		if title := strings.TrimSpace(chapters[i].Title); title != "" {
			label = fmt.Sprintf("Chapter %d: %s", i+1, utils.ShortenMiddle(title, 34))
		}
		buttons = append(buttons, dvdMenuButton{
			Label:   label,
			Command: fmt.Sprintf("jump title 1 chapter %d;", i+1),
		})
	}

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

func buildMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	bgColor := theme.BackgroundColor
	headerColor := theme.HeaderColor
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", fontArg, textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=122:text='%s'", fontArg, textColor, escapeDrawtextText("Select a title or chapter to play")),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)
	if logo.Enabled {
		logoPath := resolveMenuLogoPath(logo)
		if logoPath != "" {
			posExpr := resolveMenuLogoPosition(logo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo, width, height)
			args = append(args, "-i", logoPath)
			filterExpr = fmt.Sprintf("[0:v]%s[bg];[1:v]%s[logo];[bg][logo]overlay=%s", filterChain, scaleExpr, posExpr)
		}
	}
	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildDarkMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	bgColor := "0x000000"
	headerColor := "0x111111"
	textColor := theme.TextColor
	accentColor := theme.AccentColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", fontArg, textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", fontArg, textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=16:x=36:y=122:text='%s'", fontArg, textColor, escapeDrawtextText("Select a title or chapter to play")),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height)}
	filterExpr := fmt.Sprintf("[0:v]%s[bg]", filterChain)
	if logo.Enabled {
		logoPath := resolveMenuLogoPath(logo)
		if logoPath != "" {
			posExpr := resolveMenuLogoPosition(logo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo, width, height)
			args = append(args, "-i", logoPath)
			filterExpr = fmt.Sprintf("[0:v]%s[bg];[1:v]%s[logo];[bg][logo]overlay=%s", filterChain, scaleExpr, posExpr)
		}
	}
	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func buildPosterMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, backgroundImage string, theme *MenuTheme, logo menuLogoOptions, logFn func(string)) error {
	theme = resolveMenuTheme(theme)
	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	textColor := theme.TextColor
	fontArg := menuFontArg(theme)

	filterParts := []string{
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", fontArg, textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", fontArg, textColor, escapeDrawtextText(safeTitle)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=%s:fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", fontArg, textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{"-y", "-i", backgroundImage}
	filterExpr := fmt.Sprintf("[0:v]scale=%d:%d,%s[bg]", width, height, filterChain)
	if logo.Enabled {
		logoPath := resolveMenuLogoPath(logo)
		if logoPath != "" {
			posExpr := resolveMenuLogoPosition(logo, width, height)
			scaleExpr := resolveMenuLogoScaleExpr(logo, width, height)
			args = append(args, "-i", logoPath)
			filterExpr = fmt.Sprintf("[0:v]scale=%d:%d,%s[bg];[1:v]%s[logo];[bg][logo]overlay=%s", width, height, filterChain, scaleExpr, posExpr)
		}
	}
	args = append(args, "-filter_complex", filterExpr, "-frames:v", "1", outputPath)
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

func buildMenuMPEG(ctx context.Context, bgPath, outputPath, region, aspect string, logFn func(string)) error {
	scale := "720:480"
	if strings.ToLower(region) == "pal" {
		scale = "720:576"
	}
	args := []string{
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
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, logFn)
}

func writeSpumuxXML(path, overlayPath, highlightPath, selectPath string, buttons []dvdMenuButton) error {
	var b strings.Builder
	b.WriteString("<subpictures>\n")
	b.WriteString("  <stream>\n")
	b.WriteString(fmt.Sprintf("    <spu start=\"00:00:00.00\" end=\"00:00:30.00\" image=\"%s\" highlight=\"%s\" select=\"%s\" force=\"yes\"/>",
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

func runSpumux(ctx context.Context, spumuxXML, inputMpg, outputMpg string, logFn func(string)) error {
	args := []string{" -m", "dvd", spumuxXML}
	if logFn != nil {
		logFn(fmt.Sprintf(">> spumux -m dvd %s < %s > %s", spumuxXML, filepath.Base(inputMpg), filepath.Base(outputMpg)))
	}
	cmd := exec.CommandContext(ctx, "spumux", args...)
	inputFile, err := os.Open(inputMpg)
	if err != nil {
		return fmt.Errorf("open spumux input: %w", err)
	}
	defer inputFile.Close()
	cmd.Stdin = inputFile
	outFile, err := os.Create(outputMpg)
	if err != nil {
		return fmt.Errorf("create spumux output: %w", err)
	}
	defer outFile.Close()
	cmd.Stdout = outFile
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logging.Debug(logging.CatSystem, "spumux stderr: %s", stderr.String())
		return fmt.Errorf("spumux failed: %w", err)
	}
	return nil
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
	search := []string{
		filepath.Join("assets", "fonts", "IBMPlexMono-Regular.ttf"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		search = append(search, filepath.Join(dir, "assets", "fonts", "IBMPlexMono-Regular.ttf"))
	}
	for _, p := range search {
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
	if resolved, ok := menuThemes[theme.Name]; ok {
		return resolved
	}
	return menuThemes["VideoTools"]
}

func menuFontArg(theme *MenuTheme) string {
	// Try FontPath first (specific font file)
	if theme != nil && theme.FontPath != "" {
		if _, err := os.Stat(theme.FontPath); err == nil {
			return fmt.Sprintf("fontfile='%s'", theme.FontPath)
		}
	}
	// FontPath doesn't exist or is empty - use system-wide fonts
	// Only use FontName if it's a known universally available font
	if theme != nil && theme.FontName != "" {
		safeFonts := map[string]bool{
			"DejaVu Sans Mono":   true,
			"DejaVu Sans":        true,
			"Liberation Mono":    true,
			"Liberation Sans":    true,
			"FreeMono":           true,
			"FreeSans":           true,
		}
		if safeFonts[theme.FontName] {
			return fmt.Sprintf("font='%s'", theme.FontName)
		}
	}
	// Fallback to most universally available monospace font on Linux
	return "font='monospace'"
}

func resolveMenuLogoPath(logo menuLogoOptions) string {
	if strings.TrimSpace(logo.Path) != "" {
		return logo.Path
	}
	return filepath.Join("assets", "logo", "VT_Logo.png")
}

func resolveMenuLogoScale(logo menuLogoOptions) float64 {
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

func resolveMenuLogoScaleExpr(logo menuLogoOptions, width, height int) string {
	scale := resolveMenuLogoScale(logo)
	maxW := float64(width) * 0.25
	maxH := float64(height) * 0.25
	return fmt.Sprintf("scale=w='min(iw*%.2f,%.0f)':h='min(ih*%.2f,%.0f)':force_original_aspect_ratio=decrease", scale, maxW, scale, maxH)
}

func resolveMenuLogoPosition(logo menuLogoOptions, width, height int) string {
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
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, ":", "\\:")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	return escaped
}
