package ui

import "errors"

// ErrAutomaticPreviewsDisabled is a policy result, not a media-open failure.
var ErrAutomaticPreviewsDisabled = errors.New("automatic media previews are disabled")
