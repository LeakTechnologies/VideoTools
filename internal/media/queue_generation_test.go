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
