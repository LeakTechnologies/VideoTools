package ifo

import (
	"encoding/binary"
)

// SerializeVTSMAT returns a 2048-byte sector with all VTS_MAT fields placed
// at the byte offsets defined by the DVD-Video specification and verified
// against libdvdread's packed vtsi_mat_t struct (ifo_types.h).
//
// The struct is ATTRIBUTE_PACKED (no padding). Field sizes determine offsets.
//
// Key offsets (all big-endian):
//
//	0x000 (0)   — VTS_Identifier "DVDVIDEO-VTS" (12 bytes)
//	0x00C (12)  — VTS_Last_Sector (4 bytes)
//	0x010 (16)  — zero_1 [12 reserved zeros]
//	0x01C (28)  — vtsi_last_sector / VTSTT_VOBS_Last_Sector (4 bytes)
//	0x020 (32)  — zero_2 (1 byte)
//	0x021 (33)  — specification_version (1 byte; zeroed)
//	0x022 (34)  — VTS_Category (4 bytes)
//	0x026 (38)  — zero_3 (2 bytes)
//	0x028 (40)  — zero_4 (2 bytes)
//	0x02A (42)  — zero_5 (1 byte)
//	0x02B (43)  — zero_6 [19 bytes]
//	0x03E (62)  — zero_7 (2 bytes)
//	0x040 (64)  — zero_8 [32 bytes]
//	0x060 (96)  — zero_9 (8 bytes, uint64)
//	0x068 (104) — zero_10 [24 bytes]
//	0x080 (128) — VTSI_Last_Byte (4 bytes; last byte index of this IFO file)
//	0x084 (132) — zero_11 (4 bytes)
//	0x088 (136) — zero_12 [56 bytes] ← must stay zero
//	0x0C0 (192) — vtsm_vobs (4 bytes; start sector of menu VOBs; 0 = none)
//	0x0C4 (196) — vtstt_vobs (4 bytes; start sector of title VOBs on disc)
//	0x0C8 (200) — VTS_PTT_SRPT (4 bytes; chapter/part-of-title table offset)
//	0x0CC (204) — VTS_PGCIT (4 bytes; Program Chain Information Table offset)
//	0x0D0 (208) — VTSM_PGCI_UT (4 bytes; menu PGC unit table; 0 = none)
//	0x0D4 (212) — VTS_TMAPT (4 bytes; time map table offset)
//	0x0D8 (216) — VTSM_C_ADT (4 bytes; menu cell address table; 0 = none)
//	0x0DC (220) — VTSM_VOBU_ADMAP (4 bytes; menu VOBU address map; 0 = none)
//	0x0E0 (224) — VTS_C_ADT (4 bytes; title cell address table offset)
//	0x0E4 (228) — VTS_VOBU_ADMAP (4 bytes; title VOBU address map offset)
//	0x0E8 (232) — zero_13 [24 bytes]
//	0x100 (256) — vtsm_video_attr (2 bytes; menu domain video; zeroed)
//	0x102 (258) — zero_14 (1 byte)
//	0x103 (259) — nr_of_vtsm_audio_streams (1 byte; 0 = no menu audio)
//	0x104 (260) — vtsm_audio_attr (8 bytes; single menu audio entry)
//	0x10C (268) — zero_15 [56 bytes] (7 × 8-byte audio_attr slots)
//	0x144 (324) — zero_16 [17 bytes]
//	0x155 (341) — nr_of_vtsm_subp_streams (1 byte; 0 = no menu subpicture)
//	0x156 (342) — vtsm_subp_attr (6 bytes; single menu subpicture entry)
//	0x15C (348) — zero_17 [162 bytes] (27 × 6-byte subp_attr slots) ← must stay zero
//	0x1FE (510) — zero_18 [2 bytes]
//	0x200 (512) — vts_video_attr (2 bytes; title domain video attributes)
//	0x202 (514) — zero_19 (1 byte)
//	0x203 (515) — nr_of_vts_audio_streams (1 byte)
//	0x204 (516) — vts_audio_attr[8] (8 × 8 bytes = 64 bytes)
//	0x244 (580) — zero_20 [17 bytes]
//	0x255 (597) — nr_of_vts_subp_streams (1 byte)
//	0x256 (598) — vts_subp_attr[32] (32 × 6 bytes = 192 bytes)
//	0x316 (790) — zero_21 [2 bytes]
//	0x318 (792) — [rest zeros, padded to 2048]
func SerializeVTSMAT(mat *VTS_MAT) []byte {
	b := make([]byte, 2048)

	// 0x000: Identifier
	copy(b[0:12], mat.VTS_Identifier[:])

	// 0x00C: VTS_Last_Sector
	binary.BigEndian.PutUint32(b[12:16], mat.VTS_Last_Sector)

	// 0x010-0x01B: zero_1 (12 bytes, zeroed by make)

	// 0x01C: vtsi_last_sector (last sector of the VTSI IFO file)
	// We repurpose VTS_BUP_Last_Sector for this field.
	binary.BigEndian.PutUint32(b[28:32], mat.VTS_BUP_Last_Sector)

	// 0x020: zero_2; 0x021: specification_version (both zeroed by make)

	// 0x022: VTS_Category
	binary.BigEndian.PutUint32(b[34:38], mat.VTS_Category)

	// 0x026-0x02B: zero_3, zero_4, zero_5 (zeroed by make)
	// 0x02B-0x03D: zero_6 [19 bytes] (zeroed by make)
	// 0x03E-0x07F: zero_7, zero_8, zero_9, zero_10 (zeroed by make)

	// 0x080: VTSI_Last_Byte (last byte index of the VTSI IFO file = file_size - 1)
	binary.BigEndian.PutUint32(b[128:132], mat.VTSI_Last_Byte)

	// 0x084-0x0BF: zero_11 [4 bytes] + zero_12 [56 bytes] (zeroed by make — must stay zero)

	// 0x0C0: vtsm_vobs — start sector of VTSM VOBs (0 = no menu VOBs; zeroed by make)

	// 0x0C4: vtstt_vobs — start sector of title VOBs on disc
	binary.BigEndian.PutUint32(b[196:200], mat.VTSTT_VOBS_Sector)

	// 0x0C8: VTS_PTT_SRPT offset (chapter table; 0 = not present)
	binary.BigEndian.PutUint32(b[200:204], mat.VTS_PTT_SRPT_Offset)

	// 0x0CC: VTS_PGCIT offset (Program Chain Information Table)
	binary.BigEndian.PutUint32(b[204:208], mat.VTS_PGCITI_Offset)

	// 0x0D0: VTSM_PGCI_UT offset (menu PGC unit table; 0 = no menu)
	binary.BigEndian.PutUint32(b[208:212], mat.VTS_M_PGCI_UT_Offset)

	// 0x0D4: VTS_TMAPT offset (time map table)
	binary.BigEndian.PutUint32(b[212:216], mat.VTS_TMAPTI_Offset)

	// 0x0D8: VTSM_C_ADT offset (menu cell address table; 0 = no menu)
	binary.BigEndian.PutUint32(b[216:220], mat.VTS_M_C_ADT_Offset)

	// 0x0DC: VTSM_VOBU_ADMAP offset (menu VOBU address map; 0 = no menu)
	binary.BigEndian.PutUint32(b[220:224], mat.VTS_M_VOBU_ADMAP_Offset)

	// 0x0E0: VTS_C_ADT offset (title cell address table)
	binary.BigEndian.PutUint32(b[224:228], mat.VTS_C_ADT_Offset)

	// 0x0E4: VTS_VOBU_ADMAP offset (title VOBU address map)
	binary.BigEndian.PutUint32(b[228:232], mat.VTS_VOBU_ADMAP_Offset)

	// 0x0E8-0xFF: zero_13 [24 bytes] (zeroed by make)

	// 0x100-0x1FD: VTSM domain fields (menu video/audio/subpicture attrs)
	// We generate no-menu VTS, so all VTSM fields remain zero.
	// zero_17[27] at 0x15C-0x1FD must also remain zero (enforced by make).

	// 0x200: VTS title domain video attributes (2 bytes)
	vb0, vb1 := packVideoAttrs(mat.VTS_Attributes)
	b[512] = vb0
	b[513] = vb1

	// 0x202: zero_19 (zeroed by make)

	// 0x203: nr_of_vts_audio_streams (1 byte)
	b[515] = uint8(mat.VTS_Audio_Streams_Count)

	// 0x204: vts_audio_attr[8] — each entry is 8 bytes, packed bit fields
	for i := 0; i < 8; i++ {
		aa := mat.VTS_Audio_Attributes[i]
		off := 516 + i*8
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

	// 0x244-0x254: zero_20 [17 bytes] (zeroed by make)

	// 0x255: nr_of_vts_subp_streams (1 byte)
	b[597] = uint8(mat.VTS_Subpicture_Count)

	// 0x256: vts_subp_attr[32] — each entry is 6 bytes
	for i := 0; i < 32; i++ {
		sp := mat.VTS_Subpicture_Attrs[i]
		off := 598 + i*6
		b[off+0] = sp.CodingMode
		// byte 1: reserved zero
		copy(b[off+2:off+4], sp.LanguageCode[:])
		b[off+4] = sp.SpecificCode
		// byte 5: code_extension zero
	}

	// 0x316-0x7FF: zero_21 + rest (zeroed by make)
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
// at the byte offsets specified by the DVD-Video specification (Part 3, §3.2)
// and verified against libdvdread's ifo_types.h vmgi_mat_t layout.
//
// Key offsets (all big-endian):
//
//	0x000 (0)   — VMG_Identifier "DVDVIDEO-VMG" (12 bytes)
//	0x00C (12)  — VMG_Last_Sector (4 bytes)
//	0x010 (16)  — [12 reserved zeros]
//	0x01C (28)  — VMGM_Last_Sector (4 bytes; 0 if no menu VOB set)
//	0x020 (32)  — zero_2 (1 byte)
//	0x021 (33)  — specification_version (1 byte; zeroed)
//	0x022 (34)  — VMG_Category (4 bytes)
//	0x026 (38)  — Nr_Of_Volumes (2 bytes; always 1)
//	0x028 (40)  — This_Volume_Nr (2 bytes; always 1)
//	0x02A (42)  — Disc_Side (1 byte; 1 = side A)
//	0x02B (43)  — zero_3 (19 reserved bytes — must be zero)
//	0x03E (62)  — Nr_Of_Title_Sets (2 bytes; total VTS count on disc)
//	0x040 (64)  — Provider_Identifier (32 bytes; zeroed)
//	0x060 (96)  — VMG_Pos_Code (8 bytes; zeroed)
//	0x068 (104) — zero_4 (24 reserved bytes)
//	0x080 (128) — VMGI_Last_Byte (4 bytes; last byte of this IFO sector = 2047)
//	0x084 (132) — First_Play_PGC (4 bytes; byte offset to first-play PGC in IFO)
//	0x088 (136) — zero_5 (56 reserved bytes — must be zero)
//	0x0C0 (192) — VMGM_VOBS_Sector (4 bytes; start sector of menu VOBs; 0 if none)
//	0x0C4 (196) — TT_SRPT_Offset (4 bytes) ← Title Search Pointer Table sector
//	0x0C8 (200) — VMGM_PGCI_UT_Offset (4 bytes; VMG menu PGC)
//	0x0CC (204) — VMG_PTL_MAIT_Offset (4 bytes; parental; 0)
//	0x0D0 (208) — VMG_VTS_ATRT_Offset (4 bytes; VTS attribute table)
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

	// 0x010-0x01B: zero_1 (12 bytes; zeroed by make)

	// 0x01C: VMGM_Last_Sector (0 = no menu VOB set)
	binary.BigEndian.PutUint32(b[28:32], mat.VMG_BUP_Last_Sector)

	// 0x020: zero_2 (1 byte), 0x021: specification_version (zeroed)

	// 0x022: VMG_Category
	binary.BigEndian.PutUint32(b[34:38], uint32(mat.VMG_Category))

	// 0x026: Nr_Of_Volumes (1 for single-disc sets)
	binary.BigEndian.PutUint16(b[38:40], 1)

	// 0x028: This_Volume_Nr (1)
	binary.BigEndian.PutUint16(b[40:42], 1)

	// 0x02A: Disc_Side (1 = side A)
	b[42] = 1

	// 0x02B-0x03D: zero_3 (19 bytes; must be zero — zeroed by make)

	// 0x03E: Nr_Of_Title_Sets
	binary.BigEndian.PutUint16(b[62:64], mat.NrOfTitleSets)

	// 0x040: Provider_Identifier (32 bytes; zeroed)
	// 0x060: VMG_Pos_Code (8 bytes; zeroed)
	// 0x068-0x07F: zero_4 (24 bytes; zeroed by make)

	// 0x080: VMGI_Last_Byte (last byte index of the whole VMGI management area;
	// First_Play_PGC must be < this, so a hardcoded 2047 fails once the VMGI
	// spans more than one sector).
	lastByte := mat.VMGI_Last_Byte
	if lastByte == 0 {
		lastByte = 2047
	}
	binary.BigEndian.PutUint32(b[128:132], lastByte)

	// 0x084: First_Play_PGC — byte offset within the IFO file of the PGC that
	// DVD players execute first when the disc is inserted. Set to the main menu
	// PGC so the player shows the menu. 0 means "skip to title 1 directly".
	binary.BigEndian.PutUint32(b[132:136], mat.VMG_FirstPlayPGC)

	// 0x092-0x0BF: Reserved (46 bytes)

	// 0x0C0: VMGM_VOBS_Sector (start sector of VMGM VOBs; 0 = no menu VOBs)
	binary.BigEndian.PutUint32(b[192:196], mat.VMGM_VOBS_Sector)

	// 0x0C4: TT_SRPT_Offset (Title Search Pointer Table)
	binary.BigEndian.PutUint32(b[196:200], mat.TT_SRPT_Offset)

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
