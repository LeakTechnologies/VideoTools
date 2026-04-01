package ifo

import "strings"

// AudioCodingModeFromCodec maps an FFprobe codec name to the DVD AudioCodingMode value.
//
// DVD spec (Table 3.2.3.1):
//
//	0 = AC-3
//	2 = MPEG-1/MPEG-2 audio
//	3 = LPCM
//	4 = DTS
func AudioCodingModeFromCodec(codec string) uint8 {
	switch strings.ToLower(codec) {
	case "ac3", "eac3", "truehd":
		return 0
	case "mp2", "mp3", "mpeg2", "mpeg1audio":
		return 2
	case "lpcm", "pcm_s16be", "pcm_s24be", "pcm_s16le", "pcm_s24le":
		return 3
	case "dts", "dts-hd", "dtshd":
		return 4
	default:
		return 0 // Default to AC-3
	}
}

// LanguageCodeBytes converts an ISO 639-1 two-letter language code to the
// two-byte representation used in DVD IFO audio and subpicture attribute tables.
// Unknown or empty codes return [0x00, 0x00] (unspecified).
func LanguageCodeBytes(lang string) [2]byte {
	if len(lang) >= 2 {
		return [2]byte{lang[0], lang[1]}
	}
	return [2]byte{0x00, 0x00}
}

// NumChannelsField converts a raw channel count to the DVD IFO field value,
// which is stored as (channels - 1), clamped to 0–7.
func NumChannelsField(channels int) uint8 {
	if channels <= 1 {
		return 0
	}
	v := uint8(channels - 1)
	if v > 7 {
		v = 7
	}
	return v
}
