//go:build native_media

package media

/*
#cgo pkg-config: libavcodec libavformat
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
*/
import "C"
import (
	"sync"
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
	return &PacketQueue{
		maxSize: DefaultMaxQueueSize,
	}
}

func NewPacketQueueWithMaxSize(maxSize int) *PacketQueue {
	if maxSize <= 0 {
		maxSize = DefaultMaxQueueSize
	}
	return &PacketQueue{
		maxSize: maxSize,
	}
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
