package udf

import (
	"bytes"
	"testing"
)

// TestBuild_ISO9660PVD_Magic verifies the ISO 9660 magic "CD001" appears at sector 16, offset 1.
func TestBuild_ISO9660PVD_Magic(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "TEST_DISC")
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	data := buf.Bytes()
	const pvdOffset = 16 * SectorSize
	if len(data) < pvdOffset+6 {
		t.Fatalf("output too short (%d bytes) to contain PVD at sector 16", len(data))
	}
	// Byte 0 of PVD: descriptor type (1 = PVD); bytes 1-5: "CD001".
	if data[pvdOffset] != ISO9660PVDType {
		t.Errorf("PVD type byte = 0x%02X, want 0x%02X (ISO9660PVDType)", data[pvdOffset], ISO9660PVDType)
	}
	magic := string(data[pvdOffset+1 : pvdOffset+6])
	if magic != "CD001" {
		t.Errorf("PVD identifier = %q, want \"CD001\"", magic)
	}
}

// TestBuild_SectorAlignment verifies the output is a multiple of SectorSize.
func TestBuild_SectorAlignment(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "ALIGN_TEST")
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	if buf.Len()%SectorSize != 0 {
		t.Errorf("output length %d is not a multiple of SectorSize %d", buf.Len(), SectorSize)
	}
}

// TestBuild_MinimumSize verifies Build() produces at least 258 sectors
// (system area 16 + VDS 2 + path tables 2 + dirs 1 + UDF VDS 16 + AVDP 1 + FSD 1 + ICBs).
func TestBuild_MinimumSize(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "MIN_TEST")
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	const minSectors = 259
	if buf.Len() < minSectors*SectorSize {
		t.Errorf("output %d bytes < minimum %d bytes (%d sectors)",
			buf.Len(), minSectors*SectorSize, minSectors)
	}
}

// TestBuild_VolumeDescriptorSetTerminator verifies sector 17 is the VDS terminator.
func TestBuild_VolumeDescriptorSetTerminator(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "TERM_TEST")
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	data := buf.Bytes()
	const termOffset = 17 * SectorSize
	if len(data) < termOffset+6 {
		t.Fatalf("output too short to contain sector 17")
	}
	if data[termOffset] != ISO9660TermType {
		t.Errorf("sector 17 type = 0x%02X, want 0xFF (ISO9660TermType)", data[termOffset])
	}
	magic := string(data[termOffset+1 : termOffset+6])
	if magic != "CD001" {
		t.Errorf("sector 17 identifier = %q, want \"CD001\"", magic)
	}
}

// TestBuild_SystemAreaZero verifies sectors 0-15 are the blank system area.
func TestBuild_SystemAreaZero(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "ZERO_TEST")
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	data := buf.Bytes()
	const sysAreaEnd = 16 * SectorSize
	for i := 0; i < sysAreaEnd; i++ {
		if data[i] != 0 {
			t.Errorf("system area byte[%d] = 0x%02X, want 0x00", i, data[i])
			break
		}
	}
}

// TestEncodeCS0_Roundtrip verifies EncodeCS0 sets the compression byte and copies content.
func TestEncodeCS0_Roundtrip(t *testing.T) {
	result := EncodeCS0("HELLO", 16)
	if len(result) != 16 {
		t.Fatalf("EncodeCS0 length = %d, want 16", len(result))
	}
	if result[0] != 8 {
		t.Errorf("compression byte = %d, want 8 (CS0)", result[0])
	}
	if string(result[1:6]) != "HELLO" {
		t.Errorf("content = %q, want \"HELLO\"", string(result[1:6]))
	}
}

// TestEncodeCS0_Empty verifies empty string yields a zeroed buffer.
func TestEncodeCS0_Empty(t *testing.T) {
	result := EncodeCS0("", 8)
	for i, b := range result {
		if b != 0 {
			t.Errorf("EncodeCS0(\"\") byte[%d] = 0x%02X, want 0x00", i, b)
			break
		}
	}
}

// TestCalculateCRC_Deterministic verifies CalculateCRC returns the same value for the same input.
func TestCalculateCRC_Deterministic(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	a := CalculateCRC(data)
	b := CalculateCRC(data)
	if a != b {
		t.Errorf("CalculateCRC not deterministic: %04X != %04X", a, b)
	}
}

// TestCalculateCRC_DifferentInputs verifies distinct inputs produce distinct CRCs.
func TestCalculateCRC_DifferentInputs(t *testing.T) {
	a := CalculateCRC([]byte{0x00})
	b := CalculateCRC([]byte{0xFF})
	if a == b {
		t.Errorf("CalculateCRC collision: both inputs produced 0x%04X", a)
	}
}

// TestWriteDescriptor_TagOffsets verifies that WriteDescriptor places DescriptorCRC and
// DescriptorCRCLen at the correct byte offsets within the 16-byte tag header.
// UDF spec: [8-9] DescriptorCRC, [10-11] DescriptorCRCLen, [12-15] TagLocation.
// Regression: a prior bug wrote CRC to offset 10 and CRCLen to offset 12.
func TestWriteDescriptor_TagOffsets(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "OFFSET_TEST")

	// Trigger WriteDescriptor indirectly by building with an empty tree.
	// Build() writes FSD (TagIDFSD) and other volume descriptors via WriteDescriptor.
	if err := w.Build(); err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	data := buf.Bytes()

	// The FSD is at sector 257 (offset 257*2048).
	const fsdOffset = 257 * SectorSize
	if len(data) < fsdOffset+16 {
		t.Fatalf("output too short to contain FSD at sector 257")
	}
	tag := data[fsdOffset : fsdOffset+16]

	// TagIdentifier at [0-1] should be TagIDFSD (256).
	tagID := uint16(tag[0]) | uint16(tag[1])<<8
	if tagID != TagIDFSD {
		t.Errorf("FSD TagIdentifier = %d, want %d (TagIDFSD)", tagID, TagIDFSD)
	}

	// DescriptorCRCLen at [10-11] must be non-zero (FSD is larger than the 16-byte tag).
	crcLen := uint16(tag[10]) | uint16(tag[11])<<8
	if crcLen == 0 {
		t.Errorf("FSD DescriptorCRCLen at offset 10 = 0; "+
			"likely CRC/CRCLen swapped to wrong offsets (regression check)")
	}

	// DescriptorCRC at [8-9] must equal the CRC of the descriptor content.
	storedCRC := uint16(tag[8]) | uint16(tag[9])<<8
	content := data[fsdOffset+16 : fsdOffset+16+int(crcLen)]
	expectedCRC := CalculateCRC(content)
	if storedCRC != expectedCRC {
		t.Errorf("FSD DescriptorCRC at offset 8 = 0x%04X, want 0x%04X "+
			"(CRC of %d content bytes)", storedCRC, expectedCRC, crcLen)
	}

	// TagChecksum at [4] must be valid.
	wantChecksum := CalculateChecksum(tag)
	if tag[4] != wantChecksum {
		t.Errorf("FSD TagChecksum = 0x%02X, want 0x%02X", tag[4], wantChecksum)
	}
}
