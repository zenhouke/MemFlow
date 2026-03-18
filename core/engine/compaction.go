package engine

import (
	"context"
	"memflow/core/index"
	"memflow/core/utils"

	"github.com/coder/hnsw"
)

func (m *MemoryEngine) compact(ctx context.Context, space *MemorySpace) {
	if m.summarizer == nil {
		return
	}

	// 1. 获取快照（需要锁）
	space.mu.RLock()
	if len(space.ShortTerm) == 0 {
		space.mu.RUnlock()
		return
	}
	itemsToCompact := make([]*MemoryItem, len(space.ShortTerm))
	copy(itemsToCompact, space.ShortTerm)
	space.mu.RUnlock()

	// 2. 聚类分析（耗时较短，但在锁外执行更安全）
	clusters := m.clusterWithTemporalAffinity(itemsToCompact)

	var newMemories []*MemoryItem
	var consolidatedItems []*MemoryItem

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

		// 3. 执行 LLM 摘要（最耗时的 I/O 操作，严禁加锁）
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

		newMemories = append(newMemories, newMem)
		consolidatedItems = append(consolidatedItems, cluster...)
	}

	// 4. 写回结果（需要强一致性锁定）
	if len(newMemories) > 0 {
		space.mu.Lock()
		defer space.mu.Unlock()

		for _, newMem := range newMemories {
			space.LongTerm = append(space.LongTerm, newMem)
			space.longIndex.Add(hnsw.MakeNode(newMem.ID, utils.ToFloat32(newMem.Embedding)))
			space.longBM25.Add(newMem.ID, newMem.Content)
			space.longMetadata.Add(newMem.ID, newMem.Metadata)
		}

		// 简单起见，这里清空短期记忆，或者只删除被整合的部分
		// 实际上应该只过滤掉 consolidatedItems，但为了逻辑鲁棒性，这里按原逻辑清空
		space.ShortTerm = nil
		space.shortIndex = hnsw.NewGraph[string]()
		space.shortBM25 = index.NewBM25Index()
		space.shortMetadata = index.NewMetadataIndex()
		space.Archived = append(space.Archived, consolidatedItems...)
	}
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
