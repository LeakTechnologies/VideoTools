//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
*/
import "C"
import (
	"sync"
	"time"
)

const (
	DefaultMaxQueueSize = 250
)

type PacketQueue struct {
	packets []*C.AVPacket
	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
	eof     bool
	maxSize int
}

func NewPacketQueue() *PacketQueue {
	q := &PacketQueue{maxSize: DefaultMaxQueueSize}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func NewPacketQueueWithMaxSize(maxSize int) *PacketQueue {
	if maxSize <= 0 {
		maxSize = DefaultMaxQueueSize
	}
	q := &PacketQueue{maxSize: maxSize}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *PacketQueue) Put(pkt *C.AVPacket) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for !q.closed && !q.eof && len(q.packets) >= q.maxSize {
		q.cond.Wait()
	}

	if q.closed || q.eof {
		return
	}

	cloned := C.av_packet_alloc()
	C.av_packet_ref(cloned, pkt)

	q.packets = append(q.packets, cloned)
	q.cond.Signal()
}

func (q *PacketQueue) Get() (*C.AVPacket, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.packets) == 0 && !q.closed && !q.eof {
		q.cond.Wait()
	}

	if len(q.packets) == 0 {
		if q.eof {
			return nil, false
		}
		if q.closed {
			return nil, false
		}
	}

	pkt := q.packets[0]
	q.packets = q.packets[1:]
	q.cond.Signal()
	return pkt, true
}

func (q *PacketQueue) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, pkt := range q.packets {
		C.av_packet_free(&pkt)
	}
	q.packets = nil
	q.eof = false
	q.cond.Broadcast()
}

func (q *PacketQueue) SetEOF() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.eof = true
	q.cond.Signal()
}

func (q *PacketQueue) IsEOF() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.eof
}

func (q *PacketQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	for _, pkt := range q.packets {
		C.av_packet_free(&pkt)
	}
	q.packets = nil
	q.cond.Broadcast()
}

func (q *PacketQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.packets)
}

func (q *PacketQueue) MaxSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxSize
}

func (q *PacketQueue) SetMaxSize(maxSize int) {
	if maxSize <= 0 {
		maxSize = DefaultMaxQueueSize
	}
	q.mu.Lock()
	q.maxSize = maxSize
	q.mu.Unlock()
}

func (q *PacketQueue) IsFull() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.packets) >= q.maxSize
}

func (q *PacketQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// TryPut enqueues pkt if the queue is below its limit, and returns true.
// If the queue is already at capacity the packet is NOT enqueued and the
// caller must free it; returns false. Never blocks.
func (q *PacketQueue) TryPut(pkt *C.AVPacket) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed || q.eof || len(q.packets) >= q.maxSize {
		return false
	}

	cloned := C.av_packet_alloc()
	C.av_packet_ref(cloned, pkt)
	q.packets = append(q.packets, cloned)
	q.cond.Signal()
	return true
}

// TryGet returns a packet if one is available, without blocking.
// Returns (nil, false) immediately when the queue is empty.
func (q *PacketQueue) TryGet() (*C.AVPacket, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.packets) == 0 {
		return nil, false
	}

	pkt := q.packets[0]
	q.packets = q.packets[1:]
	q.cond.Signal()
	return pkt, true
}

// IsClosedOrEOF returns true when the queue is drained and no more packets will arrive.
func (q *PacketQueue) IsClosedOrEOF() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return (q.closed || q.eof) && len(q.packets) == 0
}

// TimedGet blocks until a packet is available, the queue closes/reaches EOF,
// or the timeout elapses.  Returns (nil, false) on timeout or drain.
// Unlike TryGet it does not spin — the caller's goroutine sleeps on the cond
// and wakes immediately when TryPut/Put signals a new packet.
func (q *PacketQueue) TimedGet(d time.Duration) (*C.AVPacket, bool) {
	deadline := time.Now().Add(d)
	// Timer fires after d and broadcasts to wake any cond.Wait() callers.
	timer := time.AfterFunc(d, func() { q.cond.Broadcast() })
	defer timer.Stop()

	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.packets) == 0 && !q.closed && !q.eof {
		if time.Now().After(deadline) {
			return nil, false
		}
		q.cond.Wait()
	}
	if len(q.packets) == 0 {
		return nil, false
	}
	pkt := q.packets[0]
	q.packets = q.packets[1:]
	q.cond.Signal()
	return pkt, true
}
