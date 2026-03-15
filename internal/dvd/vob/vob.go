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
	SubStreamAC3 = 0x80 // AC3 starts at 0x80
	SubStreamDTS = 0x88 // DTS starts at 0x88
	SubStreamSPU = 0x20 // SPU (Subpicture) starts at 0x20
)

// DVD Sector size (VOB packs are always 2048 bytes)
const PackSize = 2048

// Muxer handles multiplexing MPEG-2, AC3, and SPU into a VOB stream.
type Muxer struct {
	w io.Writer
	
	// Muxing state
	scr     uint64 // System Clock Reference (90kHz base)
	muxRate uint32 // Mux rate in 50 bytes/sec units
}

// NewMuxer creates a new VOB muxer.
func NewMuxer(w io.Writer) *Muxer {
	logging.Info(logging.CatDVD, "Initializing new VOB muxer")
	return &Muxer{
		w:       w,
		muxRate: 25200, // Default for DVD (10.08 Mbps)
	}
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

// WritePESHeader writes a PES header with optional PTS/DTS.
func (m *Muxer) WritePESHeader(streamID uint8, length uint16, pts uint64, dts uint64, hasDTS bool) error {
	var buf [19]byte
	binary.BigEndian.PutUint32(buf[0:4], 0x00000100 | uint32(streamID))
	binary.BigEndian.PutUint16(buf[4:6], length)
	
	buf[6] = 0x80 
	
	var headerLen int
	if hasDTS {
		buf[7] = 0xC0 // PTS and DTS
		buf[8] = 10
		m.encodeTimestamp(buf[9:14], 0x30, pts)
		m.encodeTimestamp(buf[14:19], 0x10, dts)
		headerLen = 19
	} else {
		buf[7] = 0x80 // PTS only
		buf[8] = 5
		m.encodeTimestamp(buf[9:14], 0x20, pts)
		headerLen = 14
	}

	if _, err := m.w.Write(buf[:headerLen]); err != nil {
		logging.Error(logging.CatDVD, "Failed to write PES header (stream 0x%X): %v", streamID, err)
		return fmt.Errorf("write pes header: %w", err)
	}
	return nil
}

func (m *Muxer) encodeTimestamp(buf []byte, prefix uint8, ts uint64) {
	buf[0] = prefix | uint8((ts>>29)&0x0E) | 0x01
	buf[1] = uint8((ts >> 22) & 0xFF)
	buf[2] = uint8((ts>>14)&0xFE) | 0x01
	buf[3] = uint8((ts >> 7) & 0xFF)
	buf[4] = uint8((ts<<1)&0xFE) | 0x01
}

// WritePadding writes a padding packet.
func (m *Muxer) WritePadding(size int) error {
	if size < 6 {
		return nil
	}
	logging.Debug(logging.CatDVD, "Writing %d bytes of padding", size)
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
	return nil
}
