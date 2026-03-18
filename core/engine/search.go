package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"memflow/core/config"
	"memflow/core/index"
	"memflow/core/llm"
	"memflow/core/retrieval"
	"memflow/core/utils"
	"sort"
	"strings"
	"time"
)

func (m *MemoryEngine) Search(ctx context.Context, namespace, query string) ([]*MemoryItem, error) {
	if namespace == "" {
		namespace = "default"
	}

	queryEmbedding, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	now := m.nowFn()

	m.mu.RLock()
	space, ok := m.getSpace(namespace)
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		space = m.getOrCreateSpace(namespace)
		m.mu.Unlock()
	}

	space.mu.Lock()
	defer space.mu.Unlock()

	if m.config.EnableHybridSearch {
		return m.hybridSearch(ctx, query, queryEmbedding, space, now)
	}

	return m.simpleSearch(ctx, queryEmbedding, space, now)
}

func (m *MemoryEngine) hybridSearch(
	ctx context.Context,
	query string,
	queryEmbedding []float64,
	space *MemorySpace,
	now time.Time,
) ([]*MemoryItem, error) {
	if m.testHybridSearchOverride != nil {
		return m.testHybridSearchOverride(ctx, query, queryEmbedding, space, now)
	}

	analyzer := retrieval.NewQueryAnalyzer(m.llmClient, m.config.HybridSearchConfig.BaseK)

	retriever := NewHybridRetriever(m.config.HybridSearchConfig, analyzer)

	results, err := retriever.Search(
		ctx,
		query,
		queryEmbedding,
		space,
		now,
		m.config.ShortTermDecay,
	)

	return results, err
}

func (m *MemoryEngine) simpleSearch(
	ctx context.Context, // 新增 context
	queryEmbedding []float64,
	space *MemorySpace,
	now time.Time,
) ([]*MemoryItem, error) {

	results := m.collectAndScore(ctx, now, queryEmbedding, space)

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	topK := m.config.TopK
	if topK > len(results) {
		topK = len(results)
	}

	final := make([]*MemoryItem, topK)

	for i := 0; i < topK; i++ {
		final[i] = results[i].item
		final[i].LastAccessedAt = now
	}

	return final, nil
}

type scored struct {
	item  *MemoryItem
	score float64
}

func (m *MemoryEngine) collectAndScore(ctx context.Context, now time.Time, q []float64, space *MemorySpace) []scored {
	// 注意：调用此方法前，外部应已对 space.mu 加 RLock
	var results []scored
	q32 := utils.ToFloat32(q)
	seenIDs := make(map[string]bool)

	// 1. 内存短期记忆检索
	if len(space.ShortTerm) > 0 && space.shortIndex != nil {
		shortNeighbors := space.shortIndex.Search(q32, m.config.TopK)
		for _, vec := range shortNeighbors {
			id := vec.Key
			mem := m.findMemoryItemByID(space.ShortTerm, id)
			if mem != nil {
				score := m.score(now, q, mem, m.config.ShortTermDecay)
				results = append(results, scored{mem, score})
				seenIDs[id] = true
			}
		}
	}

	// 2. 内存长期记忆检索
	if len(space.LongTerm) > 0 && space.longIndex != nil {
		longNeighbors := space.longIndex.Search(q32, m.config.TopK)
		for _, vec := range longNeighbors {
			id := vec.Key
			if seenIDs[id] {
				continue // 避免同一 item 在短/长期重复出现
			}
			mem := m.findMemoryItemByID(space.LongTerm, id)
			if mem != nil {
				score := m.score(now, q, mem, m.config.LongTermDecay)
				results = append(results, scored{mem, score})
				seenIDs[id] = true
			}
		}
	}

	// 3. 外部存储补足（如果启用且结果仍不足）
	if m.store != nil {
		extResults, err := m.store.Search(ctx, q32, m.config.TopK, nil)
		if err == nil {
			for _, res := range extResults {
				if seenIDs[res.ID] {
					continue
				}

				item := m.payloadToMemoryItem(res.Payload)
				if item != nil {
					// 外部库通常返回的是向量余弦相似度，我们直接作为分数或进行再次加权
					results = append(results, scored{item, float64(res.Score)})
					seenIDs[res.ID] = true
				}
			}
		}
	}

	return results
}

func (m *MemoryEngine) payloadToMemoryItem(payload map[string]interface{}) *MemoryItem {
	// 尝试从 JSON 还原完整对象
	if jsonVal, ok := payload["item_json"]; ok {
		if jsonStr, ok := jsonVal.(string); ok {
			var item MemoryItem
			if err := json.Unmarshal([]byte(jsonStr), &item); err == nil {
				return &item
			}
		}
	}

	// 降级处理：如果没有 JSON，尝试从散装 Payload 恢复基础信息
	content, _ := payload["content"].(string)
	if content == "" {
		return nil
	}

	id, _ := payload["id"].(string)
	importance, _ := payload["importance"].(float64)

	return &MemoryItem{
		ID:         id,
		Content:    content,
		Importance: importance,
	}
}

func (m *MemoryEngine) findMemoryItemByID(items []*MemoryItem, id string) *MemoryItem {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

type HybridRetriever struct {
	config        config.HybridSearchConfig
	queryAnalyzer *retrieval.QueryAnalyzer
}

func NewHybridRetriever(cfg config.HybridSearchConfig, analyzer *retrieval.QueryAnalyzer) *HybridRetriever {
	return &HybridRetriever{
		config:        cfg,
		queryAnalyzer: analyzer,
	}
}

func (m *MemoryEngine) Ask(ctx context.Context, namespace, question string) (string, error) {
	if m.llmClient == nil {
		return "", fmt.Errorf("LLM client not set")
	}

	results, err := m.Search(ctx, namespace, question)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "No relevant memories found.", nil
	}

	var context strings.Builder
	for i, r := range results {
		context.WriteString(fmt.Sprintf("[%d] %s\n", i+1, r.Content))
		if r.OriginalContent != "" {
			context.WriteString(fmt.Sprintf("    Source: %s\n", r.OriginalContent))
		}
	}

	prompt := fmt.Sprintf(`Based on the following memory context, answer the question.

Memory Context:
%s

Question: %s

Answer:`, context.String(), question)

	messages := []llm.Message{
		{Role: "system", Content: "You are a helpful assistant that answers questions based on the provided memory context."},
		{Role: "user", Content: prompt},
	}

	answer, err := m.llmClient.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	return answer, nil
}

type hybridSearchResult struct {
	Item          *MemoryItem
	Score         float64
	SemanticScore float64
	LexicalScore  float64
	SymbolicScore float64
	Explanation   string
}

func (hr *HybridRetriever) Search(
	ctx context.Context,
	query string,
	queryEmbedding []float64,
	space *MemorySpace,
	now time.Time,
	decay float64,
) ([]*MemoryItem, error) {

	intent, err := hr.queryAnalyzer.Analyze(ctx, query)
	if err != nil {
		intent = &retrieval.QueryIntent{
			RetrievalDepth: hr.config.BaseK,
			Complexity:     0.5,
		}
	}

	k := hr.config.BaseK
	if hr.config.EnableAdaptive {
		k = retrieval.CalculateDynamicK(hr.config.BaseK, intent.Complexity, hr.config.Delta)
		if k < hr.config.MinK {
			k = hr.config.MinK
		}
		if k > hr.config.MaxK {
			k = hr.config.MaxK
		}
	} else {
		k = intent.RetrievalDepth
	}

	results := hr.multiViewRetrieval(query, queryEmbedding, space, now, decay, k, intent)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	topK := k
	if topK > len(results) {
		topK = len(results)
	}

	finalResults := make([]*MemoryItem, topK)
	for i := 0; i < topK; i++ {
		finalResults[i] = results[i].Item
		finalResults[i].LastAccessedAt = now
	}

	return finalResults, nil
}

func (hr *HybridRetriever) multiViewRetrieval(
	query string,
	queryEmbedding []float64,
	space *MemorySpace,
	now time.Time,
	decay float64,
	k int,
	intent *retrieval.QueryIntent,
) []hybridSearchResult {

	candidates := make(map[string]*hybridSearchResult)

	hr.semanticRetrieval(queryEmbedding, space, now, decay, k, candidates)
	hr.lexicalRetrieval(query, space, k, candidates)
	hr.symbolicFiltering(intent, space, candidates)

	results := make([]hybridSearchResult, 0, len(candidates))
	for _, result := range candidates {
		result.Score = hr.config.SemanticWeight*result.SemanticScore +
			hr.config.LexicalWeight*result.LexicalScore +
			hr.config.SymbolicWeight*result.SymbolicScore
		results = append(results, *result)
	}

	return results
}

func (hr *HybridRetriever) semanticRetrieval(
	queryEmbedding []float64,
	space *MemorySpace,
	now time.Time,
	decay float64,
	k int,
	candidates map[string]*hybridSearchResult,
) {
	q32 := utils.ToFloat32(queryEmbedding)

	shortNeighbors := space.shortIndex.Search(q32, k)
	for _, vec := range shortNeighbors {
		id := vec.Key
		mem := hr.findMemoryItem(space.ShortTerm, id)
		if mem == nil {
			continue
		}

		sim := utils.Cosine(queryEmbedding, mem.Embedding)
		age := now.Sub(mem.LastAccessedAt).Hours() / 24.0
		recency := utils.ExpDecay(decay, age)
		semanticScore := 0.7*sim + 0.3*recency

		docID := "short:" + id
		if result, ok := candidates[docID]; ok {
			result.SemanticScore = max(result.SemanticScore, semanticScore)
		} else {
			candidates[docID] = &hybridSearchResult{
				Item:          mem,
				SemanticScore: semanticScore,
			}
		}
	}

	longNeighbors := space.longIndex.Search(q32, k)
	for _, vec := range longNeighbors {
		id := vec.Key
		mem := hr.findMemoryItem(space.LongTerm, id)
		if mem == nil {
			continue
		}

		sim := utils.Cosine(queryEmbedding, mem.Embedding)
		age := now.Sub(mem.LastAccessedAt).Hours() / 24.0
		recency := utils.ExpDecay(decay, age)
		semanticScore := 0.7*sim + 0.3*recency

		docID := "long:" + id
		if result, ok := candidates[docID]; ok {
			result.SemanticScore = max(result.SemanticScore, semanticScore)
		} else {
			candidates[docID] = &hybridSearchResult{
				Item:          mem,
				SemanticScore: semanticScore,
			}
		}
	}
}

func (hr *HybridRetriever) findMemoryItem(items []*MemoryItem, id string) *MemoryItem {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func (hr *HybridRetriever) lexicalRetrieval(
	query string,
	space *MemorySpace,
	k int,
	candidates map[string]*hybridSearchResult,
) {
	shortResults := space.shortBM25.Search(query, k)
	for _, scored := range shortResults {
		docID := "short:" + scored.DocID
		if result, ok := candidates[docID]; ok {
			result.LexicalScore = scored.Score
		} else {
			mem := hr.findMemoryItem(space.ShortTerm, scored.DocID)
			if mem != nil {
				candidates[docID] = &hybridSearchResult{
					Item:         mem,
					LexicalScore: scored.Score,
				}
			}
		}
	}

	longResults := space.longBM25.Search(query, k)
	for _, scored := range longResults {
		docID := "long:" + scored.DocID
		if result, ok := candidates[docID]; ok {
			result.LexicalScore = scored.Score
		} else {
			mem := hr.findMemoryItem(space.LongTerm, scored.DocID)
			if mem != nil {
				candidates[docID] = &hybridSearchResult{
					Item:         mem,
					LexicalScore: scored.Score,
				}
			}
		}
	}
}

func (hr *HybridRetriever) symbolicFiltering(
	intent *retrieval.QueryIntent,
	space *MemorySpace,
	candidates map[string]*hybridSearchResult,
) {
	metaQuery := index.MetadataQuery{
		Entities:  intent.Entities,
		TimeStart: nil,
		TimeEnd:   nil,
	}

	if intent.TimeRange != nil {
		metaQuery.TimeStart = intent.TimeRange.Start
		metaQuery.TimeEnd = intent.TimeRange.End
	}

	if len(intent.Entities) > 0 || intent.TimeRange != nil {
		shortMatches := space.shortMetadata.Search(metaQuery)
		for _, id := range shortMatches {
			docID := "short:" + id
			if result, ok := candidates[docID]; ok {
				result.SymbolicScore = 1.0
			}
		}

		longMatches := space.longMetadata.Search(metaQuery)
		for _, id := range longMatches {
			docID := "long:" + id
			if result, ok := candidates[docID]; ok {
				result.SymbolicScore = 1.0
			}
		}
	} else {
		for _, result := range candidates {
			result.SymbolicScore = 0.5
		}
	}
}
