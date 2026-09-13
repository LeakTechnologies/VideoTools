package ifo

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// TitleInfo holds the extracted chapter/track information from a VTS IFO file.
// All data is read from the IFO at open time; no VOB access is required.
type TitleInfo struct {
	// Chapters holds the start time of each chapter in seconds.
	// The first entry is always 0.0. Empty if the IFO has no PGC data.
	Chapters []float64

	// Duration is the total title playback time in seconds from the PGC header.
	Duration float64

	// IsNTSC is true when the frame rate is 29.97fps (NTSC), false for 25fps (PAL).
	IsNTSC bool

	// Audio describes each audio track in IFO order (index 0 = first stream).
	Audio []TrackInfo

	// Subtitles describes each subpicture track in IFO order.
	Subtitles []TrackInfo

	// HasAngles is true when the IFO contains cells with angle-block mode set,
	// indicating multi-angle content.
	HasAngles bool

	// Interlaced is true when the VTS video attributes indicate video-originated
	// (camera) content (FilmMode == 0). Film-originated content (FilmMode == 1)
	// is progressive. This drives the deinterlace decision in the Rip module.
	Interlaced bool

	// Cells holds the per-cell sector extents for the title's PGC in playback
	// order. Each cell maps to a VOB-relative byte range within its VOB file,
	// enabling cell-accurate extraction when the VOB concat fallback is used on
	// multi-title VTS discs (e.g. scene-segmented compilations where multiple
	// titles share one VOB set). Empty when cell data is unavailable.
	Cells []TitleCell
}

// TitleCell holds the per-cell byte-range metadata for one cell in a PGC.
// FirstSector/LastSector are VOB-relative sector numbers (byte offset = sector * 2048)
// within the VOB file identified by VOBID. Multiple cells in playback order produce
// the full title when their byte ranges are concatenated in order.
type TitleCell struct {
	VOBID       uint8  // VOB ID (1-based, maps to VTS_XX_VOBID.VOB)
	CellID      uint8  // Cell ID within the VOB
	FirstSector uint32 // First sector in the VOB (VOB-relative, inclusive)
	LastSector  uint32 // Last sector in the VOB (VOB-relative, inclusive)
}

// TrackInfo is a minimal description of one audio or subtitle track.
type TrackInfo struct {
	// Index is the 0-based IFO stream index; maps to the Nth audio/sub stream
	// that ffprobe reports from the VOB (IFO order matches VOB stream-ID order).
	Index int

	// Language is the ISO 639-1 two-letter code (e.g. "en", "fr").
	// Empty string when the IFO stores no language information.
	Language string

	// Codec is a human-readable codec name derived from the audio coding mode.
	// Always "dvd_subtitle" for subtitle tracks.
	Codec string

	// Channels is the channel count for audio tracks (0 for subtitle tracks).
	Channels int
}

// ReadTitleInfo opens the VTS IFO at ifoPath and extracts chapter timestamps,
// audio/subtitle track metadata, and duration for the first title-domain PGC.
//
// It is tolerant of IFO files with missing or truncated optional sections;
// partial results are returned rather than errors in those cases.
func ReadTitleInfo(ifoPath string) (*TitleInfo, error) {
	return readTitleInfo(ifoPath, 0)
}

// ReadTitleInfoForTTN opens the VTS IFO at ifoPath and extracts chapter
// timestamps, audio/subtitle track metadata, and duration for the PGC that
// serves the given VTS title number (TTN, 1-based). Multi-PGC discs (e.g.
// scene-segmented titles) give each title its own PGC; reading only the first
// title-domain PGC makes every title report that PGC's duration and chapters.
// A ttn <= 0 behaves like ReadTitleInfo (first title-domain PGC).
func ReadTitleInfoForTTN(ifoPath string, ttn int) (*TitleInfo, error) {
	return readTitleInfo(ifoPath, ttn)
}

func readTitleInfo(ifoPath string, ttn int) (*TitleInfo, error) {
	f, err := os.Open(ifoPath)
	if err != nil {
		return nil, fmt.Errorf("open IFO %s: %w", ifoPath, err)
	}
	defer f.Close()

	mat, err := ReadVTSI(f)
	if err != nil {
		return nil, fmt.Errorf("parse VTS_MAT: %w", err)
	}

	info := &TitleInfo{}

	// ── Audio tracks ─────────────────────────────────────────────────────────
	nAudio := int(mat.VTS_Audio_Streams_Count)
	if nAudio > 8 {
		nAudio = 8
	}
	for i := 0; i < nAudio; i++ {
		aa := mat.VTS_Audio_Attributes[i]
		lang := trimNull(string(aa.LanguageCode[:]))
		info.Audio = append(info.Audio, TrackInfo{
			Index:    i,
			Language: lang,
			Codec:    audioCodecName(aa.AudioCodingMode),
			Channels: int(aa.NumChannels) + 1,
		})
	}

	// ── Subtitle tracks ───────────────────────────────────────────────────────
	nSub := int(mat.VTS_Subpicture_Count)
	if nSub > 32 {
		nSub = 32
	}
	for i := 0; i < nSub; i++ {
		sp := mat.VTS_Subpicture_Attrs[i]
		lang := trimNull(string(sp.LanguageCode[:]))
		info.Subtitles = append(info.Subtitles, TrackInfo{
			Index:    i,
			Language: lang,
			Codec:    "dvd_subtitle",
		})
	}

	// ── Interlaced detection from VTS video attributes ──────────────────────────
	// FilmMode==0 means video-originated (camera) → interlaced.
	// FilmMode==1 means film-originated → progressive (24/25 fps).
	info.Interlaced = (mat.VTS_Attributes.FilmMode == 0)
	logging.Info(logging.CatDVD, "IFO extract: FilmMode=%d → Interlaced=%v (PAL=%v)",
		mat.VTS_Attributes.FilmMode, info.Interlaced, !info.IsNTSC)

	logging.Info(logging.CatDVD, "IFO extract: %d audio, %d subtitle tracks", nAudio, nSub)

	// ── Chapters from VTS_PGCITI ──────────────────────────────────────────────
	if mat.VTS_PGCITI_Offset == 0 {
		logging.Info(logging.CatDVD, "IFO extract: no PGCITI (offset=0), skipping chapters")
		return info, nil
	}

	pgcitiBase := int64(mat.VTS_PGCITI_Offset) * 2048
	if err := readChapters(f, pgcitiBase, info, ttn); err != nil {
		logging.Warning(logging.CatDVD, "IFO extract: chapter read failed (partial info returned): %v", err)
	}
	return info, nil
}

// readChapters reads the VTS_PGCITI starting at pgcitiBase (absolute file
// offset) and populates info.Chapters, info.Duration, info.IsNTSC, info.HasAngles.
// ttn selects the PGC for a specific VTS title (1-based); ttn <= 0 uses the
// first title-domain PGC.
func readChapters(f *os.File, pgcitiBase int64, info *TitleInfo, ttn int) error {
	// PGCITI header: NrOf_PGCI_SRP (uint16) + zero (uint16) + EndByte (uint32)
	hdr := make([]byte, 8)
	if _, err := f.ReadAt(hdr, pgcitiBase); err != nil {
		return fmt.Errorf("read PGCITI header: %w", err)
	}
	nrPGCI := int(binary.BigEndian.Uint16(hdr[0:2]))
	if nrPGCI == 0 {
		return nil
	}

	// PGCI_SRP table: 8 bytes per entry (TitleNr + Category + PGC_Offset)
	srpTable := make([]byte, nrPGCI*8)
	if _, err := f.ReadAt(srpTable, pgcitiBase+8); err != nil {
		return fmt.Errorf("read PGCI_SRP table: %w", err)
	}

	// Select the PGCI_SRP entry for the requested TTN. TitleNr stores the last
	// VTS TTN that shares the entry's PGC, so the PGC for TTN t is the first
	// entry whose masked TitleNr (bit7 is a flag) >= t. When TitleNr is not
	// populated (some authoring tools leave it zero and order entries by TTN),
	// fall back to the entry index ttn-1. ttn <= 0 keeps the legacy behaviour:
	// the first entry with TitleNr > 0 (title-domain PGC), else the first entry.
	sel := 0
	if ttn > 0 {
		sel = -1
		for i := 0; i < nrPGCI; i++ {
			if srpTable[i*8]&0x7F >= byte(ttn) {
				sel = i
				break
			}
		}
		if sel < 0 {
			sel = ttn - 1
			if sel >= nrPGCI {
				sel = nrPGCI - 1
			}
		}
	} else {
		for i := 0; i < nrPGCI; i++ {
			if srpTable[i*8] > 0 {
				sel = i
				break
			}
		}
	}
	pgcOffset := binary.BigEndian.Uint32(srpTable[sel*8+4 : sel*8+8])
	pgcAbsOff := pgcitiBase + int64(pgcOffset)

	// PGC header is 236 bytes.
	pgcHdr := make([]byte, 236)
	if _, err := f.ReadAt(pgcHdr, pgcAbsOff); err != nil {
		return fmt.Errorf("read PGC header: %w", err)
	}

	nrPrograms := int(pgcHdr[2])
	nrCells := int(pgcHdr[3])
	info.Duration = bcdPlaybackToSeconds(pgcHdr[4], pgcHdr[5], pgcHdr[6], pgcHdr[7])
	info.IsNTSC = (pgcHdr[7]>>6)&0x3 == 3

	logging.Info(logging.CatDVD, "IFO extract: PGC has %d programs, %d cells, duration=%.2fs, NTSC=%v",
		nrPrograms, nrCells, info.Duration, info.IsNTSC)

	if nrPrograms == 0 || nrCells == 0 {
		return nil
	}

	// Offsets within the PGC data (uint16, big-endian, from pgcHdr):
	//   [228-229] Command table offset (0 = no commands)
	//   [230-231] Program map offset
	//   [232-233] Cell playback table offset
	//   [234-235] Cell position table offset
	progMapRelOff := int(binary.BigEndian.Uint16(pgcHdr[230:232]))
	cellPlayRelOff := int(binary.BigEndian.Uint16(pgcHdr[232:234]))
	cellPosRelOff := int(binary.BigEndian.Uint16(pgcHdr[234:236]))

	if progMapRelOff == 0 || cellPlayRelOff == 0 {
		return fmt.Errorf("PGC program map or cell table offset is zero")
	}

	// Program map: one byte per program = entry cell number (1-based).
	progMap := make([]byte, nrPrograms)
	if _, err := f.ReadAt(progMap, pgcAbsOff+int64(progMapRelOff)); err != nil {
		return fmt.Errorf("read program map: %w", err)
	}

	// Cell playback table: 24 bytes per cell.
	// PlaybackTime is at bytes 4-7 of each cell entry.
	// BlockMode is at byte 0, bits 7-6.
	cellData := make([]byte, nrCells*24)
	if _, err := f.ReadAt(cellData, pgcAbsOff+int64(cellPlayRelOff)); err != nil {
		return fmt.Errorf("read cell playback table: %w", err)
	}

	// Decode cell durations and detect multi-angle cells.
	cellDur := make([]float64, nrCells)
	for i := 0; i < nrCells; i++ {
		off := i * 24
		cellDur[i] = bcdPlaybackToSeconds(cellData[off+4], cellData[off+5], cellData[off+6], cellData[off+7])
		blockMode := (cellData[off] >> 6) & 0x03
		if blockMode != 0 {
			info.HasAngles = true
		}
	}

	// Cell position table: 4 bytes per cell (uint16 VOBID BE, reserved, CellID),
	// matching the libdvdread on-disc layout. Combine VOBID/CellID with the
	// sector extents from the cell playback table (FirstSector bytes 8-11,
	// LastSector bytes 20-23). Cell sectors are VOB-relative, so a title's PGC
	// cells map to byte ranges [FirstSector*2048, (LastSector+1)*2048) inside
	// VTS_XX_{VOBID}.VOB.
	if cellPosRelOff != 0 {
		cellPos := make([]byte, nrCells*4)
		if _, err := f.ReadAt(cellPos, pgcAbsOff+int64(cellPosRelOff)); err == nil {
			for i := 0; i < nrCells; i++ {
				off := i * 4
				info.Cells = append(info.Cells, TitleCell{
					VOBID:       uint8(binary.BigEndian.Uint16(cellPos[off : off+2])),
					CellID:      cellPos[off+3],
					FirstSector: binary.BigEndian.Uint32(cellData[i*24+8 : i*24+12]),
					LastSector:  binary.BigEndian.Uint32(cellData[i*24+20 : i*24+24]),
				})
			}
		}
	}

	// Build cumulative duration array so we can look up chapter start times.
	// cumDur[i] = sum of durations of cells 0..(i-1).
	cumDur := make([]float64, nrCells+1)
	for i := 0; i < nrCells; i++ {
		cumDur[i+1] = cumDur[i] + cellDur[i]
	}

	// Map each program (chapter) to its start time using its entry cell.
	for _, entryCell := range progMap {
		idx := int(entryCell) - 1 // convert 1-based to 0-based
		if idx < 0 {
			idx = 0
		}
		if idx > nrCells {
			idx = nrCells
		}
		info.Chapters = append(info.Chapters, cumDur[idx])
	}

	logging.Info(logging.CatDVD, "IFO extract: extracted %d chapter timestamps", len(info.Chapters))
	return nil
}

// bcdPlaybackToSeconds converts a 4-byte DVD BCD PlaybackTime to seconds.
// Byte layout: [hh, mm, ss, 0xfrfps] where fr=BCD frame count, fps encodes rate.
func bcdPlaybackToSeconds(hh, mm, ss, fr byte) float64 {
	h := float64(bcdDecode(hh))
	m := float64(bcdDecode(mm))
	s := float64(bcdDecode(ss))
	frames := float64(bcdDecode(fr & 0x3F))
	fps := 25.0
	if (fr>>6)&0x3 == 3 {
		fps = 30000.0 / 1001.0 // NTSC 29.97
	}
	return h*3600 + m*60 + s + frames/fps
}

// bcdDecode converts a single BCD byte to its decimal value.
func bcdDecode(b byte) int {
	return int(b>>4)*10 + int(b&0x0F)
}

// audioCodecName maps the DVD audio coding mode byte to a human-readable name.
func audioCodecName(mode uint8) string {
	switch mode {
	case 0:
		return "ac3"
	case 2:
		return "mp1"
	case 3:
		return "mp2"
	case 4:
		return "lpcm"
	case 6:
		return "dts"
	default:
		return "unknown"
	}
}

// ReadTitleList opens the VIDEO_TS.IFO at vmgPath and returns all title entries
// from the TT_SRPT table. Returns nil, nil if the table is absent or empty.
func ReadTitleList(vmgPath string) ([]TitleSearchPointer, error) {
	f, err := os.Open(vmgPath)
	if err != nil {
		return nil, fmt.Errorf("open VMG IFO %s: %w", vmgPath, err)
	}
	defer f.Close()

	mat, err := ReadVMGI(f)
	if err != nil {
		return nil, fmt.Errorf("parse VMG_MAT: %w", err)
	}

	if mat.TT_SRPT_Offset == 0 {
		logging.Info(logging.CatDVD, "ReadTitleList: TT_SRPT absent (offset=0)")
		return nil, nil
	}

	srptBase := int64(mat.TT_SRPT_Offset) * 2048

	hdr := make([]byte, 8)
	if _, err := f.ReadAt(hdr, srptBase); err != nil {
		return nil, fmt.Errorf("read TT_SRPT header: %w", err)
	}
	numTitles := int(binary.BigEndian.Uint16(hdr[0:2]))
	if numTitles == 0 {
		return nil, nil
	}
	if numTitles > 99 {
		numTitles = 99
	}

	// Each TitleSearchPointer is 12 bytes
	data := make([]byte, numTitles*12)
	if _, err := f.ReadAt(data, srptBase+8); err != nil {
		return nil, fmt.Errorf("read TT_SRPT entries: %w", err)
	}

	titles := make([]TitleSearchPointer, numTitles)
	for i := 0; i < numTitles; i++ {
		off := i * 12
		titles[i] = TitleSearchPointer{
			TitleType:       data[off],
			NumAngles:       data[off+1],
			NumChapters:     binary.BigEndian.Uint16(data[off+2 : off+4]),
			ParentalID:      binary.BigEndian.Uint16(data[off+4 : off+6]),
			VTSNumber:       data[off+6],
			VTS_TitleNumber: data[off+7],
			StartSector:     binary.BigEndian.Uint32(data[off+8 : off+12]),
		}
	}
	logging.Info(logging.CatDVD, "ReadTitleList: found %d titles", numTitles)
	return titles, nil
}

// trimNull removes null bytes and trailing spaces from a string.
func trimNull(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}
