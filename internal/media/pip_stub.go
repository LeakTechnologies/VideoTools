//go:build native_media && !windows

package media

import (
	"fyne.io/fyne/v2"
)

func ApplyPiPExclude(win fyne.Window, enable bool) error {
	return nil
}
