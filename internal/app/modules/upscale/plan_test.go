package upscale

import "testing"

func TestResolveColorMatrix(t *testing.T) {
	cases := []struct {
		name   string
		user   string
		probed string
		height int
		want   string
	}{
		{"unknown 480", "", "", 480, "smpte170m"},
		{"unknown 539", "", "", 539, "smpte170m"},
		{"unknown 540", "", "", 540, "bt470bg"},
		{"unknown 576", "", "", 576, "bt470bg"},
		{"unknown 1080", "", "", 1080, "bt709"},
		{"probed 525", "auto", "smpte170m", 576, "smpte170m"},
		{"probed 625 beats height", "auto", "bt470bg", 480, "bt470bg"},
		{"unrecognised 576", "auto", "weird", 576, "bt470bg"},
		{"unrecognised 480", "auto", "weird", 480, "smpte170m"},
		{"explicit 601 576", "bt601", "", 576, "bt470bg"},
		{"explicit 601 480", "bt601", "", 480, "smpte170m"},
		{"explicit 709 wins", "bt709", "smpte170m", 576, "bt709"},
		{"explicit 2020", "bt2020", "", 1080, "bt2020"},
		{"rec709 alias", "", "rec709", 0, "bt709"},
		{"unknown height", "", "", 0, "bt709"},
		{"smpte240m", "smpte240m", "", 0, "smpte240m"},
		{"probed 2020", "auto", "bt2020", 1080, "bt2020"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveColorMatrix(tc.user, tc.probed, tc.height); got != tc.want {
				t.Fatalf("ResolveColorMatrix(%q, %q, %d) = %q, want %q", tc.user, tc.probed, tc.height, got, tc.want)
			}
		})
	}
}

func TestNativeScale(t *testing.T) {
	cases := map[string]int{
		"realesrgan-x4plus":       4,
		"realesrgan-x4plus-anime": 4,
		"realesrgan-x2plus":       2,
		"realcugan-pro":           2,
		"unknown-model":           4,
	}
	for model, want := range cases {
		if got := NativeScale(model); got != want {
			t.Fatalf("NativeScale(%q) = %d, want %d", model, got, want)
		}
	}
}

func TestPlanAIScale(t *testing.T) {
	const (
		preset1080 = "1080p (1920x1080)"
		preset2X   = "2X (relative)"
		matchSrc   = "Match Source"
	)
	cases := []struct {
		name       string
		model      string
		srcW, srcH int
		tgtW, tgtH int
		preset     string
		aiScale    float64
		useTarget  bool
		adjust     float64
		wantScale  int
		wantW      int
		wantH      int
		wantResize bool
		wantNote   bool
		wantErr    bool
	}{
		{
			name: "576 to 1080", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 1920, wantH: 1080, wantResize: true,
		},
		{
			name: "480 to 1080", model: "realesrgan-x4plus",
			srcW: 720, srcH: 480, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 1920, wantH: 1080, wantResize: true,
		},
		{
			name: "match source with match target routes to native", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, preset: matchSrc,
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 2880, wantH: 2304, wantResize: false, wantNote: true,
		},
		{
			name: "explicit 2x", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, preset: matchSrc,
			aiScale: 2, useTarget: false, adjust: 1,
			wantScale: 4, wantW: 1440, wantH: 1152, wantResize: true,
		},
		{
			name: "explicit 1x routes to native", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, preset: matchSrc,
			aiScale: 1, useTarget: false, adjust: 1,
			wantScale: 4, wantW: 2880, wantH: 2304, wantResize: false, wantNote: true,
		},
		{
			name: "2x relative target", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, tgtW: 1440, tgtH: 1152, preset: preset2X,
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 1440, wantH: 1152, wantResize: true,
		},
		{
			name: "4x relative target matches native", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, tgtW: 2880, tgtH: 2304, preset: "4X (relative)",
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 2880, wantH: 2304, wantResize: false,
		},
		{
			name: "explicit 8x", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, preset: matchSrc,
			aiScale: 8, useTarget: false, adjust: 1,
			wantScale: 4, wantW: 5760, wantH: 4608, wantResize: true,
		},
		{
			name: "2x native model to 1080", model: "realesrgan-x2plus",
			srcW: 720, srcH: 576, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 2, useTarget: true, adjust: 1,
			wantScale: 2, wantW: 1920, wantH: 1080, wantResize: true,
		},
		{
			name: "odd source evened", model: "realesrgan-x4plus",
			srcW: 721, srcH: 577, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 4, useTarget: true, adjust: 1,
			wantScale: 4, wantW: 1920, wantH: 1080, wantResize: true,
		},
		{
			name: "adjust unsupported", model: "realesrgan-x4plus",
			srcW: 720, srcH: 576, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 4, useTarget: true, adjust: 0.5,
			wantErr: true,
		},
		{
			name: "unknown source dimensions", model: "realesrgan-x4plus",
			srcW: 0, srcH: 0, tgtW: 1920, tgtH: 1080, preset: preset1080,
			aiScale: 4, useTarget: true, adjust: 1,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanAIScale(tc.model, tc.srcW, tc.srcH, tc.tgtW, tc.tgtH, tc.preset, tc.aiScale, tc.useTarget, tc.adjust)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", plan)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.ModelScale != tc.wantScale || plan.FinalWidth != tc.wantW || plan.FinalHeight != tc.wantH || plan.Resize != tc.wantResize {
				t.Fatalf("plan = %+v, want scale=%d size=%dx%d resize=%v", plan, tc.wantScale, tc.wantW, tc.wantH, tc.wantResize)
			}
			if tc.wantNote && plan.Note == "" {
				t.Fatalf("expected a routing note, got none")
			}
			if !tc.wantNote && plan.Note != "" {
				t.Fatalf("unexpected note: %q", plan.Note)
			}
		})
	}
}
