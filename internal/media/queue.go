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

// PacketQueue is a thread-safe queue for AVPacket pointers.
type PacketQueue struct {
	packets []*C.AVPacket
	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
}

// NewPacketQueue creates a new packet queue.
func NewPacketQueue() *PacketQueue {
	q := &PacketQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Put adds a packet to the queue.
func (q *PacketQueue) Put(pkt *C.AVPacket) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Clone the packet to ensure the demuxer can reuse its local buffer
	cloned := C.av_packet_alloc()
	C.av_packet_ref(cloned, pkt)
	
	q.packets = append(q.packets, cloned)
	q.cond.Signal()
}

// Get retrieves a packet from the queue, blocking if empty.
func (q *PacketQueue) Get() (*C.AVPacket, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.packets) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed || len(q.packets) == 0 {
		return nil, false
	}

	pkt := q.packets[0]
	q.packets = q.packets[1:]
	return pkt, true
}

// Close closes the queue and releases all packets.
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

// Size returns the current number of packets in the queue.
func (q *PacketQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.packets)
}
