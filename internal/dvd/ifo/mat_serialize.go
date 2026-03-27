package ifo

import (
	"encoding/binary"
)

// SerializeVTSMAT returns a 2048-byte sector with all VTS_MAT fields placed
// at the byte offsets specified by the DVD-Video specification (Part 3, §4.2).
//
// Our Go struct uses sequential binary.Write which puts fields at wrong positions
// because the spec has reserved gaps between logical sections. This function
// writes each field directly at its correct spec-defined offset.
//
// Key offsets (all big-endian):
//
//	0x000 (0)   — VTS_Identifier "DVDVIDEO-VTS" (12 bytes)
//	0x00C (12)  — VTS_Last_Sector (4 bytes)
//	0x010 (16)  — [12 reserved zeros]
//	0x01C (28)  — VTSTT_VOBS_Last_Sector (4 bytes)
//	0x020 (32)  — [8 reserved zeros]
//	0x028 (40)  — VTSI_MAT_Last_Sector (4 bytes)
//	0x02C (44)  — [1 reserved zero]
//	0x02D (45)  — VTS_Category (4 bytes)
//	0x031 (49)  — [90 reserved zeros]
//	0x08B (139) — NrOf_Audio_Streams (2 bytes)
//	0x08D (141) — Audio_Attr[8] (8 × 8 bytes = 64 bytes)
//	0x0CD (205) — [17 reserved zeros]
//	0x0DE (222) — NrOf_Subpicture_Streams (2 bytes)
//	0x0E0 (224) — Subpicture_Attr[32] (32 × 6 bytes = 192 bytes)
//	0x1A0 (416) — [2 reserved zeros]
//	0x1A2 (418) — VTS_PTT_SRPT_Offset (4 bytes)
//	0x1A6 (422) — VTS_PGCIT_Offset (4 bytes)  ← PGCITI
//	0x1AA (426) — VTS_M_PGCI_UT_Offset (4 bytes)
//	0x1AE (430) — VTS_TMAPT_Offset (4 bytes)
//	0x1B2 (434) — VTS_M_C_ADT_Offset (4 bytes)
//	0x1B6 (438) — VTS_M_VOBU_ADMAP_Offset (4 bytes)
//	0x1BA (442) — VTS_C_ADT_Offset (4 bytes)
//	0x1BE (446) — VTS_VOBU_ADMAP_Offset (4 bytes)
//	0x1C2 (450) — [rest zeros, padded to 2048]
func SerializeVTSMAT(mat *VTS_MAT) []byte {
	b := make([]byte, 2048)

	// 0x000: Identifier
	copy(b[0:12], mat.VTS_Identifier[:])

	// 0x00C: VTS_Last_Sector
	binary.BigEndian.PutUint32(b[12:16], mat.VTS_Last_Sector)

	// 0x010-0x01B: Reserved (12 bytes, zeroed by make)

	// 0x01C: VTSTT_VOBS_Last_Sector (Title VOB set last sector)
	// We repurpose VTS_BUP_Last_Sector for this field.
	binary.BigEndian.PutUint32(b[28:32], mat.VTS_BUP_Last_Sector)

	// 0x020-0x027: Reserved (8 bytes)

	// 0x028: VTSI_MAT_Last_Sector
	binary.BigEndian.PutUint32(b[40:44], mat.VTS_MAT_Last_Sector)

	// 0x02C: Reserved (1 byte)

	// 0x02D: VTS_Category
	binary.BigEndian.PutUint32(b[45:49], mat.VTS_Category)

	// 0x031-0x08A: Reserved (90 bytes)

	// 0x08B: NrOf_Audio_Streams
	binary.BigEndian.PutUint16(b[139:141], mat.VTS_Audio_Streams_Count)

	// 0x08D: Audio_Attr[8] — each entry is 8 bytes, packed bit fields
	for i := 0; i < 8; i++ {
		aa := mat.VTS_Audio_Attributes[i]
		off := 141 + i*8
		// Byte 0: [7:5]=audio_format, [4]=multichannel_ext, [3:2]=lang_type, [1:0]=app_mode
		b[off+0] = (aa.AudioCodingMode & 0x07) << 5
		if aa.Multichannel != 0 {
			b[off+0] |= 0x10
		}
		// Byte 1: [7:6]=quantization, [5:4]=sample_freq, [3]=0, [2:0]=num_channels
		b[off+1] = ((aa.SampleRate & 0x03) << 4) | (aa.NumChannels & 0x07)
		// Bytes 2-3: lang_code
		copy(b[off+2:off+4], aa.LanguageCode[:])
		// Byte 4: lang_extension (SpecificCode)
		b[off+4] = aa.SpecificCode
		// Bytes 5-7: zeroed (code_extension, unknown, app_mode_ext)
	}

	// 0x0CD-0x0DD: Reserved (17 bytes)

	// 0x0DE: NrOf_Subpicture_Streams
	binary.BigEndian.PutUint16(b[222:224], mat.VTS_Subpicture_Count)

	// 0x0E0: Subpicture_Attr[32] — each entry is 6 bytes
	for i := 0; i < 32; i++ {
		sp := mat.VTS_Subpicture_Attrs[i]
		off := 224 + i*6
		b[off+0] = sp.CodingMode
		// byte 1: reserved zero
		copy(b[off+2:off+4], sp.LanguageCode[:])
		b[off+4] = sp.SpecificCode
		// byte 5: code_extension zero
	}

	// 0x1A0-0x1A1: Reserved (2 bytes)

	// 0x1A2: VTS_PTT_SRPT_Offset (chapter/part-of-title table; 0 = not present)
	binary.BigEndian.PutUint32(b[418:422], mat.VTS_PTT_SRPT_Offset)

	// 0x1A6: VTS_PGCIT_Offset (Program Chain Information Table)
	binary.BigEndian.PutUint32(b[422:426], mat.VTS_PGCITI_Offset)

	// 0x1AA: VTS_M_PGCI_UT_Offset (Menu PGC Unit Table; 0 = no menu)
	binary.BigEndian.PutUint32(b[426:430], mat.VTS_M_PGCI_UT_Offset)

	// 0x1AE: VTS_TMAPT_Offset (Time Map Table)
	binary.BigEndian.PutUint32(b[430:434], mat.VTS_TMAPTI_Offset)

	// 0x1B2: VTS_M_C_ADT_Offset (Menu Cell Address Table; 0 = no menu)
	binary.BigEndian.PutUint32(b[434:438], mat.VTS_M_C_ADT_Offset)

	// 0x1B6: VTS_M_VOBU_ADMAP_Offset (Menu VOBU Address Map; 0 = no menu)
	binary.BigEndian.PutUint32(b[438:442], mat.VTS_M_VOBU_ADMAP_Offset)

	// 0x1BA: VTS_C_ADT_Offset (Title Cell Address Table; 0 = not present)
	binary.BigEndian.PutUint32(b[442:446], mat.VTS_C_ADT_Offset)

	// 0x1BE: VTS_VOBU_ADMAP_Offset (Title VOBU Address Map)
	binary.BigEndian.PutUint32(b[446:450], mat.VTS_VOBU_ADMAP_Offset)

	// 0x200 (512): VTS title video attributes (2 bytes)
	// This tells hardware players the MPEG version, TV system (NTSC/PAL),
	// aspect ratio and display flags for the title VOB set.
	vb0, vb1 := packVideoAttrs(mat.VTS_Attributes)
	b[512] = vb0
	b[513] = vb1

	// 0x202-0x7FF: zeroed (make already zeros the whole slice)
	return b
}

// packVideoAttrs packs a VideoAttributes struct into the 2-byte DVD-Video format:
//
//	Byte 0: [7:6]=MPEG_version, [5:4]=video_format, [3:2]=aspect_ratio, [1:0]=permitted_df
//	Byte 1: [7]=line21_cc1, [6]=line21_cc2, [5]=0, [4]=bit_rate,
//	        [3:2]=picture_size, [1]=letterboxed, [0]=film_mode
func packVideoAttrs(va VideoAttributes) (byte, byte) {
	b0 := ((va.CompressionMode & 0x03) << 6) |
		((va.TVSystem & 0x03) << 4) |
		((va.AspectRatio & 0x03) << 2) |
		(va.PermittedDisplay & 0x03)
	b1 := ((va.Line21_1 & 0x01) << 7) |
		((va.Line21_2 & 0x01) << 6) |
		((va.Resolution & 0x03) << 2) |
		((va.Letterboxed & 0x01) << 1) |
		(va.FilmMode & 0x01)
	return b0, b1
}

// SerializeVMGMAT returns a 2048-byte sector with all VMG_MAT fields placed
// at the byte offsets specified by the DVD-Video specification (Part 3, §3.2).
//
// Key offsets (all big-endian):
//
//	0x000 (0)   — VMG_Identifier "DVDVIDEO-VMG" (12 bytes)
//	0x00C (12)  — VMG_Last_Sector (4 bytes)
//	0x010 (16)  — [12 reserved zeros]
//	0x01C (28)  — VMGM_VOBS_Last_Sector (4 bytes; 0 if no menu VOB)
//	0x020 (32)  — [8 reserved zeros]
//	0x028 (40)  — VMGI_MAT_Last_Sector (4 bytes)
//	0x02C (44)  — VMG_Category (4 bytes; note: no 1-byte reserved gap before it)
//	0x030 (48)  — Nr_Of_Volumes (2 bytes; always 1 for single-disc sets)
//	0x032 (50)  — This_Volume_Nr (2 bytes; always 1)
//	0x034 (52)  — Disc_Side (1 byte; 1 = side A)
//	0x035 (53)  — [19 reserved zeros]
//	0x048 (72)  — Nr_Of_Title_Sets (2 bytes; total number of VTS on disc)
//	0x04A (74)  — Provider_Identifier (32 bytes; padded with spaces/zeros)
//	0x06A (106) — VMG_Pos_Code (8 bytes; zeroed)
//	0x072 (114) — [24 reserved zeros]
//	0x08A (138) — VMGI_MAT_Last_Byte (4 bytes; endByte of this structure)
//	0x08E (142) — VMGM_VOBS_Start_Sector (4 bytes; 0 if no menu VOB)
//	0x092 (146) — [46 reserved zeros]
//	0x0C0 (192) — TT_SRPT_Offset (4 bytes) ← Title Search Pointer Table
//	0x0C4 (196) — VMG_PTT_SRPT_Offset (4 bytes; 0)
//	0x0C8 (200) — VMGM_PGCI_UT_Offset (4 bytes; VMG menu PGC)
//	0x0CC (204) — VMG_PTL_MAIT_Offset (4 bytes; parental; 0)
//	0x0D0 (208) — VMG_VTS_ATRT_Offset (4 bytes; VTS attribute table; 0)
//	0x0D4 (212) — VMG_TXTDT_MG_Offset (4 bytes; text data; 0)
//	0x0D8 (216) — VMG_M_C_ADT_Offset (4 bytes; menu cell ADT; 0)
//	0x0DC (220) — VMG_M_VOBU_ADMAP_Offset (4 bytes; menu VOBU map; 0)
//	0x0E0 (224) — [rest zeros, padded to 2048]
func SerializeVMGMAT(mat *VMG_MAT) []byte {
	b := make([]byte, 2048)

	// 0x000: Identifier
	copy(b[0:12], mat.VMG_Identifier[:])

	// 0x00C: VMG_Last_Sector
	binary.BigEndian.PutUint32(b[12:16], mat.VMG_Last_Sector)

	// 0x010-0x01B: Reserved (12 bytes)

	// 0x01C: VMGM_VOBS_Last_Sector (0 = no menu VOB)
	binary.BigEndian.PutUint32(b[28:32], mat.VMG_BUP_Last_Sector)

	// 0x020-0x027: Reserved (8 bytes)

	// 0x028: VMGI_MAT_Last_Sector
	binary.BigEndian.PutUint32(b[40:44], mat.VMG_MAT_Last_Sector)

	// 0x02C: VMG_Category (note: no reserved byte before it, unlike VTS_MAT)
	binary.BigEndian.PutUint32(b[44:48], uint32(mat.VMG_Category))

	// 0x030: Nr_Of_Volumes (1 for single-disc sets)
	binary.BigEndian.PutUint16(b[48:50], 1)

	// 0x032: This_Volume_Nr (1)
	binary.BigEndian.PutUint16(b[50:52], 1)

	// 0x034: Disc_Side (1 = side A)
	b[52] = 1

	// 0x035-0x047: Reserved (19 bytes)

	// 0x048: Nr_Of_Title_Sets
	binary.BigEndian.PutUint16(b[72:74], mat.NrOfTitleSets)

	// 0x04A: Provider_Identifier (32 bytes; zeroed)

	// 0x06A: VMG_Pos_Code (8 bytes; zeroed)

	// 0x072-0x089: Reserved (24 bytes)

	// 0x08A: VMGI_MAT_Last_Byte (last byte of the VMGI management table).
	// Most tools set this to 2047 (= full 2048-byte sector - 1).
	binary.BigEndian.PutUint32(b[138:142], uint32(2047))

	// 0x08E: First_Play_PGC — byte offset within the IFO file of the PGC that
	// DVD players execute first when the disc is inserted. Set to the main menu
	// PGC so the player shows the menu. 0 means "skip to title 1 directly".
	binary.BigEndian.PutUint32(b[142:146], mat.VMG_FirstPlayPGC)

	// 0x092-0x0BF: Reserved (46 bytes)

	// 0x0C0: TT_SRPT_Offset (Title Search Pointer Table)
	binary.BigEndian.PutUint32(b[192:196], mat.TT_SRPT_Offset)

	// 0x0C4: VMG_PTT_SRPT_Offset (0 = not present)
	binary.BigEndian.PutUint32(b[196:200], mat.VMG_PTT_SRPT_Offset)

	// 0x0C8: VMGM_PGCI_UT_Offset (menu PGC; maps to VMG_PGCITI_Offset)
	binary.BigEndian.PutUint32(b[200:204], mat.VMG_PGCITI_Offset)

	// 0x0CC: VMG_PTL_MAIT_Offset (parental management; 0)
	binary.BigEndian.PutUint32(b[204:208], mat.VMG_PTL_MAIT_Offset)

	// 0x0D0: VMG_VTS_ATRT_Offset (VTS attribute table; 0)
	binary.BigEndian.PutUint32(b[208:212], mat.VMG_VTS_ATRT_Offset)

	// 0x0D4: VMG_TXTDT_MG_Offset (text data; 0)
	binary.BigEndian.PutUint32(b[212:216], mat.VMG_TXTDT_MG_Offset)

	// 0x0D8: VMG_M_C_ADT_Offset (menu cell address table; 0)
	binary.BigEndian.PutUint32(b[216:220], mat.VMG_M_C_ADT_Offset)

	// 0x0DC: VMG_M_VOBU_ADMAP_Offset (menu VOBU address map; 0)
	binary.BigEndian.PutUint32(b[220:224], mat.VMG_M_VOBU_ADMAP_Offset)

	// 0x0E0-0x7FF: zeroed (make already zeros the whole slice)
	return b
}
