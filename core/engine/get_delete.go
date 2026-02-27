package engine

import (
	"fmt"
)

func (m *MemoryEngine) Get(namespace string) ([]*MemoryItem, error) {
	if namespace == "" {
		namespace = "default"
	}

	m.mu.RLock()
	space, ok := m.spaces[namespace]
	m.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	space.mu.RLock()
	defer space.mu.RUnlock()

	var results []*MemoryItem
	totalItems := len(space.ShortTerm) + len(space.LongTerm) + len(space.Archived)
	results = make([]*MemoryItem, 0, totalItems)
	results = append(results, space.ShortTerm...)
	results = append(results, space.LongTerm...)
	results = append(results, space.Archived...)

	return results, nil
}

func (m *MemoryEngine) Delete(namespace, id string) error {
	if namespace == "" {
		namespace = "default"
	}

	m.mu.RLock()
	space, ok := m.spaces[namespace]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("namespace not found")
	}

	space.mu.Lock()
	defer space.mu.Unlock()

	// 从各个层级删除
	deleted := false
	for i, item := range space.ShortTerm {
		if item.ID == id {
			space.ShortTerm = append(space.ShortTerm[:i], space.ShortTerm[i+1:]...)
			space.shortIndex.Delete(id)
			space.shortBM25.Delete(id)
			space.shortMetadata.Delete(id)
			deleted = true
			break
		}
	}

	if !deleted {

		for i, item := range space.LongTerm {
			if item.ID == id {
				space.LongTerm = append(space.LongTerm[:i], space.LongTerm[i+1:]...)
				space.longIndex.Delete(id)
				space.longBM25.Delete(id)
				space.longMetadata.Delete(id)
				deleted = true
				break
			}
		}
	}

	if !deleted {

		for i, item := range space.Archived {
			if item.ID == id {
				space.Archived = append(space.Archived[:i], space.Archived[i+1:]...)

				deleted = true
				break
			}
		}
	}

	if !deleted {
		return fmt.Errorf("memory item %s not found in namespace %s", id, namespace)
	}

	return nil
}
