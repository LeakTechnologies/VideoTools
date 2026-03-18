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
	Hour      uint8
	Minute    uint8
	Second    uint8
	FrameRate uint8 // bits 7-6: rate (0x40=25fps, 0xC0=29.97fps), bits 5-0: BCD frame count
}

// ProgramChain represents a DVD Program Chain (PGC) — the core navigation unit.
type ProgramChain struct {
	NrOfPrograms   uint8
	NrOfCells      uint8
	PlaybackTime   PlaybackTime
	ProhibitedOps  uint32
	AudioControl   [8]uint16
	SubpictureCtl  [32]uint32
	NextPGCN       uint16
	PrevPGCN       uint16
	GoUpPGCN       uint16
	StillTime      uint8
	PGPlaybackMode uint8
	Palette        [16][4]byte // YCbCr palette for SPU
	Programs       []ProgramInfo
	CellPlayback   []CellPlayback
	CellPosition   []CellPosition
	CommandTable   *DVDCommandTable // nil = no commands
}

// CellPlayback describes a single cell within a PGC (24 bytes on disc).
type CellPlayback struct {
	BlockMode           uint8
	BlockType           uint8
	StillTime           uint8
	CommandNr           uint8
	PlaybackTime        PlaybackTime
	FirstSector         uint32
	FirstILVUEndSector  uint32
	LastVOBUStartSector uint32
	LastSector          uint32
}

// CellPosition maps a cell to its VOB ID and cell ID within the VOB.
type CellPosition struct {
	VOBID  uint16
	CellID uint8
}

// ProgramInfo records the first cell of each program within a PGC.
type ProgramInfo struct {
	EntryCell uint8 // 1-based index of the first cell in this program
}
