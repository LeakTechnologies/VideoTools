package main

import (
	"fmt"
	"os/exec"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// AI Helper Functions (smaller, manageable functions)

// detectAIUpscaleBackend returns the available Real-ESRGAN backend ("ncnn", "python", or "").
func detectAIUpscaleBackend() string {
	if _, err := exec.LookPath("realesrgan-ncnn-vulkan"); err == nil {
		return "ncnn"
	}

	cmd := exec.Command("python3", "-c", "import realesrgan")
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err == nil {
		return "python"
	}

	cmd = exec.Command("python", "-c", "import realesrgan")
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err == nil {
		return "python"
	}

	return ""
}

// checkAIFaceEnhanceAvailable verifies whether face enhancement tooling is available.
func checkAIFaceEnhanceAvailable(backend string) bool {
	if backend != "python" {
		return false
	}
	cmd := exec.Command("python3", "-c", "import realesrgan, gfpgan")
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err == nil {
		return true
	}
	cmd = exec.Command("python", "-c", "import realesrgan, gfpgan")
	utils.ApplyNoWindow(cmd)
	return cmd.Run() == nil
}

func aiUpscaleModelOptions() []string {
	return []string{
		"General (RealESRGAN_x4plus)",
		"Anime/Illustration (RealESRGAN_x4plus_anime_6B)",
		"Anime Video (realesr-animevideov3)",
		"General Tiny (realesr-general-x4v3)",
		"2x General (RealESRGAN_x2plus)",
		"Clean Restore (realesrnet-x4plus)",
	}
}

func aiUpscaleModelID(label string) string {
	switch label {
	case "Anime/Illustration (RealESRGAN_x4plus_anime_6B)":
		return "realesrgan-x4plus-anime"
	case "Anime Video (realesr-animevideov3)":
		return "realesr-animevideov3"
	case "General Tiny (realesr-general-x4v3)":
		return "realesr-general-x4v3"
	case "2x General (RealESRGAN_x2plus)":
		return "realesrgan-x2plus"
	case "Clean Restore (realesrnet-x4plus)":
		return "realesrnet-x4plus"
	default:
		return "realesrgan-x4plus"
	}
}

func aiUpscaleModelLabel(modelID string) string {
	switch modelID {
	case "realesrgan-x4plus-anime":
		return "Anime/Illustration (RealESRGAN_x4plus_anime_6B)"
	case "realesr-animevideov3":
		return "Anime Video (realesr-animevideov3)"
	case "realesr-general-x4v3":
		return "General Tiny (realesr-general-x4v3)"
	case "realesrgan-x2plus":
		return "2x General (RealESRGAN_x2plus)"
	case "realesrnet-x4plus":
		return "Clean Restore (realesrnet-x4plus)"
	case "realesrgan-x4plus":
		return "General (RealESRGAN_x4plus)"
	default:
		return ""
	}
}

// parseResolutionPreset parses resolution preset strings and returns target dimensions and whether to preserve aspect.
// Special presets like "Match Source" and relative (2X/4X) use source dimensions to preserve AR.
func parseResolutionPreset(preset string, srcW, srcH int) (width, height int, preserveAspect bool, err error) {
	// Default: preserve aspect
	preserveAspect = true

	// Sanitize source
	if srcW < 1 || srcH < 1 {
		srcW, srcH = 1920, 1080 // fallback to avoid zero division
	}

	switch preset {
	case "", "Match Source":
		return srcW, srcH, true, nil
	case "2X (relative)":
		return srcW * 2, srcH * 2, true, nil
	case "4X (relative)":
		return srcW * 4, srcH * 4, true, nil
	}

	presetMap := map[string][2]int{
		"720p (1280x720)":   {1280, 720},
		"1080p (1920x1080)": {1920, 1080},
		"1440p (2560x1440)": {2560, 1440},
		"4K (3840x2160)":    {3840, 2160},
		"8K (7680x4320)":    {7680, 4320},
		"720p":              {1280, 720},
		"1080p":             {1920, 1080},
		"1440p":             {2560, 1440},
		"4K":                {3840, 2160},
		"8K":                {7680, 4320},
	}

	if dims, ok := presetMap[preset]; ok {
		// Keep aspect by default: use target height and let FFmpeg derive width
		return dims[0], dims[1], true, nil
	}

	return 0, 0, true, fmt.Errorf("unknown resolution preset: %s", preset)
}

// buildUpscaleFilter builds FFmpeg scale filter string with selected method
func buildUpscaleFilter(targetWidth, targetHeight int, method string, preserveAspect bool) string {
	// Ensure even dimensions for encoders
	makeEven := func(v int) int {
		if v%2 != 0 {
			return v + 1
		}
		return v
	}

	h := makeEven(targetHeight)
	w := targetWidth
	if preserveAspect || w <= 0 {
		w = -2 // FFmpeg will derive width from height while preserving AR
	}
	return fmt.Sprintf("scale=%d:%d:flags=%s", w, h, method)
}

// sanitizeForPath creates a simple slug for filenames from user-visible labels
func sanitizeForPath(label string) string {
	r := strings.NewReplacer(" ", "", "(", "", ")", "", "×", "x", "/", "-", "\\", "-", ":", "-", ",", "", ".", "", "_", "")
	return strings.ToLower(r.Replace(label))
}

func (s *appState) showUpscaleView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "upscale"
	s.setContent(buildUpscaleView(s))
}

// buildUpscaleView and executeUpscaleJob will be added here incrementally...
