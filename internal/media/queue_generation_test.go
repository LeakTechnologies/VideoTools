//go:build native_media

package media

import (
	"testing"
	"time"
)

func TestPacketQueueRejectsReadBeforeSeekFlush(t *testing.T) {
	q := NewPacketQueue()
	old := q.Generation()
	q.Flush()
	// A stale packet must be rejected before it is cloned/dereferenced.
	q.PutGeneration(nil, old)
	if q.TryPutGeneration(nil, old) || q.Size() != 0 {
		t.Fatal("retired demux packet published")
	}
}

func TestPacketQueueFlushRetiresBlockedProducer(t *testing.T) {
	q := NewPacketQueueWithMaxSize(1)
	q.packets = append(q.packets, nil)
	old := q.Generation()
	done := make(chan struct{})
	go func() { q.PutGeneration(nil, old); close(done) }()
	q.Flush()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retired producer remained blocked")
	}
	if q.Size() != 0 {
		t.Fatal("old packet entered flushed queue")
	}
}

func TestAudioSpeedReturnToNormalResetsTempo(t *testing.T) {
	p := &AudioPlayer{speed: 1}
	p.SetSpeed(1.5)
	if p.filterGraph == nil {
		t.Fatal("tempo filter not initialized")
	}
	defer p.filterGraph.Release()
	p.SetSpeed(1)
	if p.filterGraph.GetTempo() != 1 {
		t.Fatal("returning to normal retained accelerated audio")
	}
}

func TestAudioReadRejectsFuturePCMFromRetiredSeek(t *testing.T) {
	q := NewPacketQueue()
	old := q.Generation()
	q.Flush()
	p := &AudioPlayer{speed: 1, volumeMul: 1, clock: NewMasterClock(), queue: q, pcmCh: make(chan audioChunk, 2)}
	p.pcmCh <- audioChunk{pts: 889, data: []byte{9, 9, 9, 9}, generation: old}
	p.pcmCh <- audioChunk{pts: 326, data: []byte{1, 2, 3, 4}, generation: q.Generation()}
	buf := make([]byte, 4)
	n, err := p.Read(buf)
	if err != nil || n != 4 || buf[0] != 1 || p.GetLastPTS() != 326 {
		t.Fatalf("stale PCM accepted: n=%d err=%v data=%v pts=%v", n, err, buf, p.GetLastPTS())
	}
}

func TestPacketQueueEOFResetAndRetirement(t *testing.T) {
	for _, mode := range []string{"seek", "stop", "close"} {
		t.Run(mode, func(t *testing.T) {
			q := NewPacketQueue()
			gen := q.Generation()
			q.SetEOF()
			stop := make(chan struct{})
			done := make(chan bool, 1)
			go func() { done <- q.WaitForReset(gen, stop) }()
			switch mode {
			case "seek":
				q.Flush()
			case "stop":
				close(stop)
			case "close":
				q.Close()
			}
			select {
			case resumed := <-done:
				if resumed != (mode == "seek") {
					t.Fatalf("resume=%v", resumed)
				}
			case <-time.After(time.Second):
				t.Fatal("EOF worker retained")
			}
		})
	}
}

func TestAudioTempoCommandsProcessAllAdvertisedRates(t *testing.T) {
	g := NewAudioFilterGraph()
	if err := g.Init(TargetSampleRate, TargetChannels); err != nil {
		t.Fatal(err)
	}
	defer g.Release()
	input := make([]byte, 4096)
	for _, rate := range []float64{1, 1.5, 1, 0.25, 0.5, 0.75, 1.25, 2, 1} {
		if err := g.SetTempo(rate); err != nil {
			t.Fatalf("rate %v: %v", rate, err)
		}
		produced := 0
		for i := 0; i < 32; i++ {
			out, err := g.Process(input)
			if err != nil {
				t.Fatal(err)
			}
			produced += len(out)
		}
		if produced == 0 {
			t.Fatalf("rate %v produced no audio", rate)
		}
	}
}
