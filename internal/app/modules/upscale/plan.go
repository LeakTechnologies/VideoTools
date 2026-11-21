package upscale

import (
	"fmt"
	"math"
	"strings"
)

// ResolveColorMatrix returns a valid FFmpeg colorspace enum for the source
// matrix. Explicit user selection wins; a probed tag comes next; height is a
// last-resort fallback only when neither is usable.
//
// 525-line SD uses smpte170m; 625-line SD uses bt470bg. They are not
// interchangeable, and "bt601" is not a valid colorspace enum.
func ResolveColorMatrix(userSetting, probed string, height int) string {
	user := strings.ToLower(strings.TrimSpace(userSetting))
	if user != "" && user != "auto" {
		return matrixFromExplicit(user, height)
	}

	switch strings.ToLower(strings.TrimSpace(probed)) {
	case "bt709", "rec709":
		return "bt709"
	case "bt2020", "rec2020":
		return "bt2020"
	case "smpte170m", "bt470bg", "smpte240m":
		return strings.ToLower(strings.TrimSpace(probed))
	case "bt601", "rec601", "sd":
		return sdMatrix(height)
	default:
		return fallbackMatrix(height)
	}
}

func matrixFromExplicit(value string, height int) string {
	switch value {
	case "bt601", "rec601":
		return sdMatrix(height)
	case "bt709", "rec709":
		return "bt709"
	case "bt2020", "rec2020":
		return "bt2020"
	case "smpte170m", "bt470bg", "smpte240m":
		return value
	default:
		return fallbackMatrix(height)
	}
}

func sdMatrix(height int) string {
	if height >= 540 {
		return "bt470bg"
	}
	return "smpte170m"
}

func fallbackMatrix(height int) string {
	if height <= 0 {
		// Unknown height: do not guess SD; leave the stream in the working space.
		return "bt709"
	}
	if height > 576 {
		return "bt709"
	}
	return sdMatrix(height)
}

// NativeScale returns the native integer scale for a model id. Unknown models
// default to 4x, the common Real-ESRGAN native scale.
func NativeScale(model string) int {
	if info := ModelInfoFromID(model); info != nil && info.DefaultScale > 0 {
		return info.DefaultScale
	}
	return 4
}

// ScalePlan is the resolved AI scale and output size for one job.
type ScalePlan struct {
	ModelScale  int
	FinalWidth  int
	FinalHeight int
	Resize      bool
	Note        string
}

// PlanAIScale separates the model's native integer scale from the final output
// size. The ncnn models are trained at a fixed scale (4x for x4plus, 2x for
// x2plus/CUGAN); feeding them a fractional -s silently truncates and can corrupt
// the image.
//
// The final size comes from the target preset when one is set, otherwise from
// the explicit AI scale. A 1x request is routed to the model's native scale
// rather than passed through as 1x.
func PlanAIScale(model string, sourceW, sourceH, targetW, targetH int, targetPreset string, aiScale float64, useTarget bool, outputAdjust float64) (ScalePlan, error) {
	if sourceW <= 0 || sourceH <= 0 {
		return ScalePlan{}, fmt.Errorf("AI upscale requires known source dimensions")
	}
	if outputAdjust <= 0 {
		outputAdjust = 1.0
	}
	if math.Abs(outputAdjust-1.0) > 1e-9 {
		return ScalePlan{}, fmt.Errorf("AI Output Adjust is not supported with native-scale upscaling yet; set it to 1.0")
	}

	native := NativeScale(model)
	plan := ScalePlan{ModelScale: native}

	targetIsSet := targetPreset != "" && targetPreset != "Match Source" && targetW > 0 && targetH > 0
	if targetIsSet {
		plan.FinalWidth = makeEven(targetW)
		plan.FinalHeight = makeEven(targetH)
	} else {
		desired := aiScale
		if desired <= 0 {
			desired = float64(native)
		}
		if useTarget {
			plan.Note = fmt.Sprintf("Match Target with Match Source resolves to 1x; using native %dx for %s", native, model)
			desired = float64(native)
		} else if desired <= 1.0 {
			plan.Note = fmt.Sprintf("1x is not supported for %s; using native %dx", model, native)
			desired = float64(native)
		}
		plan.FinalWidth = makeEven(int(math.Round(float64(sourceW) * desired)))
		plan.FinalHeight = makeEven(int(math.Round(float64(sourceH) * desired)))
	}

	if plan.FinalWidth <= 0 || plan.FinalHeight <= 0 {
		return ScalePlan{}, fmt.Errorf("AI upscale resolved invalid output dimensions")
	}

	nativeW := makeEven(sourceW * native)
	nativeH := makeEven(sourceH * native)
	plan.Resize = plan.FinalWidth != nativeW || plan.FinalHeight != nativeH
	return plan, nil
}

func makeEven(v int) int {
	if v < 2 {
		return 2
	}
	if v%2 != 0 {
		return v + 1
	}
	return v
}
