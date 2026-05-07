// Package recentfiles tracks and persists the list of recently opened files
// across all modules. Entries are stored in VideoTools/recent_files.json.
package recentfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

	const (
	maxEntries  = 15
	storageName = "recent_files"
)

// cat is the logging category for recent files operations.
// Uses CatUI since this is a UI-facing feature.
func cat() logging.Category {
	return logging.CatUI
}

// Entry is one record in the recent-files list.
type Entry struct {
	Path        string    `json:"path"`
	DisplayName string    `json:"displayName"`
	Module      string    `json:"module"`
	OpenedAt    time.Time `json:"openedAt"`
}

// Manager holds the in-memory list and handles load/save.
type Manager struct {
	mu      sync.Mutex
	entries []Entry
}

// New returns a Manager with entries loaded from disk.
// Errors are logged but not fatal — the manager starts empty on failure.
func New() *Manager {
	m := &Manager{}
	if err := m.load(); err != nil && !os.IsNotExist(err) {
		logging.Debug(cat(), "recentfiles: load error (ignored): %v", err)
	}
	return m
}

// Add inserts or bumps a file to the front of the list and persists.
// If the path already exists for that module, the existing entry is removed
// and re-added at the front so duplicates never appear.
func (m *Manager) Add(path, displayName, module string) {
	if path == "" {
		return
	}
	if displayName == "" {
		displayName = filepath.Base(path)
	}

	m.mu.Lock()
	// Remove any existing entry for this path+module combo.
	filtered := m.entries[:0]
	for _, e := range m.entries {
		if !(e.Path == path && e.Module == module) {
			filtered = append(filtered, e)
		}
	}
	// Prepend the new entry.
	entry := Entry{
		Path:        path,
		DisplayName: displayName,
		Module:      module,
		OpenedAt:    time.Now(),
	}
	m.entries = append([]Entry{entry}, filtered...)
	// Trim to cap.
	if len(m.entries) > maxEntries {
		m.entries = m.entries[:maxEntries]
	}
	snapshot := make([]Entry, len(m.entries))
	copy(snapshot, m.entries)
	m.mu.Unlock()

	go func() {
		if err := saveEntries(snapshot); err != nil {
			logging.Debug(cat(), "recentfiles: save error: %v", err)
		}
	}()
}

// Entries returns a copy of the current list, newest first.
func (m *Manager) Entries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// Remove deletes one entry by path+module and persists.
func (m *Manager) Remove(path, module string) {
	m.mu.Lock()
	filtered := m.entries[:0]
	for _, e := range m.entries {
		if !(e.Path == path && e.Module == module) {
			filtered = append(filtered, e)
		}
	}
	m.entries = filtered
	snapshot := make([]Entry, len(m.entries))
	copy(snapshot, m.entries)
	m.mu.Unlock()

	go func() {
		if err := saveEntries(snapshot); err != nil {
			logging.Debug(cat(), "recentfiles: save error: %v", err)
		}
	}()
}

// Clear removes all entries and persists.
func (m *Manager) Clear() {
	m.mu.Lock()
	m.entries = nil
	m.mu.Unlock()
	go func() {
		_ = saveEntries(nil)
	}()
}

// --- private helpers ---

func storagePath() string {
	return configpath.ModuleConfigPath(storageName)
}

func (m *Manager) load() error {
	data, err := os.ReadFile(storagePath())
	if err != nil {
		return err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	m.entries = entries
	return nil
}

func saveEntries(entries []Entry) error {
	path := storagePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
