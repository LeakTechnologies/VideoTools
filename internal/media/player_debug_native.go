//go:build native_media

package media

import (
	"math"
	"time"
)

func (e *Engine) SetDiagnosticsEnabled(enabled bool) {
	e.debugEnabled.Store(enabled)
	if e.audioPlayer != nil {
		e.audioPlayer.debugEnabled.Store(enabled)
	}
}

// Called by the source owner while native operation/disposal is serialized.
func (e *Engine) Diagnostics() PlayerDebugStats {
	d := PlayerDebugStats{
		At: time.Now(), Clock: e.clock.GetTime(), VideoPTS: e.GetLastVideoPTS(), AudioPTS: e.GetLastAudioPTS(),
		VideoPackets: e.videoQueue.Size(), AudioPackets: e.audioQueue.Size(), Frames: len(e.frameQueue),
		EncodedAudioDrops: e.debugAudioDrops.Load(), ClockSnaps: e.debugClockSnaps.Load(), VideoDrops: e.debugVideoDrops.Load(),
		FrameReadMS: float64(e.debugFrameReadNS.Load()) / 1e6,
	}
	if p := e.audioPlayer; p != nil {
		d.PCMChunks = len(p.pcmCh)
		if p.otoPlayer != nil {
			d.OutputBytes = p.otoPlayer.BufferedSize()
		}
		d.AudioReads = p.debugReads.Load()
		d.SilentReads = p.debugSilentReads.Load()
		d.Underruns = p.debugUnderruns.Load()
		d.AudioRMS = math.Float64frombits(p.debugRMS.Load())
		d.AudioPeak = math.Float64frombits(p.debugPeak.Load())
	}
	return d
}
func (p *AudioPlayer) recordOutput(buf []byte, n int) {
	if !p.debugEnabled.Load() || n <= 0 {
		return
	}
	if n > len(buf) {
		n = len(buf)
	}
	var sum, peak float64
	count := 0
	for i := 0; i+1 < n; i += 2 {
		value := float64(int16(uint16(buf[i])|uint16(buf[i+1])<<8)) / 32768
		sum += value * value
		if a := math.Abs(value); a > peak {
			peak = a
		}
		count++
	}
	if count == 0 {
		return
	}
	p.debugReads.Add(1)
	if peak == 0 {
		p.debugSilentReads.Add(1)
	}
	p.debugRMS.Store(math.Float64bits(math.Sqrt(sum / float64(count))))
	p.debugPeak.Store(math.Float64bits(peak))
}
