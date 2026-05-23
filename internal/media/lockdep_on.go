//go:build native_media && lockdep

package media

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// goroutine-local lock tracking.
// Each goroutine's held lock levels are stored as a bitmask in this sync.Map.
// Key: goroutine ID (uint64), value: int32 bitmask.
var lockState sync.Map

// goroutineID returns the numeric goroutine ID by parsing the stack header.
// Only used in lockdep builds — not performance-critical.
func goroutineID() uint64 {
	var buf [128]byte
	n := runtime.Stack(buf[:], false)
	s := string(buf[:n])
	// Format: "goroutine NNN [running]:"
	parts := strings.SplitN(s, " ", 3)
	if len(parts) < 2 {
		return 0
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func getHeldLockBits() int32 {
	gid := goroutineID()
	v, ok := lockState.Load(gid)
	if !ok {
		return 0
	}
	return v.(int32)
}

func setHeldLockBits(bits int32) {
	gid := goroutineID()
	if bits == 0 {
		lockState.Delete(gid)
	} else {
		lockState.Store(gid, bits)
	}
}

func levelName(level int) string {
	switch level {
	case lockLevelGeneral:
		return "mu"
	case lockLevelFormat:
		return "formatMu"
	case lockLevelCodec:
		return "videoCodecMu"
	case lockLevelFramepool:
		return "framepoolMu"
	default:
		return fmt.Sprintf("level(%d)", level)
	}
}

// acquired checks lock ordering and records the acquisition.
// Panics if the calling goroutine already holds a lock at level >= newLevel
// (reverse-order violation).
func (e *Engine) acquired(newLevel int) {
	held := getHeldLockBits()
	// Check each possible held level against newLevel.
	for lvl := lockLevelGeneral; lvl < newLevel; lvl++ {
		if held&(1<<lvl) != 0 {
			heldName := levelName(lvl)
			newName := levelName(newLevel)
			panic(fmt.Sprintf(
				"LOCKDEP VIOLATION: goroutine %d holds %s (level %d) and "+
					"acquired %s (level %d) — reverse-order deadlock risk. "+
					"Lock hierarchy: mu(1) → formatMu(2) → videoCodecMu(3) → framepoolMu(4)",
				goroutineID(), heldName, lvl, newName, newLevel,
			))
		}
	}
	// Record the new lock as held.
	setHeldLockBits(held | (1 << newLevel))
}

// released removes the lock level from this goroutine's held set.
func (e *Engine) released(level int) {
	held := getHeldLockBits()
	setHeldLockBits(held & ^(1 << level))
}
