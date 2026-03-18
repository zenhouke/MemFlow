package engine

import (
	"context"
	"testing"
	"time"
)

func TestAdd_RoutesByImportanceThreshold(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{"short": {1, 0}, "long": {0, 1}}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "ns", "short", 0.2); err != nil {
		t.Fatalf("Add short failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "long", 0.9); err != nil {
		t.Fatalf("Add long failed: %v", err)
	}

	space, ok := eng.getSpace("ns")
	if !ok {
		t.Fatal("expected namespace ns to exist")
	}

	if len(space.ShortTerm) != 1 {
		t.Fatalf("expected 1 short-term item, got %d", len(space.ShortTerm))
	}
	if len(space.LongTerm) != 1 {
		t.Fatalf("expected 1 long-term item, got %d", len(space.LongTerm))
	}
}

func TestAdd_EmptyNamespace_UsesDefaultNamespace(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{fixed: []float64{1, 0}}, now)

	if err := eng.Add(context.Background(), "", "hello", 0.2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	items, err := eng.Get("default")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected item in default namespace, got %d", len(items))
	}
}

func TestAdd_UsesEstimatorWhenImportanceZero(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	est := &fakeEstimator{value: 0.9}
	eng.SetImportanceEstimator(est)

	if err := eng.Add(context.Background(), "ns", "from-estimator", 0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	space, _ := eng.getSpace("ns")
	if len(space.LongTerm) != 1 {
		t.Fatalf("expected estimator-routed long-term item, got %d", len(space.LongTerm))
	}
	if est.calls != 1 {
		t.Fatalf("estimator calls = %d, want 1", est.calls)
	}
}

func TestAdd_EstimatorErrorIsNonFatal(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	est := &fakeEstimator{err: context.DeadlineExceeded}
	eng.SetImportanceEstimator(est)

	if err := eng.Add(context.Background(), "ns", "fallback", 0); err != nil {
		t.Fatalf("Add should not fail on estimator error, got %v", err)
	}

	space, _ := eng.getSpace("ns")
	if len(space.ShortTerm) != 1 {
		t.Fatalf("expected fallback short-term write, got %d", len(space.ShortTerm))
	}
}

func TestAdd_EmbedderError_NoWrite(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{err: context.Canceled}, now)

	err := eng.Add(context.Background(), "ns", "bad", 0.5)
	if err == nil {
		t.Fatal("expected embedder error")
	}

	items, getErr := eng.Get("ns")
	if getErr != nil {
		t.Fatalf("Get failed: %v", getErr)
	}
	if len(items) != 0 {
		t.Fatalf("expected no writes on embed error, got %d items", len(items))
	}
}

func TestGet_MissingNamespace_ReturnsNilNil(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC))
	items, err := eng.Get("missing")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %v", items)
	}
}

func TestGet_AggregatesAllTiers(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{fixed: []float64{1, 0}}, now)

	if err := eng.Add(context.Background(), "ns", "short", 0.2); err != nil {
		t.Fatalf("Add short failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "long", 0.9); err != nil {
		t.Fatalf("Add long failed: %v", err)
	}

	space, _ := eng.getSpace("ns")
	space.mu.Lock()
	space.Archived = append(space.Archived, &MemoryItem{ID: "archived-1", Content: "archived", CreatedAt: now, LastAccessedAt: now})
	space.mu.Unlock()

	items, err := eng.Get("ns")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 aggregated items, got %d", len(items))
	}
}

func TestDelete_RemovesFromEachTier(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"short": {1, 0},
		"long":  {0, 1},
		"query": {1, 0},
	}}
	eng := newTestEngineWithNow(emb, now)

	if err := eng.Add(context.Background(), "ns", "short", 0.2); err != nil {
		t.Fatalf("Add short failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "long", 0.9); err != nil {
		t.Fatalf("Add long failed: %v", err)
	}

	space, _ := eng.getSpace("ns")
	space.mu.Lock()
	space.Archived = append(space.Archived, &MemoryItem{ID: "archived-1", Content: "archived", Embedding: []float64{0.1, 0.1}, CreatedAt: now, LastAccessedAt: now})
	shortID := space.ShortTerm[0].ID
	longID := space.LongTerm[0].ID
	space.mu.Unlock()

	if err := eng.Delete("ns", shortID); err != nil {
		t.Fatalf("Delete short failed: %v", err)
	}
	if err := eng.Delete("ns", longID); err != nil {
		t.Fatalf("Delete long failed: %v", err)
	}
	if err := eng.Delete("ns", "archived-1"); err != nil {
		t.Fatalf("Delete archived failed: %v", err)
	}

	items, err := eng.Get("ns")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected all tiers empty after deletes, got %d", len(items))
	}

	searchRes, err := eng.Search(context.Background(), "ns", "query")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchRes) != 0 {
		t.Fatalf("expected deleted items absent from search, got %d", len(searchRes))
	}
}

func TestDelete_MissingNamespace_ReturnsError(t *testing.T) {
	eng := newTestEngineWithNow(&fakeEmbedder{}, time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC))
	err := eng.Delete("missing", "id")
	if err == nil {
		t.Fatal("expected namespace not found error")
	}
}

func TestDelete_MissingID_ReturnsError(t *testing.T) {
	now := time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC)
	eng := newTestEngineWithNow(&fakeEmbedder{fixed: []float64{1, 0}}, now)
	if err := eng.Add(context.Background(), "ns", "exists", 0.2); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err := eng.Delete("ns", "missing-id")
	if err == nil {
		t.Fatal("expected missing id error")
	}
}
