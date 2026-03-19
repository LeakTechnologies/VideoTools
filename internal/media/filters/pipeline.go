//go:build native_media

package filters

import (
	"fmt"
	"strings"
)

type FilterType string

const (
	FilterDeinterlace FilterType = "deinterlace"
	FilterScale       FilterType = "scale"
	FilterColor       FilterType = "color"
	FilterDenoise     FilterType = "denoise"
	FilterSharpen     FilterType = "sharpen"
	FilterCrop        FilterType = "crop"
	FilterRotate      FilterType = "rotate"
)

type FilterConfig struct {
	Type   FilterType
	Params map[string]interface{}
	Enable bool
}

type FilterPipeline struct {
	filters   []FilterConfig
	graph     string
	generated bool
}

func NewFilterPipeline() *FilterPipeline {
	return &FilterPipeline{
		filters:   make([]FilterConfig, 0),
		generated: false,
	}
}

func (p *FilterPipeline) Add(f FilterConfig) *FilterPipeline {
	p.filters = append(p.filters, f)
	p.generated = false
	return p
}

func (p *FilterPipeline) Enable(t FilterType, enabled bool) *FilterPipeline {
	for i := range p.filters {
		if p.filters[i].Type == t {
			p.filters[i].Enable = enabled
		}
	}
	p.generated = false
	return p
}

func (p *FilterPipeline) Remove(t FilterType) *FilterPipeline {
	filtered := make([]FilterConfig, 0)
	for _, f := range p.filters {
		if f.Type != t {
			filtered = append(filtered, f)
		}
	}
	p.filters = filtered
	p.generated = false
	return p
}

func (p *FilterPipeline) Clear() *FilterPipeline {
	p.filters = p.filters[:0]
	p.graph = ""
	p.generated = false
	return p
}

func (p *FilterPipeline) Generate() (string, error) {
	if p.generated {
		return p.graph, nil
	}

	var parts []string

	for _, f := range p.filters {
		if !f.Enable {
			continue
		}

		filter, err := p.buildFilter(f)
		if err != nil {
			return "", err
		}

		if filter != "" {
			parts = append(parts, filter)
		}
	}

	p.graph = strings.Join(parts, ",")
	p.generated = true

	return p.graph, nil
}

func (p *FilterPipeline) buildFilter(f FilterConfig) (string, error) {
	switch f.Type {
	case FilterDeinterlace:
		return p.buildDeinterlace(f)
	case FilterScale:
		return p.buildScale(f)
	case FilterColor:
		return p.buildColor(f)
	case FilterDenoise:
		return p.buildDenoise(f)
	case FilterSharpen:
		return p.buildSharpen(f)
	case FilterCrop:
		return p.buildCrop(f)
	case FilterRotate:
		return p.buildRotate(f)
	default:
		return "", fmt.Errorf("unknown filter type: %s", f.Type)
	}
}

func (p *FilterPipeline) buildDeinterlace(f FilterConfig) (string, error) {
	mode := getString(f.Params, "mode", "1")
	parity := getString(f.Params, "parity", "-1")
	field := getString(f.Params, "field", "auto")

	return fmt.Sprintf("yadif=mode=%s:parity=%s:field=%s", mode, parity, field), nil
}

func (p *FilterPipeline) buildScale(f FilterConfig) (string, error) {
	width := getInt(f.Params, "width", -1)
	height := getInt(f.Params, "height", -1)
	flags := getString(f.Params, "flags", "lanczos")

	if width == -1 && height == -1 {
		return "", nil
	}

	widthStr := fmt.Sprintf("%d", width)
	heightStr := fmt.Sprintf("%d", height)

	if width == -1 {
		widthStr = "-1"
	}
	if height == -1 {
		heightStr = "-1"
	}

	return fmt.Sprintf("scale=%s:%s:flags=%s", widthStr, heightStr, flags), nil
}

func (p *FilterPipeline) buildColor(f FilterConfig) (string, error) {
	brightness := getFloat(f.Params, "brightness", 0.0)
	saturation := getFloat(f.Params, "saturation", 1.0)
	contrast := getFloat(f.Params, "contrast", 1.0)
	gamma := getFloat(f.Params, "gamma", 1.0)

	return fmt.Sprintf("eq=brightness=%.2f:saturation=%.2f:contrast=%.2f:gamma=%.2f",
		brightness, saturation, contrast, gamma), nil
}

func (p *FilterPipeline) buildDenoise(f FilterConfig) (string, error) {
	spatial := getInt(f.Params, "spatial", 4)
	temporal := getInt(f.Params, "temporal", 4)
	env := getString(f.Params, "env", "s")

	return fmt.Sprintf("hqdn3d=%d:%d:%s", spatial, temporal, env), nil
}

func (p *FilterPipeline) buildSharpen(f FilterConfig) (string, error) {
	luma := getFloat(f.Params, "luma", 1.0)
	chroma := getFloat(f.Params, "chroma", 1.0)

	return fmt.Sprintf("unsharp=5:5:%.1f:5:5:%.1f", luma, chroma), nil
}

func (p *FilterPipeline) buildCrop(f FilterConfig) (string, error) {
	width := getInt(f.Params, "width", 0)
	height := getInt(f.Params, "height", 0)
	x := getInt(f.Params, "x", -1)
	y := getInt(f.Params, "y", -1)

	if width == 0 || height == 0 {
		return "", fmt.Errorf("crop requires width and height")
	}

	return fmt.Sprintf("crop=%d:%d:%d:%d", width, height, x, y), nil
}

func (p *FilterPipeline) buildRotate(f FilterConfig) (string, error) {
	angle := getFloat(f.Params, "angle", 0)

	return fmt.Sprintf("rotate=%.2f", angle), nil
}

func (p *FilterPipeline) Filters() []FilterConfig {
	return p.filters
}

func (p *FilterPipeline) Count() int {
	count := 0
	for _, f := range p.filters {
		if f.Enable {
			count++
		}
	}
	return count
}

func getString(params map[string]interface{}, key, defaultVal string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return defaultVal
}

func getInt(params map[string]interface{}, key string, defaultVal int) int {
	if v, ok := params[key].(int); ok {
		return v
	}
	if v, ok := params[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

func getFloat(params map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := params[key].(float64); ok {
		return v
	}
	if v, ok := params[key].(int); ok {
		return float64(v)
	}
	return defaultVal
}

type Preset string

const (
	PresetNone         Preset = ""
	PresetVintage      Preset = "vintage"
	PresetWarm         Preset = "warm"
	PresetCool         Preset = "cool"
	PresetHighContrast Preset = "high_contrast"
	PresetSoft         Preset = "soft"
	PresetVivid        Preset = "vivid"
)

func (p Preset) Apply(pipeline *FilterPipeline) *FilterPipeline {
	switch p {
	case PresetNone:
		pipeline.Clear()
	case PresetVintage:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"brightness": 0.05, "saturation": 0.8, "contrast": 1.1},
			Enable: true,
		})
	case PresetWarm:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"saturation": 1.1, "brightness": 0.02},
			Enable: true,
		})
	case PresetCool:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"saturation": 1.05, "brightness": -0.02},
			Enable: true,
		})
	case PresetHighContrast:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"contrast": 1.3, "brightness": 0.02},
			Enable: true,
		})
	case PresetSoft:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"contrast": 0.9, "saturation": 0.9},
			Enable: true,
		})
	case PresetVivid:
		pipeline.Clear()
		pipeline.Add(FilterConfig{
			Type:   FilterColor,
			Params: map[string]interface{}{"saturation": 1.4, "contrast": 1.1},
			Enable: true,
		})
	}
	return pipeline
}
