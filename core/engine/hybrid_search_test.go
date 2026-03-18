package engine

import (
	"context"
	"errors"
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
