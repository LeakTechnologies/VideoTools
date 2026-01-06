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

// MenuTemplate defines the interface for a DVD menu generator.
type MenuTemplate interface {
	Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, logFn func(string)) (string, []dvdMenuButton, error)
}

var menuTemplates = map[string]MenuTemplate{
	"Simple": &SimpleMenu{},
	"Dark":   &DarkMenu{},
	"Poster": &PosterMenu{},
}

// SimpleMenu is a basic menu template.
type SimpleMenu struct{}

// Generate creates a simple DVD menu.
func (t *SimpleMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, logFn func(string)) (string, []dvdMenuButton, error) {
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
		if err := buildMenuBackground(ctx, bgPath, title, buttons, width, height); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height); err != nil {
		return "", nil, err
	}
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect); err != nil {
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
func (t *DarkMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, logFn func(string)) (string, []dvdMenuButton, error) {
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
		if err := buildDarkMenuBackground(ctx, bgPath, title, buttons, width, height); err != nil {
			return "", nil, err
		}
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height); err != nil {
		return "", nil, err
	}
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect); err != nil {
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
func (t *PosterMenu) Generate(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, backgroundImage string, logFn func(string)) (string, []dvdMenuButton, error) {
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

	if err := buildPosterMenuBackground(ctx, bgPath, title, buttons, width, height, backgroundImage); err != nil {
		return "", nil, err
	}

	if err := buildMenuOverlays(ctx, overlayPath, highlightPath, selectPath, buttons, width, height); err != nil {
		return "", nil, err
	}
	if err := buildMenuMPEG(ctx, bgPath, menuMpg, region, aspect); err != nil {
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

func buildDVDMenuAssets(ctx context.Context, workDir, title, region, aspect string, chapters []authorChapter, logFn func(string), template MenuTemplate, backgroundImage string) (string, []dvdMenuButton, error) {
	if template == nil {
		template = &SimpleMenu{}
	}
	return template.Generate(ctx, workDir, title, region, aspect, chapters, backgroundImage, logFn)
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

func buildMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int) error {
	logoPath := findVTLogoPath()
	if logoPath == "" {
		return fmt.Errorf("VT logo not found for menu rendering")
	}

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	bgColor := "0x0f172a"
	headerColor := "0x1f2937"
	textColor := "white"
	accentColor := "0x7c3aed"

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=16:x=36:y=122:text='%s'", textColor, escapeDrawtextText("Select a title or chapter to play")),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height),
		"-i", logoPath,
		"-filter_complex", fmt.Sprintf("[0:v]%s[bg];[1:v]scale=72:-1[logo];[bg][logo]overlay=W-w-36:18", filterChain),
		"-frames:v", "1",
		outputPath,
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, nil)
}

func buildDarkMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int) error {
	logoPath := findVTLogoPath()
	if logoPath == "" {
		return fmt.Errorf("VT logo not found for menu rendering")
	}

	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	bgColor := "0x000000"
	headerColor := "0x111111"
	textColor := "white"
	accentColor := "0xeeeeee"

	filterParts := []string{
		fmt.Sprintf("drawbox=x=0:y=0:w=%d:h=72:color=%s:t=fill", width, headerColor),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", textColor, escapeDrawtextText(safeTitle)),
		fmt.Sprintf("drawbox=x=36:y=108:w=%d:h=2:color=%s:t=fill", width-72, accentColor),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=16:x=36:y=122:text='%s'", textColor, escapeDrawtextText("Select a title or chapter to play")),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=%s:s=%dx%d", bgColor, width, height),
		"-i", logoPath,
		"-filter_complex", fmt.Sprintf("[0:v]%s[bg];[1:v]scale=72:-1[logo];[bg][logo]overlay=W-w-36:18", filterChain),
		"-frames:v", "1",
		outputPath,
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, nil)
}

func buildPosterMenuBackground(ctx context.Context, outputPath, title string, buttons []dvdMenuButton, width, height int, backgroundImage string) error {
	safeTitle := utils.ShortenMiddle(strings.TrimSpace(title), 40)
	if safeTitle == "" {
		safeTitle = "DVD Menu"
	}

	textColor := "white"

	filterParts := []string{
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=28:x=36:y=20:text='%s'", textColor, escapeDrawtextText("VideoTools DVD")),
		fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=18:x=36:y=80:text='%s'", textColor, escapeDrawtextText(safeTitle)),
	}

	for i, btn := range buttons {
		label := escapeDrawtextText(btn.Label)
		y := 184 + i*34
		filterParts = append(filterParts, fmt.Sprintf("drawtext=font='DejaVu Sans Mono':fontcolor=%s:fontsize=20:x=110:y=%d:text='%s'", textColor, y, label))
	}

	filterChain := strings.Join(filterParts, ",")

	args := []string{
		"-y",
		"-i", backgroundImage,
		"-vf", fmt.Sprintf("scale=%d:%d,%s", width, height, filterChain),
		"-frames:v", "1",
		outputPath,
	}
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, nil)
}

func buildMenuOverlays(ctx context.Context, overlayPath, highlightPath, selectPath string, buttons []dvdMenuButton, width, height int) error {
	if err := buildMenuOverlay(ctx, overlayPath, buttons, width, height, "0x000000@0.0"); err != nil {
		return err
	}
	if err := buildMenuOverlay(ctx, highlightPath, buttons, width, height, "0xf59e0b@0.35"); err != nil {
		return err
	}
	if err := buildMenuOverlay(ctx, selectPath, buttons, width, height, "0xf59e0b@0.65"); err != nil {
		return err
	}
	return nil
}

func buildMenuOverlay(ctx context.Context, outputPath string, buttons []dvdMenuButton, width, height int, boxColor string) error {
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
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, nil)
}

func buildMenuMPEG(ctx context.Context, bgPath, outputPath, region, aspect string) error {
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
	return runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, nil)
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

func escapeDrawtextText(text string) string {
	escaped := strings.ReplaceAll(text, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, ":", "\\:")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	return escaped
}
