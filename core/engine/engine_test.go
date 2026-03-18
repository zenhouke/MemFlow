package engine

import (
	"context"
	"testing"
	"time"
)

func TestNew_SetsDefaults(t *testing.T) {
	eng := New(newTestConfig(), &fakeEmbedder{})

	if eng == nil {
		t.Fatal("expected engine, got nil")
	}
	if eng.spaces == nil {
		t.Fatal("expected spaces map initialized")
	}
	if eng.nowFn == nil {
		t.Fatal("expected nowFn to be initialized")
	}
}

func TestDefaultNamespace_Behavior(t *testing.T) {
	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{"alpha": {1, 0}}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "", "alpha", 0.2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	items, err := eng.Get("default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item in default namespace, got %d", len(items))
	}
}

func TestRebuildIndex_DefaultNamespace_NoPanic(t *testing.T) {
	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{"alpha": {1, 0}}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "", "alpha", 0.2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	eng.RebuildIndex("")
}

func TestRebuildIndex_MissingNamespace_NoPanic(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC))
	eng.RebuildIndex("missing")
}

func TestRebuildIndex_PreservesCountAndSearchability(t *testing.T) {
	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"alpha": {1, 0},
		"beta":  {0.8, 0.2},
	}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "ns", "alpha", 0.2); err != nil {
		t.Fatalf("Add alpha failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "beta", 0.9); err != nil {
		t.Fatalf("Add beta failed: %v", err)
	}

	beforeItems, err := eng.Get("ns")
	if err != nil {
		t.Fatalf("Get before rebuild failed: %v", err)
	}
	beforeCount := len(beforeItems)

	eng.RebuildIndex("ns")

	afterItems, err := eng.Get("ns")
	if err != nil {
		t.Fatalf("Get after rebuild failed: %v", err)
	}
	if len(afterItems) != beforeCount {
		t.Fatalf("count changed after rebuild: before=%d after=%d", beforeCount, len(afterItems))
	}

	searchRes, err := eng.Search(context.Background(), "ns", "alpha")
	if err != nil {
		t.Fatalf("Search after rebuild failed: %v", err)
	}
	if len(searchRes) == 0 {
		t.Fatal("expected searchable results after rebuild")
	}
}
