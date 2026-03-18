package engine

import (
	"context"
	"testing"
	"time"
)

func TestSearch_SimplePath_TopKAndRecencyUpdate(t *testing.T) {
	base := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"query": {1, 0},
		"a":     {1, 0},
		"b":     {0.7, 0.3},
		"c":     {0.2, 0.8},
	}}
	eng := newTestEngineWithNow(emb, base)
	eng.config.TopK = 2

	if err := eng.Add(context.Background(), "ns", "a", 0.2); err != nil {
		t.Fatalf("Add a failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "b", 0.2); err != nil {
		t.Fatalf("Add b failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "c", 0.2); err != nil {
		t.Fatalf("Add c failed: %v", err)
	}

	searchNow := base.Add(5 * time.Minute)
	eng.nowFn = func() time.Time { return searchNow }

	res, err := eng.Search(context.Background(), "ns", "query")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected topK=2 results, got %d", len(res))
	}

	for i := 1; i < len(res); i++ {
		prev := eng.score(searchNow, []float64{1, 0}, res[i-1], eng.config.ShortTermDecay)
		curr := eng.score(searchNow, []float64{1, 0}, res[i], eng.config.ShortTermDecay)
		if curr > prev+1e-9 {
			t.Fatalf("scores should be non-increasing: prev=%v curr=%v", prev, curr)
		}
	}

	for _, item := range res {
		if !item.LastAccessedAt.Equal(searchNow) {
			t.Fatalf("expected LastAccessedAt=%v got=%v", searchNow, item.LastAccessedAt)
		}
	}
}

func TestSearch_EmbedderError(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{err: context.Canceled}, time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC))
	_, err := eng.Search(context.Background(), "ns", "query")
	if err == nil {
		t.Fatal("expected search embedder error")
	}
}

func TestSearch_DefaultNamespace(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"x":     {1, 0},
		"query": {1, 0},
	}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "", "x", 0.2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	res, err := eng.Search(context.Background(), "", "query")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected default namespace search result")
	}
}

func TestPayloadToMemoryItem_FromItemJSON(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC))
	item := &MemoryItem{ID: "id1", Content: "content", Importance: 0.6}
	payload := eng.memoryItemToPayload(item, "ns")

	got := eng.payloadToMemoryItem(payload)
	if got == nil {
		t.Fatal("expected restored item from item_json")
	}
	if got.ID != "id1" || got.Content != "content" {
		t.Fatalf("unexpected restored item: %+v", *got)
	}
}

func TestPayloadToMemoryItem_FallbackPayload(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC))
	payload := map[string]interface{}{
		"id":         "id2",
		"content":    "fallback",
		"importance": 0.4,
	}

	got := eng.payloadToMemoryItem(payload)
	if got == nil {
		t.Fatal("expected fallback reconstructed item")
	}
	if got.ID != "id2" || got.Content != "fallback" || got.Importance != 0.4 {
		t.Fatalf("unexpected fallback item: %+v", *got)
	}
}

func TestPayloadToMemoryItem_InvalidPayload(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC))
	payload := map[string]interface{}{"importance": 0.4}
	got := eng.payloadToMemoryItem(payload)
	if got != nil {
		t.Fatalf("expected nil for invalid payload, got %+v", *got)
	}
}
