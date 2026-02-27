package engine

import (
	"context"
	"simplemem/core/compression"
	"simplemem/core/index"
	"simplemem/core/utils"
	"strings"
	"time"

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

	m.mu.Lock()
	defer m.mu.Unlock()

	space := m.getOrCreateSpace(namespace)

	if importance >= m.config.LongTermImportanceThreshold {
		space.LongTerm = append(space.LongTerm, item)

		space.longIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		space.longBM25.Add(item.ID, item.Content)
		space.longMetadata.Add(item.ID, item.Metadata)
	} else {
		space.ShortTerm = append(space.ShortTerm, item)

		space.shortIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
		space.shortBM25.Add(item.ID, item.Content)
		space.shortMetadata.Add(item.ID, item.Metadata)
	}

	if len(space.ShortTerm) >= m.config.CompactionThreshold {
		m.compact(ctx, space)
	}

	return nil
}

func (m *MemoryEngine) getOrCreateSpace(ns string) *MemorySpace {
	space, ok := m.spaces[ns]
	if !ok {
		space = &MemorySpace{

			shortIndex: hnsw.NewGraph[string](),
			longIndex:  hnsw.NewGraph[string](),

			shortBM25: index.NewBM25Index(),
			longBM25:  index.NewBM25Index(),

			shortMetadata: index.NewMetadataIndex(),
			longMetadata:  index.NewMetadataIndex(),

			Archived: make([]*MemoryItem, 0),
		}
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

		importance := 0.0
		if m.estimator != nil {
			imp, err := m.estimator.Estimate(ctx, unit.Content)
			if err == nil {
				importance = imp
			}
		}

		switch unit.Salience {
		case "high":
			importance = max(importance, 0.7)
		case "medium":
			importance = max(importance, 0.4)
		case "low":
			importance = max(importance, 0.2)
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

		m.mu.Lock()
		space := m.getOrCreateSpace(namespace)

		if importance >= m.config.LongTermImportanceThreshold {
			space.LongTerm = append(space.LongTerm, item)

			space.longIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
			space.longBM25.Add(item.ID, item.Content)
			space.longMetadata.Add(item.ID, item.Metadata)
		} else {
			space.ShortTerm = append(space.ShortTerm, item)

			space.shortIndex.Add(hnsw.MakeNode(item.ID, utils.ToFloat32(item.Embedding)))
			space.shortBM25.Add(item.ID, item.Content)
			space.shortMetadata.Add(item.ID, item.Metadata)
		}

		if len(space.ShortTerm) >= m.config.CompactionThreshold {
			m.compact(ctx, space)
		}

		m.mu.Unlock()
	}

	return nil
}
