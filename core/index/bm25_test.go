package index

import (
	"math"
	"reflect"
	"testing"
)

const scoreEpsilon = 1e-9

func TestBM25Index_tokenize(t *testing.T) {
	idx := NewBM25Index()

	text := "The QUICK, brown fox! jumps over the lazy dog; and an ox."
	got := idx.tokenize(text)
	want := []string{"quick", "brown", "fox", "jumps", "over", "lazy", "dog", "ox"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected tokens: got %v want %v", got, want)
	}
}

func TestBM25Index_AddDelete_UpdatesStats(t *testing.T) {
	idx := NewBM25Index()

	idx.Add("doc-1", "Apple banana apple")
	if idx.docCount != 1 {
		t.Fatalf("docCount after first add: got %d want 1", idx.docCount)
	}
	if idx.avgDocLen != 3 {
		t.Fatalf("avgDocLen after first add: got %v want 3", idx.avgDocLen)
	}
	if idx.docFreq["apple"] != 1 || idx.docFreq["banana"] != 1 {
		t.Fatalf("unexpected docFreq after first add: %+v", idx.docFreq)
	}

	idx.Add("doc-2", "Banana carrot")
	if idx.docCount != 2 {
		t.Fatalf("docCount after second add: got %d want 2", idx.docCount)
	}
	if !almostEqual(idx.avgDocLen, 2.5) {
		t.Fatalf("avgDocLen after second add: got %v want 2.5", idx.avgDocLen)
	}
	if idx.docFreq["apple"] != 1 || idx.docFreq["banana"] != 2 || idx.docFreq["carrot"] != 1 {
		t.Fatalf("unexpected docFreq after second add: %+v", idx.docFreq)
	}

	idx.Delete("doc-1")
	if idx.docCount != 1 {
		t.Fatalf("docCount after delete doc-1: got %d want 1", idx.docCount)
	}
	if !almostEqual(idx.avgDocLen, 2) {
		t.Fatalf("avgDocLen after delete doc-1: got %v want 2", idx.avgDocLen)
	}
	if _, ok := idx.docFreq["apple"]; ok {
		t.Fatalf("expected apple term removed from docFreq, got %+v", idx.docFreq)
	}
	if idx.docFreq["banana"] != 1 || idx.docFreq["carrot"] != 1 {
		t.Fatalf("unexpected docFreq after delete doc-1: %+v", idx.docFreq)
	}

	idx.Delete("doc-2")
	if idx.docCount != 0 {
		t.Fatalf("docCount after delete doc-2: got %d want 0", idx.docCount)
	}
	if idx.avgDocLen != 0 {
		t.Fatalf("avgDocLen after delete doc-2: got %v want 0", idx.avgDocLen)
	}
	if len(idx.docFreq) != 0 {
		t.Fatalf("expected empty docFreq after deleting all docs, got %+v", idx.docFreq)
	}

	idx.Delete("missing-doc")
	if idx.docCount != 0 || idx.avgDocLen != 0 || len(idx.docFreq) != 0 {
		t.Fatalf("delete missing doc should not change stats: count=%d avg=%v freq=%+v", idx.docCount, idx.avgDocLen, idx.docFreq)
	}
}

func TestBM25Index_Add_ExistingDocIDOverwritesStats(t *testing.T) {
	idx := NewBM25Index()

	idx.Add("doc-1", "apple banana apple")
	if idx.docCount != 1 {
		t.Fatalf("docCount after first add: got %d want 1", idx.docCount)
	}

	idx.Add("doc-1", "banana carrot")
	if idx.docCount != 1 {
		t.Fatalf("docCount after overwrite add: got %d want 1", idx.docCount)
	}
	if !almostEqual(idx.avgDocLen, 2) {
		t.Fatalf("avgDocLen after overwrite add: got %v want 2", idx.avgDocLen)
	}
	if len(idx.docFreq) != 2 || idx.docFreq["banana"] != 1 || idx.docFreq["carrot"] != 1 {
		t.Fatalf("unexpected docFreq after overwrite add: %+v", idx.docFreq)
	}
	if _, ok := idx.docFreq["apple"]; ok {
		t.Fatalf("expected apple removed after overwrite add, got %+v", idx.docFreq)
	}

	idx.Delete("doc-1")
	if idx.docCount != 0 {
		t.Fatalf("docCount after delete overwritten doc: got %d want 0", idx.docCount)
	}
	if idx.avgDocLen != 0 {
		t.Fatalf("avgDocLen after delete overwritten doc: got %v want 0", idx.avgDocLen)
	}
	if len(idx.docFreq) != 0 {
		t.Fatalf("expected empty docFreq after delete overwritten doc, got %+v", idx.docFreq)
	}
}

func TestBM25Index_Search_EmptyIndex(t *testing.T) {
	idx := NewBM25Index()

	got := idx.Search("anything", 5)
	if len(got) != 0 {
		t.Fatalf("expected empty result for empty index, got %v", got)
	}
}

func TestBM25Index_Search_TopKAndSortedScores(t *testing.T) {
	idx := NewBM25Index()

	idx.Add("doc-1", "alpha alpha alpha")
	idx.Add("doc-2", "alpha")
	idx.Add("doc-3", "beta beta")
	idx.Add("doc-4", "beta noise noise noise")
	idx.Add("doc-5", "noise noise")

	got := idx.Search("alpha beta", 3)
	if len(got) != 3 {
		t.Fatalf("expected topK=3 results, got %d (%v)", len(got), got)
	}

	assertScoresNonIncreasing(t, got)

	allowed := map[string]bool{
		"doc-1": true,
		"doc-2": true,
		"doc-3": true,
		"doc-4": true,
	}
	for _, item := range got {
		if !allowed[item.DocID] {
			t.Fatalf("unexpected doc returned: %s in %v", item.DocID, got)
		}
	}
}

func TestBM25Index_Search_ReturnsPositiveScoresOnly(t *testing.T) {
	idx := NewBM25Index()

	idx.Add("doc-pos", "rareterm rareterm rareterm")
	idx.Add("doc-neg-1", "common common")
	idx.Add("doc-neg-2", "common")
	idx.Add("doc-neg-3", "noise")
	idx.Add("doc-neg-4", "noise common")

	got := idx.Search("rareterm", 10)
	if len(got) == 0 {
		t.Fatal("expected at least one positive-scoring result, got none")
	}

	for _, item := range got {
		if item.Score <= 0 {
			t.Fatalf("expected only positive scores, got %v in %v", item.Score, got)
		}
		if item.DocID != "doc-pos" {
			t.Fatalf("expected only doc-pos to remain after filtering, got %v", got)
		}
	}
}

func TestBM25Index_Search_DeletedDocNotReturned(t *testing.T) {
	idx := NewBM25Index()

	idx.Add("doc-delete", "zeta zeta zeta")
	idx.Add("doc-keep", "zeta")
	idx.Add("doc-noise-1", "noise")
	idx.Add("doc-noise-2", "noise")
	idx.Add("doc-noise-3", "noise")

	idx.Delete("doc-delete")

	got := idx.Search("zeta", 10)
	if len(got) == 0 {
		t.Fatal("expected surviving matching doc to be returned")
	}

	for _, item := range got {
		if item.DocID == "doc-delete" {
			t.Fatalf("deleted doc should never be returned, got %v", got)
		}
	}

	if got[0].DocID != "doc-keep" {
		t.Fatalf("expected doc-keep to be top result after delete, got %v", got)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= scoreEpsilon
}

func assertScoresNonIncreasing(t *testing.T, docs []ScoredDoc) {
	t.Helper()

	for i := 1; i < len(docs); i++ {
		if docs[i].Score > docs[i-1].Score+scoreEpsilon {
			t.Fatalf("scores must be non-increasing: prev=%v curr=%v docs=%v", docs[i-1].Score, docs[i].Score, docs)
		}
	}
}
