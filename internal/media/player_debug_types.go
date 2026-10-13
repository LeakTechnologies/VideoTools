package media

import "time"

// PlayerDebugStats describes decoder/reader observations, not the DAC playhead.
type PlayerDebugStats struct {
	At                                                                            time.Time
	Session                                                                       uint64
	Playing                                                                       bool
	Clock, VideoPTS, AudioPTS                                                     float64
	VideoPackets, AudioPackets, Frames, PCMChunks, OutputBytes                    int
	AudioRMS, AudioPeak                                                           float64
	AudioReads, SilentReads, Underruns, EncodedAudioDrops, ClockSnaps, VideoDrops uint64
	FrameReadMS                                                                   float64
}
