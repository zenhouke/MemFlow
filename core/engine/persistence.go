package engine

import (
	"encoding/json"
	"os"
)

func (m *MemoryEngine) SaveToFile(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.spaces, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *MemoryEngine) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var spaces map[string]*MemorySpace
	if err := json.Unmarshal(data, &spaces); err != nil {
		return err
	}

	m.mu.Lock()
	m.spaces = spaces
	m.mu.Unlock()

	return nil
}
