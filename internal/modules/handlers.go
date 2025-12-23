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

// HandleAuthor handles the disc authoring module (DVD/Blu-ray) (placeholder)
func HandleAuthor(files []string) {
	logging.Debug(logging.CatModule, "author handler invoked with %v", files)
	// This will be handled by the UI drag-and-drop system
	// File loading is managed in buildAuthorView()
}

// HandleSubtitles handles the subtitles module (placeholder)
func HandleSubtitles(files []string) {
	logging.Debug(logging.CatModule, "subtitles handler invoked with %v", files)
	fmt.Println("subtitles", files)
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

// HandleCompare handles the compare module (side-by-side comparison of two videos)
func HandleCompare(files []string) {
	logging.Debug(logging.CatModule, "compare handler invoked with %v", files)
	fmt.Println("compare", files)
}

// HandlePlayer handles the player module
func HandlePlayer(files []string) {
	logging.Debug(logging.CatModule, "player handler invoked with %v", files)
	fmt.Println("player", files)
}
