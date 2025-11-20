# VideoTools Prototype

## Requirements
- Go 1.21+
- Fyne 2.x (pulled automatically via `go mod tidy`)
- FFmpeg (not yet invoked, but required for future transcoding)

## Running
Launch the GUI:
```bash
go run .
```

Run a module via CLI:
```bash
go run . convert input.avi output.mp4
go run . combine file1.mov file2.wav / final.mp4
go run . logs
```

Add `-debug` or `VIDEOTOOLS_DEBUG=1` for verbose stderr logs.

## Logs
- All actions log to `videotools.log` (override with `VIDEOTOOLS_LOG_FILE=/path/to/log`).
- CLI command `videotools logs` (or `go run . logs`) prints the last 200 lines.
- Each entry is tagged (e.g. `[UI]`, `[CLI]`, `[FFMPEG]`) so issues are easy to trace.

## Notes
- GUI requires a running display server (X11/Wayland). In headless shells it will log `[UI] DISPLAY environment variable is empty`.
- Convert screen accepts drag-and-drop or the "Open File…" button; ffprobe metadata populates instantly, the preview box animates extracted frames with simple play/pause + slider controls (and lets you grab cover art), and the "Generate Snippet" button produces a 20-second midpoint clip for quick quality checks (requires ffmpeg in `PATH`).
- Simple mode now applies smart inverse telecine by default—automatically skipping it on progressive footage—and lets you rename the target file before launching a convert job.
- Other module handlers are placeholders; hook them to actual FFmpeg calls next.
