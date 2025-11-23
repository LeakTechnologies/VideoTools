package modules

import (
	"fmt"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Module handlers - each handles the logic for a specific module

// HandleConvert handles the convert module
func HandleConvert(files []string) {
	logging.Debug(logging.CatFFMPEG, "convert handler invoked with %v", files)
	fmt.Println("convert", files)
}

// HandleMerge handles the merge module
func HandleMerge(files []string) {
	logging.Debug(logging.CatFFMPEG, "merge handler invoked with %v", files)
	fmt.Println("merge", files)
}

// HandleTrim handles the trim module
func HandleTrim(files []string) {
	logging.Debug(logging.CatModule, "trim handler invoked with %v", files)
	fmt.Println("trim", files)
}

// HandleFilters handles the filters module
func HandleFilters(files []string) {
	logging.Debug(logging.CatModule, "filters handler invoked with %v", files)
	fmt.Println("filters", files)
}

// HandleUpscale handles the upscale module
func HandleUpscale(files []string) {
	logging.Debug(logging.CatModule, "upscale handler invoked with %v", files)
	fmt.Println("upscale", files)
}

// HandleAudio handles the audio module
func HandleAudio(files []string) {
	logging.Debug(logging.CatModule, "audio handler invoked with %v", files)
	fmt.Println("audio", files)
}

// HandleThumb handles the thumb module
func HandleThumb(files []string) {
	logging.Debug(logging.CatModule, "thumb handler invoked with %v", files)
	fmt.Println("thumb", files)
}

// HandleInspect handles the inspect module
func HandleInspect(files []string) {
	logging.Debug(logging.CatModule, "inspect handler invoked with %v", files)
	fmt.Println("inspect", files)
}
