package disc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/iso9660"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
)

// scanISO reads only the IFO files from a DVD ISO. The full VIDEO_TS extraction
// path remains in the rip service for actual ripping.
func scanISO(ctx context.Context, isoPath string, opts ScanOptions) (*ScanResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fi, err := os.Stat(isoPath)
	if err != nil {
		return nil, serviceError("source_not_found", "ISO not found", err)
	}
	if format, _ := udf.IdentifyDiscFormat(isoPath); format == udf.DiscTypeBluRay {
		return nil, partialScanError("unsupported_bluray", "Blu-ray scanning is not implemented in this service yet")
	}

	f, err := os.Open(isoPath)
	if err != nil {
		return nil, serviceError("iso_open", "could not open ISO", err)
	}
	defer f.Close()

	readFile, err := isoReadFile(f)
	if err != nil {
		return nil, err
	}
	vmg, err := readFile("VIDEO_TS/VIDEO_TS.IFO")
	if err != nil {
		return nil, serviceError("iso_no_video_ts", "could not read VIDEO_TS.IFO from ISO", err)
	}
	tsps, err := parseTitleList(vmg)
	if err != nil {
		return nil, serviceError("read_title_list", "could not parse VIDEO_TS.IFO from ISO", err)
	}

	tempDir, err := os.MkdirTemp("", "vt-disc-scan-*")
	if err != nil {
		return nil, serviceError("temp_dir", "could not create scan temp dir", err)
	}
	defer os.RemoveAll(tempDir)

	videoTS := filepath.Join(tempDir, "VIDEO_TS")
	if err := os.MkdirAll(videoTS, 0755); err != nil {
		return nil, serviceError("temp_dir", "could not create scan VIDEO_TS dir", err)
	}
	if err := os.WriteFile(filepath.Join(videoTS, "VIDEO_TS.IFO"), vmg, 0644); err != nil {
		return nil, serviceError("temp_write", "could not write temporary VIDEO_TS.IFO", err)
	}

	vtsNumbers := map[int]bool{}
	for _, t := range tsps {
		vtsNumbers[int(t.VTSNumber)] = true
	}
	var warnings []string
	for vts := range vtsNumbers {
		name := fmt.Sprintf("VTS_%02d_0.IFO", vts)
		data, err := readFile("VIDEO_TS/" + name)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s could not be read from ISO: %v", name, err))
			continue
		}
		_ = os.WriteFile(filepath.Join(videoTS, name), data, 0644)
	}

	result, err := scanVideoTS(videoTS, SourceISO, isoPath, opts)
	if err != nil {
		return nil, err
	}
	result.DiscType = classifyDiscType(fi.Size())
	if result.DiscType == "" {
		result.DiscType = "DVD"
	}
	result.TotalSize = fi.Size()
	result.Warnings = append(result.Warnings, warnings...)
	return result, nil
}

func isoReadFile(f *os.File) (func(string) ([]byte, error), error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, serviceError("iso_seek", "could not read ISO", err)
	}
	udfReader := udf.NewReader(f)
	read := udfReader.ReadFileData
	if _, err := read("VIDEO_TS/VIDEO_TS.IFO"); err == nil {
		return read, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, serviceError("iso_seek", "could not rewind ISO for ISO 9660 fallback", err)
	}
	isoReader := iso9660.NewReader(f)
	return isoReader.ReadFileData, nil
}

func parseTitleList(vmg []byte) ([]ifo.TitleSearchPointer, error) {
	temp, err := os.CreateTemp("", "vt-disc-vmg-*.ifo")
	if err != nil {
		return nil, err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err := temp.Write(vmg); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	return ifo.ReadTitleList(name)
}
