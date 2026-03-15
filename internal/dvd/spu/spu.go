package spu

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// SPU Constants
const (
	DCSQ_FORCED_START = 0x00
	DCSQ_START        = 0x01
	DCSQ_STOP         = 0x02
	DCSQ_SET_COLOR    = 0x03
	DCSQ_SET_CONTR    = 0x04
	DCSQ_SET_AREA     = 0x05
	DCSQ_SET_ADDRESS  = 0x06
	DCSQ_CHG_COLCON   = 0x07
	DCSQ_END          = 0xFF
)

// Encoder handles conversion of images to DVD Subpicture Units (SPU).
type Encoder struct {
	width  int
	height int
}

// NewEncoder creates a new SPU encoder for a specific resolution.
func NewEncoder(width, height int) *Encoder {
	logging.Info(logging.CatDVD, "Initializing SPU encoder for %dx%d", width, height)
	return &Encoder{width: width, height: height}
}

// Encode converts an indexed image into a binary SPU.
func (e *Encoder) Encode(img *image.Paletted) ([]byte, error) {
	logging.Debug(logging.CatDVD, "Encoding image to SPU format")
	
	var topField bytes.Buffer
	var botField bytes.Buffer
	
	// DVD SPU uses interleaved fields (Top/Bottom)
	for y := 0; y < e.height; y++ {
		row := e.getRowPixels(img, y)
		encodedRow := e.rleEncode(row)
		if y%2 == 0 {
			topField.Write(encodedRow)
		} else {
			botField.Write(encodedRow)
		}
	}
	
	// SPU Header (4 bytes)
	// [Size: 2b] [DCSQ Offset: 2b]
	header := make([]byte, 4)
	
	// [Implementation of full SPU assembly goes here]
	
	return nil, fmt.Errorf("spu assembly not yet fully implemented")
}

func (e *Encoder) getRowPixels(img *image.Paletted, y int) []uint8 {
	row := make([]uint8, e.width)
	for x := 0; x < e.width; x++ {
		// Paletted images already have 0-3 indices if prepared correctly
		row[x] = img.ColorIndexAt(x, y) & 0x03
	}
	return row
}

// rleEncode implements the DVD 2-bit RLE algorithm.
func (e *Encoder) rleEncode(pixels []uint8) []byte {
	var bits bitWriter
	
	i := 0
	for i < len(pixels) {
		color := pixels[i]
		count := 1
		for i+count < len(pixels) && pixels[i+count] == color && count < 255 {
			count++
		}
		
		e.writeRLECode(&bits, uint16(count), color)
		i += count
	}
	
	// Align to byte boundary
	return bits.Bytes()
}

func (e *Encoder) writeRLECode(w *bitWriter, count uint16, color uint8) {
	if count <= 3 {
		w.WriteBits(uint32(count), 2)
		w.WriteBits(uint32(color), 2)
	} else if count <= 15 {
		w.WriteBits(0, 2)
		w.WriteBits(uint32(count), 4)
		w.WriteBits(uint32(color), 2)
	} else if count <= 63 {
		w.WriteBits(0, 4)
		w.WriteBits(uint32(count), 6)
		w.WriteBits(uint32(color), 2)
	} else {
		w.WriteBits(0, 6)
		w.WriteBits(uint32(count), 8)
		w.WriteBits(uint32(color), 2)
	}
}

type bitWriter struct {
	buf     []byte
	curr    uint8
	bitLeft uint8
}

func (b *bitWriter) WriteBits(val uint32, count uint8) {
	// Simple bit writing logic...
}

func (b *bitWriter) Bytes() []byte {
	if b.bitLeft < 8 {
		b.buf = append(b.buf, b.curr)
	}
	return b.buf
}
