package state

import "sync"

// StateManager coordinates Convert UI state updates without direct widget coupling.
// Callbacks are registered by UI code to keep widgets in sync.
type StateManager struct {
	mu                  sync.RWMutex
	quality             string
	bitrateMode         string
	manualQualityOption string
	onQualityChange     []func(string)
	onBitrateModeChange []func(string)
}

func NewStateManager(quality, bitrateMode, manualQualityOption string) *StateManager {
	if manualQualityOption == "" {
		manualQualityOption = "Manual (CRF)"
	}
	return &StateManager{
		quality:             quality,
		bitrateMode:         bitrateMode,
		manualQualityOption: manualQualityOption,
	}
}

func (m *StateManager) Quality() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.quality
}

func (m *StateManager) BitrateMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bitrateMode
}

func (m *StateManager) ManualQualityOption() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manualQualityOption
}

func (m *StateManager) SetQuality(val string) bool {
	m.mu.Lock()
	if m.quality == val {
		m.mu.Unlock()
		return false
	}
	m.quality = val
	callbacks := append([]func(string){}, m.onQualityChange...)
	m.mu.Unlock()

	for _, cb := range callbacks {
		cb(val)
	}
	return true
}

func (m *StateManager) SetBitrateMode(val string) bool {
	m.mu.Lock()
	if m.bitrateMode == val {
		m.mu.Unlock()
		return false
	}
	m.bitrateMode = val
	callbacks := append([]func(string){}, m.onBitrateModeChange...)
	m.mu.Unlock()

	for _, cb := range callbacks {
		cb(val)
	}
	return true
}

func (m *StateManager) OnQualityChange(fn func(string)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	m.onQualityChange = append(m.onQualityChange, fn)
	m.mu.Unlock()
}

func (m *StateManager) OnBitrateModeChange(fn func(string)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	m.onBitrateModeChange = append(m.onBitrateModeChange, fn)
	m.mu.Unlock()
}

func (m *StateManager) IsManualQuality(val string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val != "" {
		return val == m.manualQualityOption
	}
	return m.quality == m.manualQualityOption
}
