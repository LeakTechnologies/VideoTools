package vob

import (
	"encoding/binary"
	"fmt"
	"io"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// MPEG-PS Start Codes
const (
	PackStartCode     = 0x000001BA
	SystemHeaderCode  = 0x000001BB
	PaddingStreamCode = 0x000001BE
	PrivateStream1    = 0x000001BD
	VideoStream0      = 0x000001E0
)

// DVD Private Stream 1 Sub-stream IDs
const (
	SubStreamAC3Base = 0x80 // AC3 starts at 0x80 (up to 0x87)
	SubStreamDTSBase = 0x88 // DTS starts at 0x88 (up to 0x8F)
	SubStreamSPUBase = 0x20 // SPU starts at 0x20 (up to 0x3F)
)

// DVD Sector size (VOB packs are always 2048 bytes)
const PackSize = 2048

// Muxer handles multiplexing MPEG-2, AC3, and SPU into a VOB stream.
type Muxer struct {
	w io.Writer
	
	// Muxing state
	scr           uint64   // System Clock Reference (90kHz base)
	muxRate       uint32   // Mux rate in 50 bytes/sec units
	currentSector uint32   // Current sector address within the VOB set
	NAVPCKSectors []uint32 // List of sector addresses where NAV_PCKs were written
}

// NewMuxer creates a new VOB muxer.
func NewMuxer(w io.Writer) *Muxer {
	logging.Info(logging.CatDVD, "Initializing new VOB muxer")
	return &Muxer{
		w:       w,
		muxRate: 25200, // Default for DVD (10.08 Mbps)
	}
}

// GetNAVPCKSectors returns the collected sector map.
func (m *Muxer) GetNAVPCKSectors() []uint32 {
	return m.NAVPCKSectors
}

// WritePackHeader writes a Pack Header to the stream.
func (m *Muxer) WritePackHeader(scr uint64) error {
	var buf [14]byte
	binary.BigEndian.PutUint32(buf[0:4], PackStartCode)
	
	base := scr / 300
	ext := scr % 300
	
	buf[4] = 0x44 | uint8((base>>30)&0x07)
	buf[5] = uint8((base >> 22) & 0xFF)
	buf[6] = 0x01 | uint8((base>>14)&0xFE)
	buf[7] = uint8((base >> 7) & 0xFF)
	buf[8] = 0x01 | uint8((base<<1)&0xFE)
	buf[9] = 0x01 | uint8((ext>>1)&0x7F)
	buf[10] = 0x01 | uint8((ext<<7)&0x80)
	
	buf[10] |= uint8((m.muxRate >> 15) & 0x7F)
	buf[11] = uint8((m.muxRate >> 7) & 0xFF)
	buf[12] = 0x01 | uint8((m.muxRate<<1)&0xFE)
	
	buf[13] = 0xF8 // stuffing length = 0
	
	if _, err := m.w.Write(buf[:]); err != nil {
		logging.Error(logging.CatDVD, "Failed to write pack header at SCR %d: %v", scr, err)
		return fmt.Errorf("write pack header: %w", err)
	}
	return nil
}

// WritePESHeader writes a PES header with optional PTS/DTS and DVD sub-stream support.
func (m *Muxer) WritePESHeader(streamID uint8, subStreamID uint8, payloadLen uint16, pts uint64, dts uint64, hasDTS bool) error {
	// PES length includes header data and payload
	headerDataLen := 5
	if hasDTS {
		headerDataLen = 10
	}
	
	// For Private Stream 1, we also include the 1-byte sub-stream ID
	isPrivate1 := streamID == 0xBD
	totalPESHeaderLen := 3 + headerDataLen // flags(1) + flags(1) + dataLen(1) + data
	if isPrivate1 {
		totalPESHeaderLen += 1
	}
	
	totalLen := uint16(totalPESHeaderLen) + payloadLen
	
	var buf [20]byte
	binary.BigEndian.PutUint32(buf[0:4], 0x00000100|uint32(streamID))
	binary.BigEndian.PutUint16(buf[4:6], totalLen)
	
	buf[6] = 0x80 // Fixed 10
	if hasDTS {
		buf[7] = 0xC0 // PTS and DTS
		buf[8] = uint8(headerDataLen)
		m.encodeTimestamp(buf[9:14], 0x30, pts)
		m.encodeTimestamp(buf[14:19], 0x10, dts)
		if isPrivate1 {
			buf[19] = subStreamID
			_, err := m.w.Write(buf[:20])
			return err
		}
		_, err := m.w.Write(buf[:19])
		return err
	} else {
		buf[7] = 0x80 // PTS only
		buf[8] = uint8(headerDataLen)
		m.encodeTimestamp(buf[9:14], 0x20, pts)
		if isPrivate1 {
			buf[14] = subStreamID
			_, err := m.w.Write(buf[:15])
			return err
		}
		_, err := m.w.Write(buf[:14])
		return err
	}
}

func (m *Muxer) encodeTimestamp(buf []byte, prefix uint8, ts uint64) {
	buf[0] = prefix | uint8((ts>>29)&0x0E) | 0x01
	buf[1] = uint8((ts >> 22) & 0xFF)
	buf[2] = uint8((ts>>14)&0xFE) | 0x01
	buf[3] = uint8((ts >> 7) & 0xFF)
	buf[4] = uint8((ts<<1)&0xFE) | 0x01
}

// TickSCR advances the System Clock Reference by the given number of 27MHz ticks.
// Call this to keep SCR in sync as data is written between nav packs.
func (m *Muxer) TickSCR(ticks uint64) {
	m.scr += ticks
}

// WriteVideo writes an MPEG-2 video PES packet inside a pack.
// data should be a complete MPEG-2 video elementary stream access unit.
// pts is the presentation timestamp in 90kHz ticks.
func (m *Muxer) WriteVideo(data []byte, pts uint64) error {
	if len(data) == 0 {
		return nil
	}

	// Pack header (14 bytes) + PES header (14 bytes with PTS) + payload
	const pesHeaderLen = 14
	payloadLen := uint16(len(data))
	packPayload := pesHeaderLen + len(data)

	if err := m.WritePackHeader(m.scr); err != nil {
		return fmt.Errorf("video pack header: %w", err)
	}
	if err := m.WritePESHeader(uint8(VideoStream0&0xFF), 0, payloadLen, pts, 0, false); err != nil {
		return fmt.Errorf("video PES header: %w", err)
	}
	if _, err := m.w.Write(data); err != nil {
		return fmt.Errorf("video payload: %w", err)
	}

	// Pad to sector boundary
	written := 14 + packPayload
	if rem := written % PackSize; rem != 0 {
		if err := m.WritePadding(PackSize - rem); err != nil {
			return fmt.Errorf("video padding: %w", err)
		}
	} else {
		m.currentSector++
	}

	// Advance SCR by one sector worth of 90kHz ticks (≈ 2048 bytes at 10.08 Mbps)
	m.scr += 1800
	return nil
}

// WriteAudio writes an AC-3 audio PES packet inside a pack.
// data should be a complete AC-3 audio frame.
// pts is the presentation timestamp in 90kHz ticks.
// subStreamID should be SubStreamAC3Base (0x80) for the first audio track.
func (m *Muxer) WriteAudio(data []byte, pts uint64, subStreamID uint8) error {
	if len(data) == 0 {
		return nil
	}

	payloadLen := uint16(len(data))

	if err := m.WritePackHeader(m.scr); err != nil {
		return fmt.Errorf("audio pack header: %w", err)
	}
	// Private Stream 1 (0xBD) carries AC-3/DTS sub-streams
	if err := m.WritePESHeader(0xBD, subStreamID, payloadLen, pts, 0, false); err != nil {
		return fmt.Errorf("audio PES header: %w", err)
	}
	if _, err := m.w.Write(data); err != nil {
		return fmt.Errorf("audio payload: %w", err)
	}

	// Pad to sector boundary
	written := 14 + 15 + len(data) // pack + private1 PES + payload
	if rem := written % PackSize; rem != 0 {
		if err := m.WritePadding(PackSize - rem); err != nil {
			return fmt.Errorf("audio padding: %w", err)
		}
	} else {
		m.currentSector++
	}

	m.scr += 1800
	return nil
}

// WritePadding writes a padding packet and finalizes the sector.
func (m *Muxer) WritePadding(size int) error {
	if size < 6 {
		// If padding is too small, we might need stuffing bytes in the previous PES header,
		// but for DVD sectors we usually aim for large padding or perfect fit.
		padding := make([]byte, size)
		if _, err := m.w.Write(padding); err != nil {
			return err
		}
		m.currentSector++
		return nil
	}
	
	var buf [6]byte
	binary.BigEndian.PutUint32(buf[0:4], PaddingStreamCode)
	binary.BigEndian.PutUint16(buf[4:6], uint16(size-6))
	if _, err := m.w.Write(buf[:]); err != nil {
		return fmt.Errorf("write padding header: %w", err)
	}
	padding := make([]byte, size-6)
	if _, err := m.w.Write(padding); err != nil {
		return fmt.Errorf("write padding data: %w", err)
	}
	
	m.currentSector++
	return nil
}
