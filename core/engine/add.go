package engine

import (
	"context"
	"simplemem/core/compression"
	"simplemem/core/index"
	"simplemem/core/utils"
	"simplemem/core/vectorstore"
	"strings"
	"time"

	"encoding/json"

	"github.com/coder/hnsw"
	"github.com/google/uuid"
)

func (m *MemoryEngine) Add(ctx context.Context, namespace, content string, importance float64) error {
	if namespace == "" {
		namespace = "default"
	}

	if importance == 0 && m.estimator != nil {
		imp, err := m.estimator.Estimate(ctx, content)
		if err == nil {
			importance = imp
		}
	}

	embedding, err := m.embedder.Embed(ctx, content)
	if err != nil {
		return err
	}

	now := time.Now()

	item := &MemoryItem{
		ID:             uuid.New().String(),
		Content:        content,
		Embedding:      embedding,
		Importance:     importance,
		CreatedAt:      now,
		LastAccessedAt: now,
		Metadata: index.Metadata{
			Timestamp: now,
			Extra:     make(map[string]string),
		},
	}

	// 1. 获取/创建空间（使用引擎锁保护 map）
	m.mu.Lock()
	space := m.getOrCreateSpace(namespace)
	m.mu.Unlock()

	// 2. 空间内部操作（使用细粒度锁）
	space.mu.Lock()
	defer space.mu.Unlock()

	if importance >= m.config.LongTermImportanceThreshold {
		space.LongTerm = append(space.LongTerm, item)

		space.longIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		space.longBM25.Add(item.ID, item.Content)
		space.longMetadata.Add(item.ID, item.Metadata)

		// 同步到外部存储
		if m.store != nil {
			payload := m.memoryItemToPayload(item, namespace)
			m.store.Add(ctx, []vectorstore.VectorRecord{
				{
					ID:      item.ID,
					Vector:  utils.ToFloat32(item.Embedding),
					Payload: payload,
				},
			})
		}
	} else {
		space.ShortTerm = append(space.ShortTerm, item)

		space.shortIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		space.shortBM25.Add(item.ID, item.Content)
		space.shortMetadata.Add(item.ID, item.Metadata)
	}

	if len(space.ShortTerm) >= m.config.CompactionThreshold && !space.IsCompacting {
		// 异步执行压缩，避免阻塞主流程
		space.IsCompacting = true
		go func(s *MemorySpace) {
			m.compact(ctx, s)
			s.mu.Lock()
			s.IsCompacting = false
			s.mu.Unlock()
		}(space)
	}

	return nil
}

func (m *MemoryEngine) getOrCreateSpace(ns string) *MemorySpace {
	// 调用此方法前需保证 m.mu 已锁定
	space, ok := m.spaces[ns]
	if !ok {
		space = &MemorySpace{}
		space.Init()
		m.spaces[ns] = space
	}
	return space
}

func (m *MemoryEngine) AddDialogue(ctx context.Context, namespace, speaker, content string, timestamp time.Time) error {
	dialogues := []compression.Dialogue{
		{ID: uuid.New().String(), Speaker: speaker, Content: content, Timestamp: timestamp},
	}
	return m.AddDialogues(ctx, namespace, dialogues)
}

func (m *MemoryEngine) AddDialogues(ctx context.Context, namespace string, dialogues []compression.Dialogue) error {
	if namespace == "" {
		namespace = "default"
	}

	if m.compressor == nil {
		for _, d := range dialogues {
			if err := m.Add(ctx, namespace, d.Content, 0); err != nil {
				return err
			}
		}
		return nil
	}

	units, err := m.compressor.ProcessDialogues(ctx, dialogues)
	if err != nil {
		return err
	}

	for _, unit := range units {

		importance := unit.Importance
		if importance == 0 && m.estimator != nil {
			imp, err := m.estimator.Estimate(ctx, unit.Content)
			if err == nil {
				importance = imp
			}
		}

		if importance == 0 {
			switch unit.Salience {
			case "high":
				importance = 0.7
			case "medium":
				importance = 0.4
			case "low":
				importance = 0.2
			}
		}

		embedding, err := m.embedder.Embed(ctx, unit.Content)
		if err != nil {
			continue
		}

		createdAt := time.Now()
		if unit.Timestamp != nil {
			createdAt = *unit.Timestamp
		}

		item := &MemoryItem{
			ID:              unit.ID,
			Content:         unit.Content,
			OriginalContent: unit.OriginalContent,
			Embedding:       embedding,
			Importance:      importance,
			CreatedAt:       createdAt,
			LastAccessedAt:  createdAt,
			Metadata: index.Metadata{
				Entities:  unit.Entities,
				Topic:     unit.Topic,
				Timestamp: createdAt,
				Tags:      []string{unit.Salience},
				Extra:     map[string]string{"keywords": strings.Join(unit.Keywords, ",")},
			},
		}

		// 1. 获取空间
		m.mu.Lock()
		space := m.getOrCreateSpace(namespace)
		m.mu.Unlock()

		// 2. 写入空间
		space.mu.Lock()

		if importance >= m.config.LongTermImportanceThreshold {
			space.LongTerm = append(space.LongTerm, item)

			space.longIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
			space.longBM25.Add(item.ID, item.Content)
			space.longMetadata.Add(item.ID, item.Metadata)

			// 同步到外部存储
			if m.store != nil {
				payload := m.memoryItemToPayload(item, namespace)
				m.store.Add(ctx, []vectorstore.VectorRecord{
					{
						ID:      item.ID,
						Vector:  utils.ToFloat32(item.Embedding),
						Payload: payload,
					},
				})
			}
		} else {
			space.ShortTerm = append(space.ShortTerm, item)

			space.shortIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
			space.shortBM25.Add(item.ID, item.Content)
			space.shortMetadata.Add(item.ID, item.Metadata)
		}

		if len(space.ShortTerm) >= m.config.CompactionThreshold && !space.IsCompacting {
			space.IsCompacting = true
			go func(s *MemorySpace) {
				m.compact(ctx, s)
				s.mu.Lock()
				s.IsCompacting = false
				s.mu.Unlock()
			}(space)
		}

		space.mu.Unlock()
	}

	return nil
}

func (m *MemoryEngine) memoryItemToPayload(item *MemoryItem, namespace string) map[string]interface{} {
	// 将整个 item 序列化为 JSON 以便完整恢复
	data, _ := json.Marshal(item)
	return map[string]interface{}{
		"item_json":  string(data),
		"content":    item.Content, // 冗余一份用于数据库端预览或简单过滤
		"namespace":  namespace,
		"importance": item.Importance,
		"timestamp":  item.CreatedAt.Unix(),
	}
}
