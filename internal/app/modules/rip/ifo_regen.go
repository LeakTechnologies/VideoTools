package rip

import (
	"fmt"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
)

// convertedVTS holds metadata about one converted VOB set for IFO regeneration.
type convertedVTS struct {
	Name       string   // "VTS_01", "VTS_02", or "VIDEO_TS"
	VOBPath    string   // absolute path to the converted VOB
	VOBSize    int64    // file size in bytes
	IsMenu     bool     // true for VIDEO_TS.VOB (menu)
	ChapterSec []float64 // chapter timestamps in seconds (scaled for region convert)
	Duration   float64  // title duration in seconds (0 if unknown)
}

// RegenerateIFOs reads the original IFO structure from sourceVideoTS and
// generates new IFO/BUP files in outputVideoTS with correct NTSC/PAL timing
// and video attributes matching the converted VOBs.
//
// This is Stage 3 of the full-disc PAL→NTSC conversion pipeline.
func RegenerateIFOs(sourceVideoTS, outputVideoTS string, vtsList []convertedVTS, isNTSC bool, appendLog func(string)) error {
	if len(vtsList) == 0 {
		return fmt.Errorf("no converted VTS sets to generate IFOs for")
	}

	// Read original VMG IFO to get title list structure
	vmgPath := filepath.Join(sourceVideoTS, "VIDEO_TS.IFO")
	titleSPs, err := ifo.ReadTitleList(vmgPath)
	if err != nil {
		appendLog(fmt.Sprintf("Warning: could not read original title list: %v", err))
		titleSPs = nil
	}

	// Read original VMG_MAT for metadata
	origVMGMAT, err := readVMGMAT(vmgPath)
	if err != nil {
		appendLog(fmt.Sprintf("Warning: could not read original VMG_MAT: %v", err))
		origVMGMAT = nil
	}

	nrOfTitleSets := uint16(0)
	if origVMGMAT != nil {
		nrOfTitleSets = origVMGMAT.NrOfTitleSets
	}

	// Phase 1: Regenerate per-VTS IFO files
	var vtsMats []*ifo.VTS_MAT
	var vtsATRTEntries []ifo.VTS_ATRT_Entry

	for _, cv := range vtsList {
		if cv.IsMenu {
			continue // menu VOB has no VTS IFO
		}

		// Extract VTS number from name ("VTS_01" → 1)
		var vtsNum int
		if n, err := fmt.Sscanf(cv.Name, "VTS_%d", &vtsNum); err != nil || n != 1 {
			appendLog(fmt.Sprintf("Warning: could not parse VTS number from %s, skipping IFO", cv.Name))
			continue
		}

		// Read original VTS IFO for audio/subtitle attributes
		origVTSIFO := filepath.Join(sourceVideoTS, cv.Name+"_0.IFO")
		origMat, _ := readVTSMAT(origVTSIFO)

		// Build new VTS_MAT with NTSC/PAL video attributes
		newMat := buildVTSMat(cv, origMat, isNTSC)

		// Build single-cell PGC with correct NTSC timing
		pgc := buildVTSPGC(cv, isNTSC)

		// Build TMAPT from VOB file size
		var tmapt *ifo.VTS_TMAPT
		totalSectors := uint32(cv.VOBSize / 2048)
		if totalSectors > 0 && cv.Duration > 0 {
			tmapt = ifo.BuildLinearTMAPT(totalSectors, cv.Duration, 1)
		}

		// Build chapter table from timestamps
		var pttSrpt *ifo.VTS_PTT_SRPT
		if len(cv.ChapterSec) > 1 {
			pttSrpt = &ifo.VTS_PTT_SRPT{
				NrOfChapters: uint16(len(cv.ChapterSec) - 1),
			}
		}

		// Build VOBU_ADMAP from linear approximation
		var admap *ifo.VOBU_ADMAP
		if totalSectors > 0 {
			sectors := make([]uint32, 0, totalSectors/15)
			for s := uint32(0); s < totalSectors; s += 15 {
				sectors = append(sectors, s)
			}
			if len(sectors) > 0 {
				admap = ifo.BuildVOBU_ADMAP(sectors)
			}
		}

		// Generate VTS_IFO and VTS_BUP
		builder := ifo.NewBuilder(outputVideoTS)
		if err := builder.GenerateVTS_IFO(vtsNum, newMat, pgc, tmapt, admap, pttSrpt); err != nil {
			return fmt.Errorf("generate VTS %d IFO: %w", vtsNum, err)
		}

		appendLog(fmt.Sprintf("Generated VTS_%02d IFO/BUP: %d cells, %d sectors, %.1f sec%s",
			vtsNum, pgc.NrOfCells, totalSectors, cv.Duration,
			chapterSummary(len(cv.ChapterSec))))

		vtsMats = append(vtsMats, newMat)
		vtsATRTEntries = append(vtsATRTEntries, ifo.VTS_ATRT_Entry{
			VTS_MAT_Last_Sector: newMat.VTS_Last_Sector,
			Video_Attrs:         newMat.VTS_Attributes,
			NumAudio:            newMat.VTS_Audio_Streams_Count,
			Audio_Attrs:         newMat.VTS_Audio_Attributes,
			NumSubpicture:       newMat.VTS_Subpicture_Count,
			Subpicture_Attrs:    newMat.VTS_Subpicture_Attrs,
		})
	}

	// Phase 2: Generate VMG IFO with title list
	if len(vtsMats) == 0 {
		appendLog("Warning: no VTS IFOs generated, skipping VMG IFO")
		return nil
	}

	if nrOfTitleSets == 0 {
		nrOfTitleSets = uint16(len(vtsMats))
	}

	vmgMat := ifo.NewVMGMAT()
	vmgMat.NrOfTitleSets = nrOfTitleSets

	// Build TT_SRPT from title data
	srpt := buildTTSRPT(titleSPs, vtsList, vtsMats, isNTSC)

	// Build VTS_ATRT from per-VTS attributes
	atrt := ifo.BuildVTS_ATRT(vtsMats)

	// Generate VMG IFO
	vmgiBuilder := ifo.NewBuilder(outputVideoTS)
	if err := vmgiBuilder.GenerateVMG_IFO(vmgMat, srpt, nil, atrt); err != nil {
		return fmt.Errorf("generate VMG IFO: %w", err)
	}

	appendLog(fmt.Sprintf("Generated VIDEO_TS.IFO/BUP: %d title sets, %d titles",
		nrOfTitleSets, len(srpt.Titles)))

	return nil
}

// readVTSMAT reads the VTS_MAT from a VTS_xx_0.IFO file.
func readVTSMAT(ifoPath string) (*ifo.VTS_MAT, error) {
	f, err := os.Open(ifoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ifo.ReadVTSI(f)
}

// readVMGMAT reads the VMG_MAT from a VIDEO_TS.IFO file.
func readVMGMAT(ifoPath string) (*ifo.VMG_MAT, error) {
	f, err := os.Open(ifoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ifo.ReadVMGI(f)
}

// buildVTSMat creates a VTS_MAT for the converted VOB, copying audio/subtitle
// attributes from the original IFO when available.
func buildVTSMat(cv convertedVTS, origMat *ifo.VTS_MAT, isNTSC bool) *ifo.VTS_MAT {
	mat := ifo.NewVTSMAT()

	// Set video attributes for NTSC or PAL
	mat.VTS_Attributes.CompressionMode = 1 // MPEG-2
	if isNTSC {
		mat.VTS_Attributes.TVSystem = 0 // 525/60 (NTSC)
	} else {
		mat.VTS_Attributes.TVSystem = 1 // 625/50 (PAL)
	}
	mat.VTS_Attributes.AspectRatio = 3    // 16:9 (default, overridden from original)
	mat.VTS_Attributes.Resolution = 0     // 720x480/576
	mat.VTS_Attributes.FilmMode = 1       // progressive (converted from interlaced)

	// Copy audio/subtitle attributes from original IFO
	if origMat != nil {
		mat.VTS_Attributes.AspectRatio = origMat.VTS_Attributes.AspectRatio
		mat.VTS_Attributes.PermittedDisplay = origMat.VTS_Attributes.PermittedDisplay
		mat.VTS_Audio_Streams_Count = origMat.VTS_Audio_Streams_Count
		mat.VTS_Audio_Attributes = origMat.VTS_Audio_Attributes
		mat.VTS_Subpicture_Count = origMat.VTS_Subpicture_Count
		mat.VTS_Subpicture_Attrs = origMat.VTS_Subpicture_Attrs
	} else {
		// Defaults if no original IFO
		mat.VTS_Audio_Streams_Count = 1
		mat.VTS_Audio_Attributes[0] = ifo.AudioAttributes{
			AudioCodingMode: 0, // AC-3
			SampleRate:      0, // 48kHz
			NumChannels:     1, // 2 channels (stereo)
		}
	}

	// Mark no menu VOBs within VTS
	mat.VTS_M_PGCI_UT_Offset = 0
	mat.VTS_M_C_ADT_Offset = 0
	mat.VTS_M_VOBU_ADMAP_Offset = 0

	return mat
}

// buildVTSPGC creates a single-cell PGC with the correct NTSC/PAL duration.
func buildVTSPGC(cv convertedVTS, isNTSC bool) *ifo.ProgramChain {
	duration := cv.Duration
	if duration <= 0 {
		// Estimate duration from VOB size at ~6 Mbps (DVD max)
		duration = float64(cv.VOBSize) / (6_000_000 / 8)
	}

	totalSectors := uint32(cv.VOBSize / 2048)
	lastSector := totalSectors
	if lastSector > 0 {
		lastSector--
	}

	return ifo.BuildSingleCellPGC(0, lastSector, duration, isNTSC)
}

// buildTTSRPT constructs the Title Search Pointer Table from original title data
// and converted VTS info. Falls back to a simple 1-title-per-VTS entry.
func buildTTSRPT(originalSPs []ifo.TitleSearchPointer, vtsList []convertedVTS, vtsMats []*ifo.VTS_MAT, isNTSC bool) *ifo.TT_SRPT {
	if len(originalSPs) > 0 {
		// Map original titles to converted VTS sets
		var titles []ifo.TitleSearchPointer
		for _, sp := range originalSPs {
			vtsNum := int(sp.VTSNumber)
			for _, cv := range vtsList {
				if cv.IsMenu {
					continue
				}
				var cvVTSNum int
				if n, _ := fmt.Sscanf(cv.Name, "VTS_%d", &cvVTSNum); n == 1 && cvVTSNum == vtsNum {
					titles = append(titles, sp)
					break
				}
			}
		}
		if len(titles) > 0 {
			return &ifo.TT_SRPT{
				NumTitles: uint16(len(titles)),
				Titles:    titles,
			}
		}
	}

	// Fallback: one title per VTS
	var titles []ifo.TitleSearchPointer
	titleNum := uint8(1)
	for _, cv := range vtsList {
		if cv.IsMenu {
			continue
		}
		var vtsNum uint8
		if n, _ := fmt.Sscanf(cv.Name, "VTS_%d", &vtsNum); n != 1 {
			continue
		}
		titles = append(titles, ifo.TitleSearchPointer{
			TitleType:       0,
			NumAngles:       1,
			NumChapters:     uint16(len(cv.ChapterSec)),
			VTSNumber:       vtsNum,
			VTS_TitleNumber: titleNum,
			StartSector:     0,
		})
		titleNum++
	}
	return &ifo.TT_SRPT{
		NumTitles: uint16(len(titles)),
		Titles:    titles,
	}
}

func chapterSummary(nChapters int) string {
	if nChapters > 1 {
		return fmt.Sprintf(", %d chapters", nChapters-1)
	}
	return ""
}


