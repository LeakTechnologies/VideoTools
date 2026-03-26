package filters

import "fmt"

// FilterChainParams holds the stylistic filter state values needed to build a filter chain.
type FilterChainParams struct {
	StylisticMode string
	Scanlines     bool
	ChromaNoise   float64
	ColorBleeding bool
	TapeNoise     float64
	TrackingError float64
	Dropout       float64
	Interlacing   string
}

// BuildStylisticFilterChain creates FFmpeg filter chains for decade-based stylistic effects.
func BuildStylisticFilterChain(p FilterChainParams) []string {
	var chain []string

	switch p.StylisticMode {
	case "8mm Film":
		chain = append(chain, "eq=contrast=1.0:saturation=0.9:brightness=0.02")
		chain = append(chain, "unsharp=6:6:0.2:6:6:0.2")
		chain = append(chain, "scale=iw*0.8:ih*0.8:flags=lanczos")
		chain = append(chain, "fftnorm=nor=0.08:Links=0")
		if p.TapeNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.TapeNoise*0.1))
		}
		if p.TrackingError > 0 {
			chain = append(chain, fmt.Sprintf("crop='iw-mod(iw*%f/200,1)':'ih-mod(ih*%f/200,1)':%f:%f",
				p.TrackingError, p.TrackingError*0.5, p.TrackingError*2, p.TrackingError))
		}

	case "16mm Film":
		chain = append(chain, "eq=contrast=1.05:saturation=1.0:brightness=0.0")
		chain = append(chain, "unsharp=5:5:0.4:5:5:0.4")
		chain = append(chain, "scale=iw*0.9:ih*0.9:flags=lanczos")
		chain = append(chain, "fftnorm=nor=0.06:Links=0")
		if p.TapeNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.TapeNoise*0.08))
		}
		if p.Dropout > 0 {
			scratches := int(p.Dropout * 5)
			if scratches > 0 {
				chain = append(chain, "geq=lum=lum:cb=cb:cr=cr,boxblur=1:1:cr=0:ar=1")
			}
		}

	case "B&W Film":
		chain = append(chain, "colorchannelmixer=.299:.587:.114:0:.299:.587:.114:0:.299:.587:.114")
		chain = append(chain, "eq=contrast=1.1:brightness=-0.02")
		chain = append(chain, "unsharp=4:4:0.3:4:4:0.3")
		chain = append(chain, "fftnorm=nor=0.05:Links=0")
		if p.ColorBleeding {
			chain = append(chain, "unsharp=7:7:0.8:7:7:0.8")
		}

	case "Silent Film":
		chain = append(chain, "framerate=18")
		chain = append(chain, "colorchannelmixer=.393:.769:.189:0:.393:.769:.189:0:.393:.769:.189")
		chain = append(chain, "eq=contrast=1.15:brightness=0.05")
		chain = append(chain, "unsharp=8:8:0.1:8:8:0.1")
		chain = append(chain, "fftnorm=nor=0.12:Links=0")
		if p.TrackingError > 0 {
			chain = append(chain, fmt.Sprintf("crop='iw-mod(iw*%f/100,2)':'ih-mod(ih*%f/100,2)':%f:%f",
				p.TrackingError*3, p.TrackingError*1.5, p.TrackingError*5, p.TrackingError*2))
		}

	case "70s":
		chain = append(chain, "eq=contrast=0.95:saturation=0.85:brightness=0.05")
		chain = append(chain, "unsharp=5:5:0.3:5:5:0.3")
		chain = append(chain, "fftnorm=nor=0.15:Links=0")
		if p.ChromaNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.ChromaNoise*0.2))
		}

	case "80s":
		chain = append(chain, "eq=contrast=1.1:saturation=1.2:brightness=0.02")
		chain = append(chain, "unsharp=3:3:0.4:3:3:0.4")
		chain = append(chain, "fftnorm=nor=0.2:Links=0")
		if p.ColorBleeding {
			chain = append(chain, "format=yuv420p,scale=iw+2:ih+2:flags=neighbor,crop=iw:ih")
		}
		if p.ChromaNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.ChromaNoise*0.3))
		}

	case "90s":
		chain = append(chain, "eq=contrast=1.05:saturation=1.1:brightness=0.0")
		chain = append(chain, "unsharp=3:3:0.5:3:3:0.5")
		chain = append(chain, "fftnorm=nor=0.1:Links=0")
		if p.TapeNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.TapeNoise*0.15))
		}

	case "VHS":
		chain = append(chain, "eq=contrast=1.08:saturation=1.15:brightness=0.03")
		chain = append(chain, "unsharp=4:4:0.4:4:4:0.4")
		chain = append(chain, "fftnorm=nor=0.18:Links=0")
		if p.ColorBleeding {
			chain = append(chain, "format=yuv420p,scale=iw+4:ih+4:flags=neighbor,crop=iw:ih")
		}
		if p.TrackingError > 0 {
			errorLevel := p.TrackingError * 2.0
			chain = append(chain, fmt.Sprintf("crop='iw-mod(iw*%f/100,2)':'ih-mod(ih*%f/100,2)':%f:%f",
				errorLevel, errorLevel/2, errorLevel/2, errorLevel/4))
		}
		if p.Dropout > 0 {
			dropoutLevel := int(p.Dropout * 20)
			if dropoutLevel > 0 {
				chain = append(chain, fmt.Sprintf("geq=lum=lum:cb=cb:cr=cr,sendcmd=f=%d:'drawbox w=iw h=2 y=%f:color=black@1:t=fill',drawbox w=iw h=2 y=%f:color=black@1:t=fill'",
					dropoutLevel, 100.0, 200.0))
			}
		}

	case "Webcam":
		chain = append(chain, "eq=contrast=1.15:saturation=0.9:brightness=-0.05")
		chain = append(chain, "scale=640:480:flags=neighbor")
		chain = append(chain, "unsharp=2:2:0.8:2:2:0.8")
		chain = append(chain, "fftnorm=nor=0.25:Links=0")
		if p.ChromaNoise > 0 {
			chain = append(chain, fmt.Sprintf("fftnorm=nor=%.2f:Links=0", p.ChromaNoise*0.4))
		}
	}

	if p.Scanlines {
		chain = append(chain, "format=yuv420p,scale=ih*2/3:ih:flags=neighbor,setsar=1,scale=ih*3/2:ih")
	}

	switch p.Interlacing {
	case "Interlaced":
		chain = append(chain, "interlace=scan=tff:lowpass=1")
	case "Progressive":
		chain = append(chain, "yadif=0:-1:0")
	}

	return chain
}
