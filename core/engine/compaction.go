package engine

import (
	"context"
	"simplemem/core/index"
	"simplemem/core/utils"

	"github.com/coder/hnsw"
)

func (m *MemoryEngine) compact(ctx context.Context, space *MemorySpace) {
	if m.summarizer == nil {
		return
	}

	clusters := m.clusterWithTemporalAffinity(space.ShortTerm)

	for _, cluster := range clusters {

		if len(cluster) < 2 {
			continue
		}

		var texts []string
		var maxImportance float64
		var latestTime = cluster[0].CreatedAt
		var allEntities []string
		var topics []string

		for _, mem := range cluster {
			texts = append(texts, mem.Content)
			if mem.Importance > maxImportance {
				maxImportance = mem.Importance
			}
			if mem.CreatedAt.After(latestTime) {
				latestTime = mem.CreatedAt
			}

			allEntities = append(allEntities, mem.Metadata.Entities...)
			if mem.Metadata.Topic != "" {
				topics = append(topics, mem.Metadata.Topic)
			}
		}

		summary, err := m.summarizer.Summarize(ctx, texts)
		if err != nil {
			continue
		}

		embedding, err := m.embedder.Embed(ctx, summary)
		if err != nil {
			continue
		}

		newMem := &MemoryItem{
			ID:             cluster[0].ID,
			Content:        summary,
			Embedding:      embedding,
			Importance:     maxImportance,
			CreatedAt:      latestTime,
			LastAccessedAt: latestTime,
			Metadata: index.Metadata{
				Entities:  utils.UniqueStrings(allEntities),
				Topic:     selectMostCommonTopic(topics),
				Timestamp: latestTime,
				Tags:      []string{"consolidated"},
				Extra:     make(map[string]string),
			},
		}

		space.LongTerm = append(space.LongTerm, newMem)

		space.longIndex.Add(hnsw.MakeNode(newMem.ID, utils.ToFloat32(newMem.Embedding)))
		space.longBM25.Add(newMem.ID, newMem.Content)
		space.longMetadata.Add(newMem.ID, newMem.Metadata)

		space.Archived = append(space.Archived, cluster...)
	}

	space.ShortTerm = nil

	space.shortIndex = hnsw.NewGraph[string]()
	space.shortBM25 = index.NewBM25Index()
	space.shortMetadata = index.NewMetadataIndex()
}

func (m *MemoryEngine) cluster(memories []*MemoryItem) [][]*MemoryItem {
	var clusters [][]*MemoryItem

	for _, mem := range memories {
		added := false
		for i, cluster := range clusters {
			if utils.Cosine(mem.Embedding, cluster[0].Embedding) > m.config.MergeSimilarityThreshold {
				clusters[i] = append(clusters[i], mem)
				added = true
				break
			}
		}
		if !added {
			clusters = append(clusters, []*MemoryItem{mem})
		}
	}

	return clusters
}

func (m *MemoryEngine) clusterWithTemporalAffinity(memories []*MemoryItem) [][]*MemoryItem {
	if len(memories) == 0 {
		return nil
	}

	const (
		beta   = 0.7
		lambda = 0.01
	)

	var clusters [][]*MemoryItem

	for _, mem := range memories {
		bestClusterIdx := -1
		bestAffinity := m.config.MergeSimilarityThreshold

		for i, cluster := range clusters {

			representative := cluster[0]

			semanticSim := utils.Cosine(mem.Embedding, representative.Embedding)

			timeDiff := mem.CreatedAt.Sub(representative.CreatedAt).Seconds()
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			temporalSim := utils.ExpDecay(lambda, timeDiff)

			affinity := beta*semanticSim + (1-beta)*temporalSim

			if affinity > bestAffinity {
				bestAffinity = affinity
				bestClusterIdx = i
			}
		}

		if bestClusterIdx >= 0 {
			clusters[bestClusterIdx] = append(clusters[bestClusterIdx], mem)
		} else {
			clusters = append(clusters, []*MemoryItem{mem})
		}
	}

	return clusters
}

func selectMostCommonTopic(topics []string) string {
	if len(topics) == 0 {
		return ""
	}

	freq := make(map[string]int)
	for _, topic := range topics {
		freq[topic]++
	}

	maxCount := 0
	mostCommon := ""
	for topic, count := range freq {
		if count > maxCount {
			maxCount = count
			mostCommon = topic
		}
	}

	return mostCommon
}
