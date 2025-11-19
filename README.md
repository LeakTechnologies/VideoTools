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
- Module handlers are placeholders; hook them to actual FFmpeg calls next.
