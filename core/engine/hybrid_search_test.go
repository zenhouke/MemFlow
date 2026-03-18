package engine

import (
	"context"
	"errors"
	"fmt"
	"memflow/core/config"
	"testing"
	"time"
)

func TestHybrid_Search_EmbedderError(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{err: context.Canceled}, now)

	llmFake := &fakeLLMClient{response: `{"type":"factual","complexity":0.1,"retrieval_depth":3}`}
	eng.SetLLMClient(llmFake)

	_, err := eng.Search(context.Background(), "ns", "project zephyr status")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected embedder error context.Canceled, got %v", err)
	}
	if llmFake.chatCallCount != 0 {
		t.Fatalf("expected no llm calls when embedding fails, got %d", llmFake.chatCallCount)
	}
}

func TestHybrid_Search_UsesHybridPath_ConfigEnabled(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)

	emb := &fakeEmbedder{vectors: map[string][]float64{
		"find projectzephyr launch details": {1, 0},
		"semantic near only":                {1, 0},
		"projectzephyr launch launch":       {0.5, 0.8660254},
		"unrelated filler":                  {0, 1},
	}}

	eng := newHybridTestEngine(emb, now)
	eng.config.HybridSearchConfig = config.HybridSearchConfig{
		SemanticWeight: 0.2,
		LexicalWeight:  0.7,
		SymbolicWeight: 0.1,
		EnableAdaptive: false,
		BaseK:          3,
		Delta:          2.0,
		MinK:           3,
		MaxK:           20,
	}

	llmFake := &fakeLLMClient{response: `{"type":"factual","keywords":["projectzephyr","launch"],"entities":["ProjectZephyr"],"complexity":0.2,"reasoning":"entity-aware search","retrieval_depth":3}`}
	eng.SetLLMClient(llmFake)

	if err := eng.Add(context.Background(), "ns", "semantic near only", 0.2); err != nil {
		t.Fatalf("add semantic doc failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "projectzephyr launch launch", 0.2); err != nil {
		t.Fatalf("add lexical+symbolic doc failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "unrelated filler", 0.2); err != nil {
		t.Fatalf("add filler doc failed: %v", err)
	}

	mustTagEntityOnContent(t, eng, "ns", "projectzephyr launch launch", "ProjectZephyr")

	res, err := eng.Search(context.Background(), "ns", "find projectzephyr launch details")
	if err != nil {
		t.Fatalf("hybrid search failed: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected hybrid search results")
	}
	if res[0].Content != "projectzephyr launch launch" {
		t.Fatalf("expected hybrid-ranked top content to be lexical+symbolic doc, got %q", res[0].Content)
	}
	if llmFake.chatCallCount != 1 {
		t.Fatalf("expected one llm call from hybrid query analyzer, got %d", llmFake.chatCallCount)
	}
}

func TestHybrid_Intent_ShortQueryFastPath_OneAndTwoTokens_NoLLMCall(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		query string
	}{
		{name: "one_token", query: "status"},
		{name: "two_tokens", query: "status update"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
			eng.config.HybridSearchConfig.EnableAdaptive = false
			eng.config.HybridSearchConfig.BaseK = 3

			llmFake := &fakeLLMClient{response: `{"type":"aggregation","complexity":0.9,"retrieval_depth":20}`}
			eng.SetLLMClient(llmFake)

			if err := eng.Add(context.Background(), "ns", "status update from alpha", 0.3); err != nil {
				t.Fatalf("add fixture failed: %v", err)
			}

			_, err := eng.Search(context.Background(), "ns", tc.query)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if llmFake.chatCallCount != 0 {
				t.Fatalf("expected no llm call for short query %q, got %d", tc.query, llmFake.chatCallCount)
			}
		})
	}
}

func TestHybrid_Intent_ThreeTokens_NotFastPath(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	eng.config.HybridSearchConfig.EnableAdaptive = false
	eng.config.HybridSearchConfig.BaseK = 3

	llmFake := &fakeLLMClient{response: `{"type":"factual","complexity":0.1,"retrieval_depth":1}`}
	eng.SetLLMClient(llmFake)

	for i := 0; i < 4; i++ {
		if err := eng.Add(context.Background(), "ns", fmt.Sprintf("alpha beta gamma fixture %d", i), 0.3); err != nil {
			t.Fatalf("add fixture %d failed: %v", i, err)
		}
	}

	res, err := eng.Search(context.Background(), "ns", "alpha beta gamma")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if llmFake.chatCallCount != 1 {
		t.Fatalf("expected one llm call for 3-token query, got %d", llmFake.chatCallCount)
	}
	if len(res) != 1 {
		t.Fatalf("expected retrieval_depth from llm to apply (1), got %d", len(res))
	}
}

func TestHybrid_Intent_LLMFailure_FallsBackRuleBased(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	eng.config.HybridSearchConfig.EnableAdaptive = false
	eng.config.HybridSearchConfig.BaseK = 4

	llmFake := &fakeLLMClient{err: errors.New("llm unavailable")}
	eng.SetLLMClient(llmFake)

	addKSelectionFixtures(t, eng, "ns", 16)

	res, err := eng.Search(context.Background(), "ns", "why did deploy fail")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if llmFake.chatCallCount != 1 {
		t.Fatalf("expected one llm call before fallback, got %d", llmFake.chatCallCount)
	}
	if len(res) != 8 {
		t.Fatalf("expected rule-based fallback depth baseK*2 (8), got %d", len(res))
	}
}

func TestHybrid_Intent_LLMSuccess_UsesIntentFields(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	eng.config.HybridSearchConfig = config.HybridSearchConfig{
		SemanticWeight: 0.0,
		LexicalWeight:  0.0,
		SymbolicWeight: 1.0,
		EnableAdaptive: false,
		BaseK:          3,
		Delta:          2.0,
		MinK:           3,
		MaxK:           20,
	}

	llmFake := &fakeLLMClient{response: `{"type":"factual","keywords":["status","update"],"entities":["ProjectZephyr"],"complexity":0.2,"reasoning":"entity-specific lookup","retrieval_depth":2}`}
	eng.SetLLMClient(llmFake)

	if err := eng.Add(context.Background(), "ns", "status update projectzephyr today", 0.3); err != nil {
		t.Fatalf("add entity fixture failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "status update unrelated today", 0.3); err != nil {
		t.Fatalf("add non-entity fixture failed: %v", err)
	}

	mustTagEntityOnContent(t, eng, "ns", "status update projectzephyr today", "ProjectZephyr")

	res, err := eng.Search(context.Background(), "ns", "status update projectzephyr today please")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if llmFake.chatCallCount != 1 {
		t.Fatalf("expected llm to be called once, got %d", llmFake.chatCallCount)
	}
	if len(res) != 2 {
		t.Fatalf("expected retrieval_depth from llm to apply (2), got %d", len(res))
	}
	if res[0].Content != "status update projectzephyr today" {
		t.Fatalf("expected entity-matched item ranked first, got %q", res[0].Content)
	}
}

func mustTagEntityOnContent(t *testing.T, eng *MemoryEngine, namespace, content, entity string) {
	t.Helper()

	eng.mu.RLock()
	space, ok := eng.spaces[namespace]
	eng.mu.RUnlock()
	if !ok {
		t.Fatalf("namespace %q not found", namespace)
	}

	space.mu.Lock()
	defer space.mu.Unlock()

	for _, item := range space.ShortTerm {
		if item.Content != content {
			continue
		}
		item.Metadata.Entities = []string{entity}
		space.shortMetadata.Add(item.ID, item.Metadata)
		return
	}

	t.Fatalf("content %q not found in short-term memory", content)
}

func TestHybrid_KSelection_AdaptiveClamp(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		complexity float64
		baseK      int
		delta      float64
		minK       int
		maxK       int
		wantK      int
	}{
		{name: "clamps_to_min", complexity: 0.0, baseK: 5, delta: 2.0, minK: 3, maxK: 20, wantK: 5},
		{name: "clamps_to_max", complexity: 1.0, baseK: 5, delta: 2.0, minK: 3, maxK: 8, wantK: 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
			eng.config.HybridSearchConfig = config.HybridSearchConfig{
				SemanticWeight: 0.6,
				LexicalWeight:  0.3,
				SymbolicWeight: 0.1,
				EnableAdaptive: true,
				BaseK:          tc.baseK,
				Delta:          tc.delta,
				MinK:           tc.minK,
				MaxK:           tc.maxK,
			}

			llmFake := &fakeLLMClient{response: fmt.Sprintf(`{"type":"factual","complexity":%.1f,"retrieval_depth":3}`, tc.complexity)}
			eng.SetLLMClient(llmFake)

			addKSelectionFixtures(t, eng, "ns", 16)

			res, err := eng.Search(context.Background(), "ns", "kselection shared terms")
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if len(res) != tc.wantK {
				t.Fatalf("expected %d results, got %d", tc.wantK, len(res))
			}
		})
	}
}

func TestHybrid_KSelection_AdaptiveDeltaZeroUsesDefault(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	eng.config.HybridSearchConfig = config.HybridSearchConfig{
		SemanticWeight: 0.6,
		LexicalWeight:  0.3,
		SymbolicWeight: 0.1,
		EnableAdaptive: true,
		BaseK:          5,
		Delta:          0,
		MinK:           3,
		MaxK:           20,
	}

	llmFake := &fakeLLMClient{response: `{"type":"factual","complexity":0.5,"retrieval_depth":3}`}
	eng.SetLLMClient(llmFake)

	addKSelectionFixtures(t, eng, "ns", 16)

	res, err := eng.Search(context.Background(), "ns", "kselection shared terms")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(res) != 10 {
		t.Fatalf("expected 10 results, got %d", len(res))
	}
}

func TestHybrid_KSelection_NonAdaptiveUsesIntentDepth(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	eng := newHybridTestEngine(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	eng.config.HybridSearchConfig = config.HybridSearchConfig{
		SemanticWeight: 0.6,
		LexicalWeight:  0.3,
		SymbolicWeight: 0.1,
		EnableAdaptive: false,
		BaseK:          3,
		Delta:          2.0,
		MinK:           3,
		MaxK:           20,
	}

	llmFake := &fakeLLMClient{response: `{"type":"factual","complexity":0.1,"retrieval_depth":7}`}
	eng.SetLLMClient(llmFake)

	addKSelectionFixtures(t, eng, "ns", 16)

	res, err := eng.Search(context.Background(), "ns", "kselection shared terms")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(res) != 7 {
		t.Fatalf("expected 7 results, got %d", len(res))
	}
}

func addKSelectionFixtures(t *testing.T, eng *MemoryEngine, namespace string, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		content := fmt.Sprintf("kselection fixture doc %02d shared terms", i)
		if err := eng.Add(context.Background(), namespace, content, 0.4); err != nil {
			t.Fatalf("add fixture %d failed: %v", i, err)
		}
	}
}
