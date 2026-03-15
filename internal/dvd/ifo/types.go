package ifo

// VideoAttributes defines the properties of a video stream on DVD.
type VideoAttributes struct {
	CompressionMode uint8 // 0: MPEG-1, 1: MPEG-2
	TVSystem        uint8 // 0: 525/60 (NTSC), 1: 625/50 (PAL)
	AspectRatio     uint8 // 0: 4:3, 3: 16:9
	PermittedDisplay uint8
	Line21_1        uint8
	Line21_2        uint8
	Resolution      uint8 // 0: 720x480/576, 1: 704x480/576, etc.
	Letterboxed     uint8
	FilmMode        uint8
}

// AudioAttributes defines the properties of an audio stream on DVD.
type AudioAttributes struct {
	AudioCodingMode uint8 // 0: AC-3, 2: MPEG-1/2, 3: LPCM, 4: DTS
	Multichannel    uint8
	LanguageCode    [2]byte
	SpecificCode    uint8
	SampleRate      uint8 // 0: 48kHz
	NumChannels     uint8 // 0: 1ch, 1: 2ch, 5: 6ch
}

// SubpictureAttributes defines the properties of a subpicture stream.
type SubpictureAttributes struct {
	CodingMode   uint8
	LanguageCode [2]byte
	SpecificCode uint8
}

// PlaybackTime represents the DVD BCD time format (8 bytes).
type PlaybackTime struct {
	Hour     uint8
	Minute   uint8
	Second   uint8
	FrameRate uint8 // bits 7-6: rate, 5-0: frames
}
