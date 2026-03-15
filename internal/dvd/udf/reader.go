package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Reader provides methods to read files from a UDF 1.02 ISO image.
type Reader struct {
	rs io.ReadSeeker
}

// NewReader creates a new UDF reader.
func NewReader(rs io.ReadSeeker) *Reader {
	return &Reader{rs: rs}
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
		return DiscTypeDVD, nil
	}
	if strings.Contains(content, "BDMV") {
		return DiscTypeBluRay, nil
	}
	
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
	
	avdp, err := r.readAVDP()
	if err != nil {
		return fmt.Errorf("read avdp: %w", err)
	}

	lvd, err := r.findLVD(avdp.MainVolumeDescriptorSeq)
	if err != nil {
		return fmt.Errorf("find lvd: %w", err)
	}

	fsd, err := r.findFSD(lvd)
	if err != nil {
		return fmt.Errorf("find fsd: %w", err)
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	return r.extractRecursively(fsd.RootDirectoryICB, targetDir, destPath)
}

func (r *Reader) readAVDP() (*AnchorVolumeDescriptorPointer, error) {
	tagID, data, err := r.ReadDescriptor(256)
	if err != nil {
		return nil, err
	}
	if tagID != TagIDAVDP {
		return nil, fmt.Errorf("not an AVDP (tag %d)", tagID)
	}
	avdp := &AnchorVolumeDescriptorPointer{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, avdp)
	return avdp, nil
}

func (r *Reader) findLVD(extent ExtentAd) (*LogicalVolumeDescriptor, error) {
	numSectors := extent.Len / SectorSize
	for i := uint32(0); i < numSectors; i++ {
		tagID, data, err := r.ReadDescriptor(extent.Location + i)
		if err != nil {
			continue
		}
		if tagID == TagIDLVD {
			lvd := &LogicalVolumeDescriptor{}
			binary.Read(bytes.NewReader(data), binary.LittleEndian, lvd)
			return lvd, nil
		}
		if tagID == TagIDTerm {
			break
		}
	}
	return nil, fmt.Errorf("LVD not found in VDS")
}

func (r *Reader) findFSD(lvd *LogicalVolumeDescriptor) (*FileSetDescriptor, error) {
	tagID, data, err := r.ReadDescriptor(257)
	if err == nil && tagID == TagIDFSD {
		fsd := &FileSetDescriptor{}
		binary.Read(bytes.NewReader(data), binary.LittleEndian, fsd)
		return fsd, nil
	}
	return nil, fmt.Errorf("FSD not found at sector 257")
}

func (r *Reader) extractRecursively(icb LongAd, targetDir, destPath string) error {
	tagID, data, err := r.ReadDescriptor(icb.Location)
	if err != nil || tagID != TagIDICB {
		return fmt.Errorf("failed to read ICB at %d", icb.Location)
	}
	
	entry := &FileEntryICB{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, entry)

	if entry.ICBTag.FileType == 1 { // Directory
		fids, err := r.readFIDs(entry)
		if err != nil {
			return err
		}
		
		for _, fid := range fids {
			if fid.Name == "." || fid.Name == ".." {
				continue
			}
			
			if targetDir != "" {
				if strings.EqualFold(fid.Name, targetDir) {
					// Recurse into targetDir but set targetDir to "" so we extract its children
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

func (r *Reader) readFIDs(entry *FileEntryICB) ([]fidInfo, error) {
	// For DVD-Video compliance, directory data usually follows ICB or is in next sector
	// Simplified contiguous read for initial implementation
	if _, err := r.rs.Seek(int64(entry.Tag.TagLocation+1)*SectorSize, io.SeekStart); err != nil {
		return nil, err
	}
	
	buf := make([]byte, SectorSize)
	if _, err := io.ReadFull(r.rs, buf); err != nil {
		return nil, err
	}
	
	var fids []fidInfo
	offset := 0
	for offset < SectorSize-38 {
		tagID := binary.LittleEndian.Uint16(buf[offset : offset+2])
		if tagID != TagIDFID {
			break
		}
		
		charLen := int(buf[offset+19])
		if charLen == 0 {
			// Root or dot entry
			offset += 38
			offset = (offset + 3) & ^3
			continue
		}
		
		fid := fidInfo{
			Name:  string(buf[offset+38 : offset+38+charLen]),
			IsDir: (buf[offset+18] & 0x02) != 0,
		}
		
		icbData := buf[offset+20 : offset+36]
		binary.Read(bytes.NewReader(icbData), binary.LittleEndian, &fid.ICB)
		
		fids = append(fids, fid)
		
		offset += 38 + charLen
		offset = (offset + 3) & ^3
	}
	
	return fids, nil
}

func (r *Reader) extractFile(icb LongAd, destPath string) error {
	logging.Debug(logging.CatDVD, "Extracting file to %s (location %d, len %d)", destPath, icb.Location, icb.Len)
	
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	
	if _, err := r.rs.Seek(int64(icb.Location)*SectorSize, io.SeekStart); err != nil {
		return err
	}
	
	_, err = io.CopyN(f, r.rs, int64(icb.Len))
	return err
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
		return 0, nil, err
	}
	
	return tagID, data, nil
}

// GetVolumeLabel returns the disc label from the PVD.
func (r *Reader) GetVolumeLabel() (string, error) {
	tagID, data, err := r.ReadDescriptor(32) 
	if err != nil || tagID != TagIDPVD {
		return "", fmt.Errorf("failed to read PVD")
	}
	pvd := &PrimaryVolumeDescriptor{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, pvd)
	
	label := string(pvd.VolumeIdentifier[1:])
	return strings.TrimRight(label, "\x00 "), nil
}
