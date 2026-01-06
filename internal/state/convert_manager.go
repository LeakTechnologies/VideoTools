package state

import (
	"strings"
	"sync"
)

// ConvertManager centralizes convert UI state and change notifications.
// It is intentionally UI-agnostic; widgets should subscribe via callbacks.
type ConvertManager struct {
	mu                  sync.RWMutex
	quality             string
	bitrateMode         string
	manualQualityOption string
	onQualityChange     []func(string)
	onBitrateModeChange []func(string)
}

func NewConvertManager(quality, bitrateMode, manualQualityOption string) *ConvertManager {
	if strings.TrimSpace(manualQualityOption) == "" {
		manualQualityOption = "Manual (CRF)"
	}
	return &ConvertManager{
		quality:             quality,
		bitrateMode:         bitrateMode,
		manualQualityOption: manualQualityOption,
	}
}

func (m *ConvertManager) Quality() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.quality
}

func (m *ConvertManager) BitrateMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bitrateMode
}

func (m *ConvertManager) ManualQualityOption() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manualQualityOption
}

func (m *ConvertManager) SetQuality(val string) bool {
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

func (m *ConvertManager) SetBitrateMode(val string) bool {
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

func (m *ConvertManager) OnQualityChange(fn func(string)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	m.onQualityChange = append(m.onQualityChange, fn)
	m.mu.Unlock()
}

func (m *ConvertManager) OnBitrateModeChange(fn func(string)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	m.onBitrateModeChange = append(m.onBitrateModeChange, fn)
	m.mu.Unlock()
}

func (m *ConvertManager) IsManualQuality() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.quality == m.manualQualityOption
}

func NormalizeBitrateMode(mode string) string {
	switch {
	case strings.HasPrefix(mode, "CRF"):
		return "CRF"
	case strings.HasPrefix(mode, "CBR"):
		return "CBR"
	case strings.HasPrefix(mode, "VBR"):
		return "VBR"
	case strings.HasPrefix(mode, "Target Size"):
		return "Target Size"
	default:
		return mode
	}
}
