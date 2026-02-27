package engine

import (
	"encoding/json"
	"os"
)

func (m *MemoryEngine) SaveToFile(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 对于全量持久化，我们持有全局读锁以确保导出的 map 结构一致性
	// 细粒度的 space 锁在 Marshal 期间会被间接尊重（作为数据快照）
	data, err := json.Marshal(m.spaces)
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

	for _, space := range spaces {
		space.RebuildIndex()
	}

	m.mu.Lock()
	m.spaces = spaces
	m.mu.Unlock()

	return nil
}
