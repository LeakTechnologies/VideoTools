package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// Reader provides methods to read files from a UDF 1.02 ISO image.
type Reader struct {
	rs                io.ReadSeeker
	partitionStartAbs uint32 // absolute sector where the UDF partition starts (from PD)
}

// NewReader creates a new UDF reader.
func NewReader(rs io.ReadSeeker) *Reader {
	return &Reader{
		rs:                rs,
		partitionStartAbs: partitionStart, // default fallback
	}
}

// DiscType represents the detected format of the disc.
type DiscType string

const (
	DiscTypeDVD     DiscType = "DVD"
	DiscTypeBluRay  DiscType = "Blu-ray"
	DiscTypeUnknown DiscType = "Unknown"
)

// DetectDiscType scans the ISO for specific signatures.
func (r *Reader) DetectDiscType() (DiscType, error) {
	logging.Info(logging.CatDVD, "Detecting disc type via metadata scan...")

	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := r.rs.Seek(0, io.SeekStart); err != nil {
		return DiscTypeUnknown, err
	}

	if _, err := io.ReadFull(r.rs, buf); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return DiscTypeUnknown, err
		}
	}

	content := string(buf)
	if strings.Contains(content, "VIDEO_TS") {
		logging.Info(logging.CatDVD, "Detected disc type: DVD (VIDEO_TS found in metadata)")
		return DiscTypeDVD, nil
	}
	if strings.Contains(content, "BDMV") {
		logging.Info(logging.CatDVD, "Detected disc type: Blu-ray (BDMV found in metadata)")
		return DiscTypeBluRay, nil
	}

	logging.Info(logging.CatDVD, "Disc type not detected (no VIDEO_TS or BDMV in metadata)")
	return DiscTypeUnknown, nil
}

// IdentifyDiscFormat is a static helper for quick detection.
func IdentifyDiscFormat(path string) (DiscType, error) {
	f, err := os.Open(path)
	if err != nil {
		return DiscTypeUnknown, err
	}
	defer f.Close()

	r := NewReader(f)
	return r.DetectDiscType()
}

// ExtractDirectory extracts a directory (like VIDEO_TS) from the ISO to a local path.
func (r *Reader) ExtractDirectory(targetDir, destPath string) error {
	logging.Info(logging.CatDVD, "Extracting directory %s from ISO to %s", targetDir, destPath)

	// Log the total ISO size for diagnostics
	if seeker, ok := r.rs.(io.Seeker); ok {
		size, err := seeker.Seek(0, io.SeekEnd)
		if err == nil {
			logging.Info(logging.CatDVD, "ISO total size: %d bytes (%.1f MB, %d sectors)",
				size, float64(size)/(1024*1024), size/int64(SectorSize))
		}
		seeker.Seek(0, io.SeekStart)
	}

	avdp, err := r.readAVDP()
	if err != nil {
		return fmt.Errorf("read avdp: %w", err)
	}
	logging.Info(logging.CatDVD, "AVDP found: MainVDS at sector %d (len %d, %d sectors)",
		avdp.MainVolumeDescriptorSeq.Location,
		avdp.MainVolumeDescriptorSeq.Len,
		avdp.MainVolumeDescriptorSeq.Len/SectorSize)
	logging.Info(logging.CatDVD, "AVDP reserve VDS at sector %d (len %d)",
		avdp.ReserveVolumeDescriptorSeq.Location,
		avdp.ReserveVolumeDescriptorSeq.Len)

	lvd, err := r.findLVD(avdp.MainVolumeDescriptorSeq)
	if err != nil {
		// Try the reserve VDS before giving up
		if avdp.ReserveVolumeDescriptorSeq.Len > 0 {
			logging.Info(logging.CatDVD, "Main VDS failed, trying reserve VDS at sector %d",
				avdp.ReserveVolumeDescriptorSeq.Location)
			lvd, err = r.findLVD(avdp.ReserveVolumeDescriptorSeq)
		}
		if err != nil {
			return fmt.Errorf("find lvd: %w", err)
		}
	}
	logging.Info(logging.CatDVD, "LVD found: block size=%d, seq=%d",
		lvd.LogicalBlockSize, lvd.VolumeDescriptorSeqNumber)

	fsd, err := r.findFSD(lvd)
	if err != nil {
		return fmt.Errorf("find fsd: %w", err)
	}
	logging.Info(logging.CatDVD, "FSD found: root ICB at sector %d (partition %d)",
		fsd.RootDirectoryICB.Location, fsd.RootDirectoryICB.Partition)

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	return r.extractRecursively(fsd.RootDirectoryICB, targetDir, destPath)
}

func (r *Reader) readAVDP() (*AnchorVolumeDescriptorPointer, error) {
	// UDF 1.02 spec: AVDP should be at sector 256, but also check 512 and N-256.
	// Try the primary location first.
	sectorsToTry := []uint32{256, 512}

	// Get ISO size to compute N-256 location
	if seeker, ok := r.rs.(io.Seeker); ok {
		size, err := seeker.Seek(0, io.SeekEnd)
		if err == nil {
			numSectors := uint32(size / SectorSize)
			if numSectors > 256 {
				sectorsToTry = append(sectorsToTry, numSectors-256)
				sectorsToTry = append(sectorsToTry, numSectors)
			}
		}
		seeker.Seek(0, io.SeekStart)
	}

	var lastErr error
	for _, sector := range sectorsToTry {
		tagID, data, err := r.ReadDescriptor(sector)
		if err != nil {
			logging.Debug(logging.CatDVD, "AVDP check at sector %d: %v", sector, err)
			lastErr = err
			continue
		}
		if tagID == TagIDAVDP {
			avdp := &AnchorVolumeDescriptorPointer{}
			if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, avdp); err != nil {
				logging.Debug(logging.CatDVD, "AVDP at sector %d: parse error %v", sector, err)
				lastErr = err
				continue
			}
			logging.Info(logging.CatDVD, "AVDP found at sector %d: mainVDS=(%d,%d) reserveVDS=(%d,%d)",
				sector,
				avdp.MainVolumeDescriptorSeq.Location, avdp.MainVolumeDescriptorSeq.Len,
				avdp.ReserveVolumeDescriptorSeq.Location, avdp.ReserveVolumeDescriptorSeq.Len)
			return avdp, nil
		}
		logging.Debug(logging.CatDVD, "Sector %d: tag=%d (not AVDP)", sector, tagID)
	}

	return nil, fmt.Errorf("no AVDP found in ISO (tried sectors %v, last err: %v)", sectorsToTry, lastErr)
}

func (r *Reader) findLVD(extent ExtentAd) (*LogicalVolumeDescriptor, error) {
	numSectors := extent.Len / SectorSize
	logging.Info(logging.CatDVD, "Scanning VDS at sector %d for %d sectors...", extent.Location, numSectors)

	foundTags := make(map[uint16]int)
	for i := uint32(0); i < numSectors; i++ {
		sector := extent.Location + i
		tagID, data, err := r.ReadDescriptor(sector)
		if err != nil {
			logging.Debug(logging.CatDVD, "VDS sector %d: read error: %v", sector, err)
			continue
		}

		foundTags[tagID]++

		switch tagID {
		case TagIDLVD:
			lvd := &LogicalVolumeDescriptor{}
			if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, lvd); err != nil {
				logging.Debug(logging.CatDVD, "LVD at sector %d: parse error %v", sector, err)
				continue
			}
			logging.Info(logging.CatDVD, "LVD found at VDS sector %d (abs %d): seq=%d blockSize=%d",
				i, sector, lvd.VolumeDescriptorSeqNumber, lvd.LogicalBlockSize)
			return lvd, nil

		case TagIDPVD:
			logging.Debug(logging.CatDVD, "VDS sector %d (abs %d): PVD", i, sector)

		case TagIDPD:
			pd := &PartitionDescriptor{}
			if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, pd); err == nil {
				if pd.PartitionStartingLocation > 0 {
					r.partitionStartAbs = pd.PartitionStartingLocation
					logging.Debug(logging.CatDVD, "VDS sector %d (abs %d): PD start=%d len=%d",
						i, sector, pd.PartitionStartingLocation, pd.PartitionLength)
				}
			} else {
				logging.Debug(logging.CatDVD, "VDS sector %d (abs %d): PD (parse err %v)", i, sector, err)
			}

		case TagIDTerm:
			logging.Info(logging.CatDVD, "VDS sector %d (abs %d): Terminating descriptor (end of VDS), scanned %d sectors",
				i, sector, i+1)
			break

		case 0:
			logging.Debug(logging.CatDVD, "VDS sector %d (abs %d): empty/zapped sector", i, sector)

		default:
			logging.Debug(logging.CatDVD, "VDS sector %d (abs %d): unknown tag=%d", i, sector, tagID)
		}
	}

	logging.Info(logging.CatDVD, "VDS scan complete: %v", foundTags)
	return nil, fmt.Errorf("LVD not found in VDS (extent at sector %d, len %d, tags found: %v)",
		extent.Location, extent.Len, foundTags)
}

func (r *Reader) findFSD(lvd *LogicalVolumeDescriptor) (*FileSetDescriptor, error) {
	// FSD is normally at partition LBN 0 (absolute sector partitionStart = 257).
	// Try both the standard location and decode the LogicalVolumeContentsUse LongAd.
	sectorsToTry := []uint32{r.partitionStartAbs}

	// Decode LogicalVolumeContentsUse which contains a LongAd pointing to FSD.
	// UDF 2.60+ uses different encoding, but for UDF 1.02 it's a single LongAd.
	if lvd.LogicalVolumeContentsUse[0] != 0 || lvd.LogicalVolumeContentsUse[1] != 0 {
		fsdLen := binary.LittleEndian.Uint32(lvd.LogicalVolumeContentsUse[0:4])
		fsdLoc := binary.LittleEndian.Uint32(lvd.LogicalVolumeContentsUse[4:8])
		fsdPart := binary.LittleEndian.Uint16(lvd.LogicalVolumeContentsUse[8:10])
		if fsdLoc > 0 {
			logging.Debug(logging.CatDVD, "LVD.LogicalVolumeContentsUse points to FSD: loc=%d len=%d part=%d",
				fsdLoc, fsdLen, fsdPart)
			sectorsToTry = append([]uint32{fsdLoc + partitionStart}, sectorsToTry...)
		}
	}

	var lastErr error
	for _, sector := range sectorsToTry {
		tagID, data, err := r.ReadDescriptor(sector)
		if err != nil {
			logging.Debug(logging.CatDVD, "FSD check at sector %d: %v", sector, err)
			lastErr = fmt.Errorf("read sector %d: %w", sector, err)
			continue
		}
		if tagID == TagIDFSD {
			fsd := &FileSetDescriptor{}
			if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, fsd); err != nil {
				logging.Debug(logging.CatDVD, "FSD at sector %d: parse error %v", sector, err)
				lastErr = err
				continue
			}
			logging.Info(logging.CatDVD, "FSD found at sector %d: root ICB loc=%d part=%d",
				sector, fsd.RootDirectoryICB.Location, fsd.RootDirectoryICB.Partition)
			return fsd, nil
		}
		logging.Debug(logging.CatDVD, "Sector %d: tag=%d (not FSD)", sector, tagID)
	}

	return nil, fmt.Errorf("FSD not found (tried sectors %v, last err: %v)", sectorsToTry, lastErr)
}

func (r *Reader) extractRecursively(icb LongAd, targetDir, destPath string) error {
	tagID, data, err := r.ReadDescriptor(icb.Location + r.partitionStartAbs)
	if err != nil || tagID != TagIDICB {
		return fmt.Errorf("failed to read ICB at %d (tag=%d, err=%v)", icb.Location, tagID, err)
	}

	entry := &FileEntryICB{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, entry)

	logging.Debug(logging.CatDVD, "ICB at %d: fileType=%d", icb.Location, entry.ICBTag.FileType)

	if entry.ICBTag.FileType == 1 { // Directory
		fids, err := r.readFIDs(entry, data)
		if err != nil {
			return err
		}

		for _, fid := range fids {
			if fid.Name == "." || fid.Name == ".." {
				continue
			}

			logging.Debug(logging.CatDVD, "FID: %s (dir=%v, ICB loc=%d)", fid.Name, fid.IsDir, fid.ICB.Location)

			if targetDir != "" {
				if strings.EqualFold(fid.Name, targetDir) {
					logging.Info(logging.CatDVD, "Found target directory '%s' at ICB %d", targetDir, fid.ICB.Location)
					if err := r.extractRecursively(fid.ICB, "", destPath); err != nil {
						return err
					}
				}
				continue
			}

			subDest := filepath.Join(destPath, fid.Name)
			if fid.IsDir {
				os.MkdirAll(subDest, 0755)
				if err := r.extractRecursively(fid.ICB, "", subDest); err != nil {
					return err
				}
			} else {
				logging.Debug(logging.CatDVD, "Extracting file: %s (loc=%d, len=%d)", fid.Name, fid.ICB.Location, fid.ICB.Len)
				if err := r.extractFile(fid.ICB, subDest); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type fidInfo struct {
	Name  string
	IsDir bool
	ICB   LongAd
}

// readSector reads raw SectorSize bytes from an absolute sector.
func (r *Reader) readSector(sector uint32) ([]byte, error) {
	buf := make([]byte, SectorSize)
	if _, err := r.rs.Seek(int64(sector)*SectorSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to sector %d: %w", sector, err)
	}
	if _, err := io.ReadFull(r.rs, buf); err != nil {
		return nil, fmt.Errorf("read sector %d: %w", sector, err)
	}
	return buf, nil
}

func (r *Reader) readFIDs(entry *FileEntryICB, rawICBData []byte) ([]fidInfo, error) {
	// First try: use the ICB's allocation descriptors to find the directory data.
	// Parse ShortAd (8-byte) allocation descriptors that follow the FileEntryICB struct.
	feSize := binary.Size(FileEntryICB{})
	allocStart := feSize + int(entry.LengthOfExtendedAttributes)
	allocLen := int(entry.LengthOfAllocationDescriptors)

	var fids []fidInfo
	if allocLen > 0 && allocStart+allocLen <= len(rawICBData) {
		allocData := rawICBData[allocStart : allocStart+allocLen]

		// Collect data from all extents into one buffer.
		var dirBuf bytes.Buffer
		for off := 0; off+8 <= allocLen; off += 8 {
			adLen := binary.LittleEndian.Uint32(allocData[off : off+4])
			adPos := binary.LittleEndian.Uint32(allocData[off+4 : off+8])
			if adLen == 0 {
				continue
			}
			absSector := adPos + r.partitionStartAbs
			numSectors := (adLen + SectorSize - 1) / SectorSize
			for i := uint32(0); i < numSectors; i++ {
				sectorData, err := r.readSector(absSector + i)
				if err != nil {
					logging.Debug(logging.CatDVD, "readFIDs: read sector %d: %v", absSector+i, err)
					continue
				}
				dirBuf.Write(sectorData)
			}
		}

		if dirBuf.Len() > 0 {
			fids = parseFIDs(dirBuf.Bytes())
		}
	}

	// Fallback: TagLocation+1 heuristic for poorly mastered ISOs.
	if len(fids) == 0 {
		sectorData, err := r.readSector(entry.Tag.TagLocation + 1)
		if err != nil {
			return nil, fmt.Errorf("fallback read FID sector at %d: %w", entry.Tag.TagLocation+1, err)
		}
		fids = parseFIDs(sectorData)
	}

	logging.Debug(logging.CatDVD, "readFIDs at ICB sector %d: found %d entries", entry.Tag.TagLocation, len(fids))
	return fids, nil
}

// parseFIDs parses File Identifier Descriptor entries from raw directory data.
func parseFIDs(buf []byte) []fidInfo {
	var fids []fidInfo
	offset := 0
	for offset < len(buf)-38 {
		tagID := binary.LittleEndian.Uint16(buf[offset : offset+2])
		if tagID != TagIDFID {
			break
		}

		charLen := int(buf[offset+19])
		if charLen == 0 {
			offset += 38
			offset = (offset + 3) & ^3
			continue
		}

		name := string(buf[offset+38 : offset+38+charLen])

		fid := fidInfo{
			Name:  name,
			IsDir: (buf[offset+18] & 0x02) != 0,
		}

		icbData := buf[offset+20 : offset+36]
		binary.Read(bytes.NewReader(icbData), binary.LittleEndian, &fid.ICB)

		fids = append(fids, fid)

		offset += 38 + charLen
		offset = (offset + 3) & ^3
	}
	return fids
}

func (r *Reader) extractFile(icb LongAd, destPath string) error {
	logging.Debug(logging.CatDVD, "Extracting file to %s (ICB loc=%d)", destPath, icb.Location)

	// Read the File Entry (ICB) descriptor.
	tagID, data, err := r.ReadDescriptor(icb.Location + r.partitionStartAbs)
	if err != nil || tagID != TagIDICB {
		return fmt.Errorf("read file ICB at %d: tag=%d err=%v", icb.Location+r.partitionStartAbs, tagID, err)
	}
	entry := &FileEntryICB{}
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, entry); err != nil {
		return fmt.Errorf("parse file ICB: %w", err)
	}

	if entry.ICBTag.FileType != 2 { // Not a file
		return fmt.Errorf("expected file but found type %d", entry.ICBTag.FileType)
	}

	fileLen := int64(entry.InformationLength)
	if fileLen == 0 {
		logging.Debug(logging.CatDVD, "extractFile: zero-length file %s", destPath)
		f, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create file %s: %w", destPath, err)
		}
		f.Close()
		return nil
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer f.Close()

	// Read allocation descriptors from ICB data.
	feSize := binary.Size(FileEntryICB{})
	allocStart := feSize + int(entry.LengthOfExtendedAttributes)
	allocLen := int(entry.LengthOfAllocationDescriptors)

	if allocLen == 0 || allocStart+allocLen > len(data) {
		return fmt.Errorf("invalid allocation descriptors: start=%d len=%d dataLen=%d",
			allocStart, allocLen, len(data))
	}

	allocData := data[allocStart : allocStart+allocLen]
	var written int64

	for off := 0; off+8 <= allocLen && written < fileLen; off += 8 {
		adLen := binary.LittleEndian.Uint32(allocData[off : off+4])
		adPos := binary.LittleEndian.Uint32(allocData[off+4 : off+8])
		if adLen == 0 {
			continue
		}
		readLen := int64(adLen)
		if written+readLen > fileLen {
			readLen = fileLen - written
		}
		absSector := adPos + r.partitionStartAbs
		if _, err := r.rs.Seek(int64(absSector)*SectorSize, io.SeekStart); err != nil {
			return fmt.Errorf("seek to file extent at %d: %w", absSector, err)
		}
		if _, err := io.CopyN(f, r.rs, readLen); err != nil {
			return fmt.Errorf("copy file extent at %d: %w", absSector, err)
		}
		written += readLen
	}

	if written < fileLen {
		return fmt.Errorf("short read: wrote %d of %d bytes for %s", written, fileLen, destPath)
	}

	logging.Debug(logging.CatDVD, "Extracted %s: %d bytes from %d extents", destPath, written, (allocLen+7)/8)
	return nil
}

// ReadDescriptor reads a UDF descriptor from a specific sector.
func (r *Reader) ReadDescriptor(sector uint32) (uint16, []byte, error) {
	if _, err := r.rs.Seek(int64(sector)*SectorSize, io.SeekStart); err != nil {
		return 0, nil, err
	}

	header := make([]byte, 16)
	if _, err := io.ReadFull(r.rs, header); err != nil {
		return 0, nil, err
	}

	tagID := binary.LittleEndian.Uint16(header[0:2])
	dataLen := binary.LittleEndian.Uint16(header[12:14])

	if dataLen > SectorSize-16 {
		dataLen = SectorSize - 16
	}

	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r.rs, data); err != nil {
		return 0, nil, fmt.Errorf("read descriptor data at sector %d (tag=%d, dataLen=%d): %w",
			sector, tagID, dataLen, err)
	}

	return tagID, data, nil
}

// ReadFileData returns the raw bytes of a single file identified by its UDF path
// (e.g. "VIDEO_TS/VIDEO_TS.IFO"). The path separator is "/" and matching is
// case-insensitive.
func (r *Reader) ReadFileData(udfPath string) ([]byte, error) {
	parts := strings.Split(strings.Trim(udfPath, "/"), "/")

	avdp, err := r.readAVDP()
	if err != nil {
		return nil, fmt.Errorf("read AVDP: %w", err)
	}
	lvd, err := r.findLVD(avdp.MainVolumeDescriptorSeq)
	if err != nil {
		return nil, fmt.Errorf("find LVD: %w", err)
	}
	fsd, err := r.findFSD(lvd)
	if err != nil {
		return nil, fmt.Errorf("find FSD: %w", err)
	}

	// Walk from root ICB through each path component.
	icb := fsd.RootDirectoryICB
	for _, part := range parts {
		tagID, data, err := r.ReadDescriptor(icb.Location + r.partitionStartAbs)
		if err != nil || tagID != TagIDICB {
			return nil, fmt.Errorf("read ICB at %d: tag=%d err=%v", icb.Location+r.partitionStartAbs, tagID, err)
		}
		entry := &FileEntryICB{}
		binary.Read(bytes.NewReader(data), binary.LittleEndian, entry)

		if entry.ICBTag.FileType != 1 { // Not a directory
			return nil, fmt.Errorf("expected directory at component %q", part)
		}

		fids, err := r.readFIDs(entry, data)
		if err != nil {
			return nil, fmt.Errorf("read FIDs: %w", err)
		}

		found := false
		for _, fid := range fids {
			if fid.Name == "." || fid.Name == ".." {
				continue
			}
			if !strings.EqualFold(fid.Name, part) {
				continue
			}
			icb = fid.ICB
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("path component %q not found in ISO", part)
		}
	}

	// Read file data from the ICB's allocation descriptors.
	tagID, data, err := r.ReadDescriptor(icb.Location + r.partitionStartAbs)
	if err != nil || tagID != TagIDICB {
		return nil, fmt.Errorf("read file ICB at %d: tag=%d err=%v", icb.Location+r.partitionStartAbs, tagID, err)
	}
	entry := &FileEntryICB{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, entry)

	if entry.ICBTag.FileType != 2 { // Not a file
		return nil, fmt.Errorf("expected file but found type %d", entry.ICBTag.FileType)
	}

	fileLen := int64(entry.InformationLength)
	buf := bytes.NewBuffer(make([]byte, 0, fileLen))

	feSize := binary.Size(FileEntryICB{})
	allocStart := feSize + int(entry.LengthOfExtendedAttributes)
	allocLen := int(entry.LengthOfAllocationDescriptors)

	if allocLen > 0 && allocStart+allocLen <= len(data) {
		allocData := data[allocStart : allocStart+allocLen]
		for off := 0; off+8 <= allocLen && int64(buf.Len()) < fileLen; off += 8 {
			adLen := binary.LittleEndian.Uint32(allocData[off : off+4])
			adPos := binary.LittleEndian.Uint32(allocData[off+4 : off+8])
			if adLen == 0 {
				continue
			}
			readLen := int64(adLen)
			if int64(buf.Len())+readLen > fileLen {
				readLen = fileLen - int64(buf.Len())
			}
			absSector := adPos + r.partitionStartAbs
			if _, err := r.rs.Seek(int64(absSector)*SectorSize, io.SeekStart); err != nil {
				return nil, fmt.Errorf("seek to file extent at %d: %w", absSector, err)
			}
			if _, err := io.CopyN(buf, r.rs, readLen); err != nil {
				return nil, fmt.Errorf("read file extent at %d: %w", absSector, err)
			}
		}
	}

	if int64(buf.Len()) < fileLen {
		return nil, fmt.Errorf("short read for %s: got %d of %d bytes", udfPath, buf.Len(), fileLen)
	}

	return buf.Bytes(), nil
}

// GetVolumeLabel returns the disc label from the PVD.
func (r *Reader) GetVolumeLabel() (string, error) {
	tagID, data, err := r.ReadDescriptor(32)
	if err != nil || tagID != TagIDPVD {
		return "", fmt.Errorf("failed to read PVD at sector 32 (tag=%d, err=%v)", tagID, err)
	}
	pvd := &PrimaryVolumeDescriptor{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, pvd)

	label := string(pvd.VolumeIdentifier[1:])
	return strings.TrimRight(label, "\x00 "), nil
}
