package modules

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
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

// HandleRip handles the rip module (placeholder)
func HandleRip(files []string) {
	logging.Debug(logging.CatModule, "rip handler invoked with %v", files)
	fmt.Println("rip", files)
}

// HandleBluRay handles the Blu-Ray authoring module (placeholder)
func HandleBluRay(files []string) {
	logging.Debug(logging.CatModule, "bluray handler invoked with %v", files)
	fmt.Println("bluray", files)
}

// HandleSubtitles handles the subtitles module (placeholder)
func HandleSubtitles(files []string) {
	logging.Debug(logging.CatModule, "subtitles handler invoked with %v", files)
	fmt.Println("subtitles", files)
}

// HandleThumbnail handles the thumbnail module
func HandleThumbnail(files []string) {
	logging.Debug(logging.CatModule, "thumbnail handler invoked with %v", files)
	fmt.Println("thumbnail", files)
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

func HandleEnhance(files []string) {
	// Enhancement module not ready yet - show placeholder
	logging.Debug(logging.CatModule, "enhance handler invoked with %v", files)
	fmt.Println("enhance", files)

	if len(files) > 0 {
		dialog.ShowInformation("Enhancement", "Opening multiple files not supported yet. Select single video for enhancement.", fyne.CurrentApp().Driver().AllWindows()[0])
		return
	}

	if len(files) == 1 {
		// Show coming soon message
		dialog.ShowInformation("Enhancement",
			fmt.Sprintf("Enhancement module coming soon!\n\nSelected file: %s\n\nThis feature will be available in a future update.", files[0]),
			fyne.CurrentApp().Driver().AllWindows()[0])
	}
}
