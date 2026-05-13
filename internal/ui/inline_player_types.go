package ui

import "time"

type LoadPhase int

const (
	LoadPhaseStarted    LoadPhase = iota
	LoadPhaseOpen
	LoadPhaseFirstFrame
	LoadPhaseReady
	LoadPhaseFailed
)

func (p LoadPhase) String() string {
	switch p {
	case LoadPhaseStarted:
		return "Starting"
	case LoadPhaseOpen:
		return "Engine open"
	case LoadPhaseFirstFrame:
		return "First frame"
	case LoadPhaseReady:
		return "Ready"
	case LoadPhaseFailed:
		return "Failed"
	}
	return "Unknown"
}

type LoadEvent struct {
	Phase LoadPhase
	At    time.Time
	Err   error
}
