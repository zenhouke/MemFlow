package vectorstore

import (
	"context"
	"fmt"
	"math"
	"simplemem/core/config"
	"sort"
	"sync"
)

func init() {
	Register("memory", func(cfg config.VectorStoreConfig) (VectorStore, error) {
		return NewMemory(cfg)
	})
}
func NewMemory(cfg config.VectorStoreConfig) (*MemoryStore, error) {
	return &MemoryStore{
		records:   make(map[string]VectorRecord),
		dimension: cfg.Dimension,
	}, nil
}
type MemoryStore struct {
	mu        sync.RWMutex
	records   map[string]VectorRecord
	dimension int
}

func (m *MemoryStore) Add(ctx context.Context, vectors []VectorRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range vectors {
		if len(v.Vector) != m.dimension {
			return fmt.Errorf("vector %s has dimension %d, expected %d", v.ID, len(v.Vector), m.dimension)
		}
		if _, exists := m.records[v.ID]; exists {
			return fmt.Errorf("vector %s already exists, use Upsert to update", v.ID)
		}
		m.records[v.ID] = v
	}
	return nil
}

func (m *MemoryStore) Upsert(ctx context.Context, vectors []VectorRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range vectors {
		if len(v.Vector) != m.dimension {
			return fmt.Errorf("vector %s has dimension %d, expected %d", v.ID, len(v.Vector), m.dimension)
		}
		m.records[v.ID] = v
	}
	return nil
}

func (m *MemoryStore) Search(ctx context.Context, query []float32, topK int, filter *Filter) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(query) != m.dimension {
		return nil, fmt.Errorf("query has dimension %d, expected %d", len(query), m.dimension)
	}

	var results []SearchResult

	for _, record := range m.records {
		if filter != nil && !matchFilter(record, filter) {
			continue
		}

		score := cosineSimilarity(query, record.Vector)
		results = append(results, SearchResult{
			ID:      record.ID,
			Score:   score,
			Payload: record.Payload,
		})
	}

	// 按分数降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func (m *MemoryStore) Delete(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		delete(m.records, id)
	}
	return nil
}

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = nil
	return nil
}

// ========== 辅助函数 ==========

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func matchFilter(record VectorRecord, filter *Filter) bool {
	// Must 条件：全部满足
	for _, cond := range filter.Must {
		if !matchCondition(record, cond) {
			return false
		}
	}

	// Should 条件：至少满足一个（如果有的话）
	if len(filter.Should) > 0 {
		matched := false
		for _, cond := range filter.Should {
			if matchCondition(record, cond) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func matchCondition(record VectorRecord, cond Condition) bool {
	val, ok := record.Payload[cond.Field]
	if !ok {
		return false
	}

	switch cond.Operator {
	case OpEqual:
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", cond.Value)

	case OpIn:
		if values, ok := cond.Value.([]interface{}); ok {
			for _, v := range values {
				if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", v) {
					return true
				}
			}
		}
		return false

	case OpRange:
		if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
			floatVal, ok := toFloat64(val)
			if !ok {
				return false
			}
			if minVal, ok := rangeVal["gte"]; ok {
				if minFloat, ok := toFloat64(minVal); ok && floatVal < minFloat {
					return false
				}
			}
			if maxVal, ok := rangeVal["lte"]; ok {
				if maxFloat, ok := toFloat64(maxVal); ok && floatVal > maxFloat {
					return false
				}
			}
			return true
		}
		return false

	default:
		return false
	}
}

