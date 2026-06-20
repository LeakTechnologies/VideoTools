package spu

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// SPU DCSQ command codes (DVD spec §4.6.3).
const (
	DCSQ_FORCED_START = 0x00 // Force display on
	DCSQ_START        = 0x01 // Start display
	DCSQ_STOP         = 0x02 // Stop display
	DCSQ_SET_COLOR    = 0x03 // Map pixel indices → PGC palette entries
	DCSQ_SET_CONTR    = 0x04 // Set opacity for each pixel index
	DCSQ_SET_AREA     = 0x05 // Bounding rectangle of subpicture
	DCSQ_SET_ADDRESS  = 0x06 // Byte offsets to top/bottom field pixel data
	DCSQ_CHG_COLCON   = 0x07 // Per-line color/contrast change (not used here)
	DCSQ_END          = 0xFF // End of DCSQ
)

// SPUOptions controls how 2-bit pixel indices map to the PGC palette.
type SPUOptions struct {
	// PaletteIndices maps each 2-bit SPU pixel color (0-3) to a PGC palette
	// entry index (0-15).
	PaletteIndices [4]uint8
	// AlphaValues sets opacity for each 2-bit SPU pixel color.
	// 0 = fully transparent, 15 = fully opaque.
	AlphaValues [4]uint8
}

// DefaultSPUOptions returns a sensible default:
//   - color 0: palette 0, transparent  (background)
//   - color 1: palette 1, fully opaque (pattern / text)
//   - color 2: palette 2, fully opaque (emphasis / outline)
//   - color 3: palette 3, semi-transparent (shadow)
func DefaultSPUOptions() SPUOptions {
	return SPUOptions{
		PaletteIndices: [4]uint8{0, 1, 2, 3},
		AlphaValues:    [4]uint8{0, 15, 15, 8},
	}
}

// DefaultPalette returns a 4-color palette suitable for DVD menus.
func DefaultPalette() color.Palette {
	return color.Palette{
		color.RGBA{0, 0, 0, 0},         // 0: transparent
		color.RGBA{255, 255, 255, 255}, // 1: white (text/buttons)
		color.RGBA{0, 0, 0, 255},       // 2: black (outline)
		color.RGBA{128, 128, 128, 255}, // 3: gray (shadow)
	}
}

// Encoder converts indexed images into DVD Subpicture Units (SPU).
type Encoder struct {
	width  int
	height int
}

// NewEncoder creates a new SPU encoder for the given display resolution.
func NewEncoder(width, height int) *Encoder {
	logging.Info(logging.CatDVD, "Initializing SPU encoder for %dx%d", width, height)
	return &Encoder{width: width, height: height}
}

// Encode converts an indexed image into a binary DVD SPU packet.
//
// Packet layout (DVD spec 5.1.1):
//
//	[0:2]      SP_DCSQ_SZ  - total packet size (big-endian uint16)
//	[2:4]      SP_DCSQTA   - byte offset to first DCSQ from packet start
//	[4:dcsqTA] pixel data  - top-field (even rows) then bottom-field (odd rows)
//	[dcsqTA:]  DCSQ[0]     - forced display: SET_COLOR, SET_CONTR, SET_AREA, SET_ADDRESS
//	           DCSQ[1]     - STOP terminator (self-referencing)
func (e *Encoder) Encode(img *image.Paletted, opts SPUOptions) ([]byte, error) {
	logging.Debug(logging.CatDVD, "Encoding SPU %dx%d", e.width, e.height)

	// Encode pixel rows: even -> top field, odd -> bottom field.
	var topField, botField bytes.Buffer
	for y := 0; y < e.height; y++ {
		encoded := e.rleEncode(e.getRowPixels(img, y))
		if y%2 == 0 {
			topField.Write(encoded)
		} else {
			botField.Write(encoded)
		}
	}
	topData := topField.Bytes()
	botData := botField.Bytes()

	// Compute byte offsets (all measured from packet start = byte 0).
	topOffset := uint16(4)
	botOffset := uint16(4 + len(topData))
	dcsqTA := uint16(4 + len(topData) + len(botData))

	// Build DCSQ[0] commands
	dcsq0Cmds := buildDCSQ0Commands(opts, e.width, e.height, topOffset, botOffset)
	dcsq0Total := uint16(4 + len(dcsq0Cmds))

	// DCSQ[1] self-reference offset
	dcsq1Offset := dcsqTA + dcsq0Total
	dcsq1 := buildDCSQ1(dcsq1Offset)

	totalSize := int(dcsqTA) + int(dcsq0Total) + len(dcsq1)
	buf := make([]byte, totalSize)

	binary.BigEndian.PutUint16(buf[0:2], uint16(totalSize))
	binary.BigEndian.PutUint16(buf[2:4], dcsqTA)
	copy(buf[4:], topData)
	copy(buf[4+len(topData):], botData)

	// DCSQ[0]
	d0 := int(dcsqTA)
	binary.BigEndian.PutUint16(buf[d0:], 0x0000)
	binary.BigEndian.PutUint16(buf[d0+2:], dcsq1Offset)
	copy(buf[d0+4:], dcsq0Cmds)

	// DCSQ[1]: STOP terminator
	copy(buf[int(dcsq1Offset):], dcsq1)

	logging.Info(logging.CatDVD, "SPU encoded: %d bytes", totalSize)
	return buf, nil
}

// MenuEncoder handles DVD menu SPU generation with button states.
// It converts any image to DVD-compliant indexed palette images.
type MenuEncoder struct {
	width   int
	height  int
	palette color.Palette
}

// NewMenuEncoder creates a new menu SPU encoder.
func NewMenuEncoder(width, height int) *MenuEncoder {
	return &MenuEncoder{
		width:   width,
		height:  height,
		palette: DefaultPalette(),
	}
}

// SetPalette sets the 4-color palette for SPU encoding.
func (e *MenuEncoder) SetPalette(palette color.Palette) {
	e.palette = palette
}

// EncodeMenuImage converts any image to DVD SPU format.
// It maps colors to the palette and generates the SPU packet.
func (e *MenuEncoder) EncodeMenuImage(img image.Image, opts SPUOptions) ([]byte, error) {
	if e.palette == nil {
		e.palette = DefaultPalette()
	}

	// Convert to indexed paletted image
	pi := image.NewPaletted(image.Rect(0, 0, e.width, e.height), e.palette)

	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			c := img.At(x, y)
			pi.SetColorIndex(x, y, e.findClosestColor(c))
		}
	}

	enc := NewEncoder(e.width, e.height)
	return enc.Encode(pi, opts)
}

// findClosestColor finds the closest palette color to the given color.
func (e *MenuEncoder) findClosestColor(c color.Color) uint8 {
	_, r, g, b := c.RGBA()
	r >>= 8
	g >>= 8
	b >>= 8

	minDist := ^uint32(0)
	bestIdx := uint8(0)

	for i := 0; i < len(e.palette) && i < 4; i++ {
		pc := e.palette[i]
		_, pr, pg, pb := pc.RGBA()
		pr >>= 8
		pg >>= 8
		pb >>= 8

		dr := uint32(r) - uint32(pr)
		dg := uint32(g) - uint32(pg)
		db := uint32(b) - uint32(pb)
		dist := dr*dr + dg*dg + db*db

		if dist < minDist {
			minDist = dist
			bestIdx = uint8(i)
		}
	}

	return bestIdx
}

// buildDCSQ0Commands returns command bytes for the display-on DCSQ.
func buildDCSQ0Commands(opts SPUOptions, w, h int, topOffset, botOffset uint16) []byte {
	var b bytes.Buffer

	// FORCED_START: force display onto screen immediately.
	b.WriteByte(DCSQ_FORCED_START)

	// SET_COLOR
	colorWord := uint16(opts.PaletteIndices[3])<<12 |
		uint16(opts.PaletteIndices[2])<<8 |
		uint16(opts.PaletteIndices[1])<<4 |
		uint16(opts.PaletteIndices[0])
	b.WriteByte(DCSQ_SET_COLOR)
	b.WriteByte(byte(colorWord >> 8))
	b.WriteByte(byte(colorWord))

	// SET_CONTR
	contWord := uint16(opts.AlphaValues[3])<<12 |
		uint16(opts.AlphaValues[2])<<8 |
		uint16(opts.AlphaValues[1])<<4 |
		uint16(opts.AlphaValues[0])
	b.WriteByte(DCSQ_SET_CONTR)
	b.WriteByte(byte(contWord >> 8))
	b.WriteByte(byte(contWord))

	// SET_AREA
	b.WriteByte(DCSQ_SET_AREA)
	b.Write(pack12pair(0, uint16(w-1)))
	b.Write(pack12pair(0, uint16(h-1)))

	// SET_ADDRESS
	b.WriteByte(DCSQ_SET_ADDRESS)
	b.WriteByte(byte(topOffset >> 8))
	b.WriteByte(byte(topOffset))
	b.WriteByte(byte(botOffset >> 8))
	b.WriteByte(byte(botOffset))

	b.WriteByte(DCSQ_END)
	return b.Bytes()
}

// buildDCSQ1 returns the 6-byte self-referencing STOP terminator.
func buildDCSQ1(selfOffset uint16) []byte {
	b := make([]byte, 6)
	binary.BigEndian.PutUint16(b[0:2], 0x0000)
	binary.BigEndian.PutUint16(b[2:4], selfOffset)
	b[4] = DCSQ_STOP
	b[5] = DCSQ_END
	return b
}

// pack12pair encodes two 12-bit values into 3 bytes.
func pack12pair(v1, v2 uint16) []byte {
	return []byte{
		byte(v1 >> 4),
		byte(v1&0x0F)<<4 | byte(v2>>8),
		byte(v2),
	}
}

func (e *Encoder) getRowPixels(img *image.Paletted, y int) []uint8 {
	row := make([]uint8, e.width)
	for x := 0; x < e.width; x++ {
		row[x] = img.ColorIndexAt(x, y) & 0x03
	}
	return row
}

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
	if b.bitLeft == 0 {
		b.bitLeft = 8
	}
	for count > 0 {
		take := count
		if take > b.bitLeft {
			take = b.bitLeft
		}
		shift := count - take
		mask := uint32((1 << take) - 1)
		b.curr |= uint8((val>>shift)&mask) << (b.bitLeft - take)
		count -= take
		b.bitLeft -= take
		if b.bitLeft == 0 {
			b.buf = append(b.buf, b.curr)
			b.curr = 0
			b.bitLeft = 8
		}
	}
}

func (b *bitWriter) Bytes() []byte {
	if b.bitLeft < 8 && b.bitLeft > 0 {
		b.buf = append(b.buf, b.curr)
	}
	return b.buf
}
