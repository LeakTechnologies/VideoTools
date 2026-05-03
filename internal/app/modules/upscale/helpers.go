package upscale

import (
	"fmt"
	"os/exec"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type ModelFamily int

const (
	ModelFamilyRealESRGAN ModelFamily = iota
	ModelFamilyRealCUGAN
)

type ModelInfo struct {
	ID              string
	Label           string
	Family          ModelFamily
	SupportsDenoise bool
	SupportsTTA     bool
	DefaultScale    int
}

var modelCatalog = []ModelInfo{
	// Real-ESRGAN models
	{ID: "realesrgan-x4plus", Label: "General (RealESRGAN x4)", Family: ModelFamilyRealESRGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 4},
	{ID: "realesrgan-x4plus-anime", Label: "Anime (RealESRGAN x4)", Family: ModelFamilyRealESRGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 4},
	{ID: "realesr-animevideov3", Label: "Anime Video (RealESRGAN)", Family: ModelFamilyRealESRGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 4},
	{ID: "realesr-general-x4v3", Label: "General Fast (RealESRGAN)", Family: ModelFamilyRealESRGAN, SupportsDenoise: true, SupportsTTA: true, DefaultScale: 4},
	{ID: "realesrgan-x2plus", Label: "2x General (RealESRGAN)", Family: ModelFamilyRealESRGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 2},
	{ID: "realesrnet-x4plus", Label: "Clean Restore (RealESRGAN)", Family: ModelFamilyRealESRGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 4},
	// Real-CUGAN models
	{ID: "realcugan-pro", Label: "Pro (Real-CUGAN)", Family: ModelFamilyRealCUGAN, SupportsDenoise: true, SupportsTTA: true, DefaultScale: 2},
	{ID: "realcugan-se", Label: "Standard (Real-CUGAN)", Family: ModelFamilyRealCUGAN, SupportsDenoise: true, SupportsTTA: true, DefaultScale: 2},
	{ID: "realcugan-no-denoise", Label: "No Denoise (Real-CUGAN)", Family: ModelFamilyRealCUGAN, SupportsDenoise: false, SupportsTTA: true, DefaultScale: 2},
}

func ModelOptions() []string {
	labels := make([]string, len(modelCatalog))
	for i, m := range modelCatalog {
		labels[i] = m.Label
	}
	return labels
}

func ModelInfoFromID(id string) *ModelInfo {
	for _, m := range modelCatalog {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

func ModelLabelFromID(id string) string {
	if m := ModelInfoFromID(id); m != nil {
		return m.Label
	}
	return ""
}

func ModelIDFromLabel(label string) string {
	for _, m := range modelCatalog {
		if m.Label == label {
			return m.ID
		}
	}
	return "realesrgan-x4plus"
}

func DetectAIUpscaleBackend() string {
	if _, ok := utils.FindTool("realesrgan-ncnn-vulkan"); ok {
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

func DetectRealCUGANAvailable() bool {
	_, ok := utils.FindTool("realcugan-ncnn-vulkan")
	return ok
}

// PresetID uniquely identifies a one-click upscale workflow.
type PresetID string

const (
	PresetHobbySD   PresetID = "hobby-sd"   // 480p→1080p, clean up, light AI
	PresetHobbyHD   PresetID = "hobby-hd"   // 720p→1080p, balanced
	PresetSemiPro4K PresetID = "semi-pro-4k" // 1080p→4K, high quality
	PresetAnime     PresetID = "anime"      // Anime-specific model
	PresetRestoration PresetID = "restoration" // Old/scratchy footage repair
	PresetSocial    PresetID = "social"      // 720p→1080p + 30fps for social media
)

// PresetInfo describes a one-click workflow.
type PresetInfo struct {
	ID          PresetID
	Label       string
	Description string
	// AI model
	Model string
	// Scale target
	TargetRes string
	// Frame rate
	FrameRate string
	// Encoding
	VideoCodec string
	QualityPreset string
	CRF int
	// Optional RIFE
	EnableRIFE bool
	RIFEMultiplier int
}

var presetCatalog = []PresetInfo{
	{
		ID: PresetHobbySD, Label: "Hobbyist SD→HD (480p→1080p)",
		Description: "Clean up SD footage, light AI upscale. Good for home videos.",
		Model: "realesrgan-x4plus", TargetRes: "1080p (1920x1080)",
		FrameRate: "Source", VideoCodec: "H.264", QualityPreset: "Fast", CRF: 18,
		EnableRIFE: false,
	},
	{
		ID: PresetHobbyHD, Label: "Hobbyist HD→HD (720p→1080p)",
		Description: "Sharpen and standardise 720p to 1080p.",
		Model: "realesrgan-x4plus", TargetRes: "1080p (1920x1080)",
		FrameRate: "Source", VideoCodec: "H.264", QualityPreset: "Medium", CRF: 16,
		EnableRIFE: false,
	},
	{
		ID: PresetSemiPro4K, Label: "Semi-Pro 1080p→4K",
		Description: "High-quality upscale for professional-looking output.",
		Model: "realesr-general-x4v3", TargetRes: "4K (3840x2160)",
		FrameRate: "Source", VideoCodec: "H.265", QualityPreset: "Slow", CRF: 16,
		EnableRIFE: true, RIFEMultiplier: 2,
	},
	{
		ID: PresetAnime, Label: "Anime Optimised",
		Description: "Uses anime-specific model + RIFE for smooth playback.",
		Model: "realesrgan-x4plus-anime", TargetRes: "1080p (1920x1080)",
		FrameRate: "60", VideoCodec: "H.264", QualityPreset: "Medium", CRF: 18,
		EnableRIFE: true, RIFEMultiplier: 2,
	},
	{
		ID: PresetRestoration, Label: "Restoration (Old Footage)",
		Description: "Repair scratchy/old footage with denoise model.",
		Model: "realesr-general-x4v3", TargetRes: "Match Source",
		FrameRate: "Source", VideoCodec: "H.264", QualityPreset: "Slow", CRF: 16,
		EnableRIFE: false,
	},
	{
		ID: PresetSocial, Label: "Social Media Ready (→1080p 30fps)",
		Description: "Optimised for YouTube/TikTok/Instagram upload.",
		Model: "realesrgan-x4plus", TargetRes: "1080p (1920x1080)",
		FrameRate: "30", VideoCodec: "H.264", QualityPreset: "Fast", CRF: 18,
		EnableRIFE: true, RIFEMultiplier: 2,
	},
}

// PresetOptions returns the list of preset labels for the dropdown.
func PresetOptions() []string {
	labels := make([]string, len(presetCatalog))
	for i, p := range presetCatalog {
		labels[i] = p.Label
	}
	return labels
}

// PresetInfoFromLabel finds preset info by label.
func PresetInfoFromLabel(label string) *PresetInfo {
	for _, p := range presetCatalog {
		if p.Label == label {
			return &p
		}
	}
	return nil
}

// PresetIDs returns all preset IDs.
func PresetIDs() []PresetID {
	ids := make([]PresetID, len(presetCatalog))
	for i, p := range presetCatalog {
		ids[i] = p.ID
	}
	return ids
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

// DetectRIFEBackend returns "ncnn" if rife-ncnn-vulkan is found in PATH or the
// VideoTools app-local bin directory, otherwise "".
func DetectRIFEBackend() string {
	if _, ok := utils.FindTool("rife-ncnn-vulkan"); ok {
		return "ncnn"
	}
	return ""
}

// RIFEModelOptions returns the list of supported rife-ncnn-vulkan model names.
func RIFEModelOptions() []string {
	return []string{
		"rife-v4.6",
		"rife-v4.13-lite",
		"rife-anime",
	}
}
