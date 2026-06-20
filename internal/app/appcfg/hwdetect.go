//go:build native_media

package appcfg

import "github.com/LeakTechnologies/VideoTools/internal/media"

func DetectHWDeviceType() int {
	if !media.HWDecodeEnabled() {
		return 0
	}
	switch media.DetectHWDevice() {
	case media.HWDeviceVAAPI:
		return 1
	case media.HWDeviceD3D11VA:
		return 2
	case media.HWDeviceQSV:
		return 3
	default:
		return 0
	}
}