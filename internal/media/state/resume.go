//go:build native_media

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type PlaybackPosition struct {
	Path        string    `json:"path"`
	Position    float64   `json:"position"`
	Duration    float64   `json:"duration"`
	LastWatched time.Time `json:"lastWatched"`
	Completed   bool      `json:"completed"`
	Volume      float64   `json:"volume,omitempty"`
	Speed       float64   `json:"speed,omitempty"`
}

type ResumeState struct {
	positions map[string]*PlaybackPosition
	mu        sync.RWMutex
	filePath  string
}

func NewResumeState(configDir string) (*ResumeState, error) {
	s := &ResumeState{
		positions: make(map[string]*PlaybackPosition),
		filePath:  filepath.Join(configDir, "playback_state.json"),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *ResumeState) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read resume state: %w", err)
	}

	var positions map[string]*PlaybackPosition
	if err := json.Unmarshal(data, &positions); err != nil {
		return fmt.Errorf("failed to parse resume state: %w", err)
	}

	s.positions = positions
	return nil
}

func (s *ResumeState) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.positions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resume state: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write resume state: %w", err)
	}

	return nil
}

func (s *ResumeState) SavePosition(path string, position, duration float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedPath := filepath.Clean(path)

	pos := &PlaybackPosition{
		Path:        normalizedPath,
		Position:    position,
		Duration:    duration,
		LastWatched: time.Now(),
		Completed:   position >= duration*0.98,
	}

	if existing, ok := s.positions[normalizedPath]; ok {
		pos.Volume = existing.Volume
		pos.Speed = existing.Speed
	}

	s.positions[normalizedPath] = pos

	go s.save()
	return nil
}

func (s *ResumeState) SaveSettings(path string, volume, speed float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedPath := filepath.Clean(path)

	if pos, ok := s.positions[normalizedPath]; ok {
		pos.Volume = volume
		pos.Speed = speed
		go s.save()
	}

	return nil
}

func (s *ResumeState) GetPosition(path string) (*PlaybackPosition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedPath := filepath.Clean(path)
	pos, ok := s.positions[normalizedPath]

	if !ok {
		return nil, false
	}

	if pos.Completed || pos.Position < pos.Duration*0.02 {
		return nil, false
	}

	if time.Since(pos.LastWatched) > 30*24*time.Hour {
		return nil, false
	}

	return pos, true
}

func (s *ResumeState) MarkCompleted(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedPath := filepath.Clean(path)

	if pos, ok := s.positions[normalizedPath]; ok {
		pos.Completed = true
		go s.save()
	}

	return nil
}

func (s *ResumeState) ClearPosition(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedPath := filepath.Clean(path)
	delete(s.positions, normalizedPath)

	go s.save()
	return nil
}

func (s *ResumeState) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.positions = make(map[string]*PlaybackPosition)
	return s.save()
}

func (s *ResumeState) GetRecentPositions(limit int) []*PlaybackPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	positions := make([]*PlaybackPosition, 0, len(s.positions))
	for _, pos := range s.positions {
		if !pos.Completed && time.Since(pos.LastWatched) <= 30*24*time.Hour {
			positions = append(positions, pos)
		}
	}

	sort.Slice(positions, func(i, j int) bool {
		return positions[j].LastWatched.Before(positions[i].LastWatched)
	})

	if limit > 0 && len(positions) > limit {
		positions = positions[:limit]
	}

	return positions
}

func (s *ResumeState) ShouldResume(pos *PlaybackPosition) bool {
	if pos == nil {
		return false
	}

	if pos.Completed {
		return false
	}

	if pos.Position < pos.Duration*0.02 {
		return false
	}

	percentRemaining := 1.0 - (pos.Position / pos.Duration)
	if percentRemaining < 0.05 {
		return false
	}

	if time.Since(pos.LastWatched) > 7*24*time.Hour {
		return false
	}

	return true
}
