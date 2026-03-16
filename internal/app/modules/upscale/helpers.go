package upscale

import (
	"fmt"
	"os/exec"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func DetectAIUpscaleBackend() string {
	if _, err := exec.LookPath("realesrgan-ncnn-vulkan"); err == nil {
		return "ncnn"
	}

	cmd := utils.HideWindowExec("python3", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return "python"
	}

	cmd = utils.HideWindowExec("python", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return "python"
	}

	return ""
}

func CheckAIFaceEnhanceAvailable(backend string) bool {
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

func ModelOptions() []string {
	return []string{
		"General (RealESRGAN_x4plus)",
		"Anime/Illustration (RealESRGAN_x4plus_anime_6B)",
		"Anime Video (realesr-animevideov3)",
		"General Tiny (realesr-general-x4v3)",
		"2x General (RealESRGAN_x2plus)",
		"Clean Restore (realesrnet-x4plus)",
	}
}

func ModelIDFromLabel(label string) string {
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

func ModelLabelFromID(modelID string) string {
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

func ParseResolutionPreset(preset string, srcW, srcH int) (width, height int, preserveAspect bool, err error) {
	preserveAspect = true

	if srcW < 1 || srcH < 1 {
		srcW, srcH = 1920, 1080
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
		return dims[0], dims[1], true, nil
	}

	return 0, 0, true, fmt.Errorf("unknown resolution preset: %s", preset)
}

func BuildUpscaleFilter(targetWidth, targetHeight int, method string, preserveAspect bool) string {
	makeEven := func(v int) int {
		if v%2 != 0 {
			return v + 1
		}
		return v
	}

	h := makeEven(targetHeight)
	w := targetWidth
	if preserveAspect || w <= 0 {
		w = -2
	}
	return fmt.Sprintf("scale=%d:%d:flags=%s", w, h, method)
}

func SanitizeForPath(label string) string {
	r := strings.NewReplacer(
		" ", "",
		"(", "",
		")", "",
		"×", "x",
		"/", "-",
		"\\", "-",
		":", "-",
		",", "",
		".", "",
		"_", "",
		"'", "",
		"\"", "",
		"`", "",
		"!", "",
		"?", "",
		"&", "and",
	)
	return strings.ToLower(r.Replace(label))
}

func BitratePresetValue(preset string) (int, string) {
	switch preset {
	case "500 Kbps - Low":
		return 500, "500k"
	case "1 Mbps - Low":
		return 1000, "1M"
	case "1.5 Mbps - Medium-Low":
		return 1500, "1500k"
	case "2 Mbps - Medium":
		return 2000, "2M"
	case "2.5 Mbps - Medium":
		return 2500, "2500k"
	case "3 Mbps - Medium-High":
		return 3000, "3M"
	case "4 Mbps - High":
		return 4000, "4M"
	case "5 Mbps - Higher":
		return 5000, "5M"
	case "6 Mbps - High":
		return 6000, "6M"
	case "8 Mbps - Very High":
		return 8000, "8M"
	case "10 Mbps - Maximum":
		return 10000, "10M"
	default:
		return 2500, "2500k"
	}
}

func ParseCRFValue(preset string) int {
	switch preset {
	case "Lossless (CRF 0)":
		return 0
	case "Near-lossless (CRF 16)":
		return 16
	case "High (CRF 18)":
		return 18
	case "Medium (CRF 23)":
		return 23
	case "Low (CRF 28)":
		return 28
	default:
		return 23
	}
}

func ParseEncoderPreset(preset string) string {
	presetMap := map[string]string{
		"Ultra Fast": "ultrafast",
		"Super Fast": "superfast",
		"Very Fast":  "veryfast",
		"Faster":     "faster",
		"Fast":       "fast",
		"Medium":     "medium",
		"Slow":       "slow",
		"Slower":     "slower",
		"Very Slow":  "veryslow",
	}
	if v, ok := presetMap[preset]; ok {
		return v
	}
	return "slow"
}

func VideoCodecID(name string) string {
	switch name {
	case "H.264":
		return "h264"
	case "H.265":
		return "hevc"
	case "VP9":
		return "vp9"
	case "AV1":
		return "av1"
	case "Copy":
		return "copy"
	default:
		return "h264"
	}
}

func OutputFormatFromCodec(codec string) string {
	switch codec {
	case "h264":
		return "mp4"
	case "hevc":
		return "mkv"
	case "vp9":
		return "webm"
	case "av1":
		return "mkv"
	default:
		return "mp4"
	}
}

func TTAFromPreset(preset string) (enabled bool, tileSize int) {
	switch preset {
	case "Ultra Fast":
		return false, 800
	case "Fast":
		return false, 800
	case "Balanced":
		return false, 512
	case "High Quality":
		return false, 256
	case "Maximum Quality":
		return true, 0
	default:
		return false, 512
	}
}

func ValidateAIUpscaleParams(model string, scale float64) string {
	if scale < 1 || scale > 8 {
		return "AI scale must be between 1x and 8x"
	}
	if model == "" {
		return "AI model is required"
	}
	return ""
}
