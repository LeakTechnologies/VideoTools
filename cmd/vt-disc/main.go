package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/LeakTechnologies/VideoTools/internal/disc"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "scan" {
		fmt.Fprintln(os.Stderr, "usage: vt-disc scan --source <dvd-folder|iso|video-ts> [--json]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("source", "", "DVD folder, ISO, VIDEO_TS directory or file")
	jsonOut := fs.Bool("json", true, "emit JSON")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *source == "" {
		fmt.Fprintln(os.Stderr, "vt-disc scan: --source is required")
		os.Exit(2)
	}

	result, err := disc.ScanSource(context.Background(), *source, disc.ScanOptions{
		OnNote: func(note string) { fmt.Fprintln(os.Stderr, note) },
	})
	if err != nil {
		writeError(err)
		os.Exit(1)
	}
	if *jsonOut {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			writeError(err)
			os.Exit(1)
		}
		return
	}
	fmt.Printf("%s: %d titles, %s, %s\n", result.DiscType, len(result.Titles), result.Region, result.VideoStandard)
}

func writeError(err error) {
	payload := map[string]any{
		"error": map[string]string{
			"code":    "scan_failed",
			"message": err.Error(),
		},
	}
	if typed, ok := err.(*disc.Error); ok {
		payload["error"] = map[string]string{"code": typed.Code, "message": typed.Message}
	}
	_ = json.NewEncoder(os.Stderr).Encode(payload)
}
