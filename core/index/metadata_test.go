package index

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestMetadataIndex_AddAndDirectSearch(t *testing.T) {
	idx := NewMetadataIndex()
	now := time.Date(2026, time.March, 17, 12, 0, 0, 0, time.UTC)

	idx.Add("doc-1", Metadata{
		Entities:  []string{"Alice"},
		Topic:     "Roadmap",
		Timestamp: now,
	})
	idx.Add("doc-2", Metadata{
		Entities:  []string{"ALICE"},
		Topic:     "ROADMAP",
		Timestamp: now.Add(time.Minute),
	})

	gotEntity := idx.SearchByEntity("aLiCe")
	assertSameElements(t, gotEntity, []string{"doc-1", "doc-2"})

	gotTopic := idx.SearchByTopic("rOaDmAp")
	assertSameElements(t, gotTopic, []string{"doc-1", "doc-2"})
}

func TestMetadataIndex_Search_ByFilters(t *testing.T) {
	idx := NewMetadataIndex()
	base := time.Date(2026, time.March, 17, 8, 0, 0, 0, time.UTC)

	idx.Add("doc-1", Metadata{
		Entities:  []string{"Alice"},
		Topic:     "Planning",
		Tags:      []string{"Team", "Urgent"},
		Timestamp: base,
	})
	idx.Add("doc-2", Metadata{
		Entities:  []string{"Bob"},
		Topic:     "Research",
		Tags:      []string{"Team", "Draft"},
		Timestamp: base.Add(time.Hour),
	})
	idx.Add("doc-3", Metadata{
		Entities:  []string{"alice"},
		Topic:     "PLANNING",
		Tags:      []string{"Archive"},
		Timestamp: base.Add(2 * time.Hour),
	})

	t.Run("entity-only", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Entities: []string{"ALICE"}})
		assertSetEqual(t, got, []string{"doc-1", "doc-3"})
	})

	t.Run("topic-only", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Topic: "planning"})
		assertSetEqual(t, got, []string{"doc-1", "doc-3"})
	})

	t.Run("tag-only", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Tags: []string{"TEAM"}})
		assertSetEqual(t, got, []string{"doc-1", "doc-2"})
	})

	t.Run("mixed-entity-plus-tag", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Entities: []string{"ALICE"}, Tags: []string{"DRAFT"}})
		assertSetEqual(t, got, []string{"doc-1", "doc-2", "doc-3"})
	})

	t.Run("mixed-topic-plus-tag", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Topic: "RESEARCH", Tags: []string{"ARCHIVE"}})
		assertSetEqual(t, got, []string{"doc-2", "doc-3"})
	})

	t.Run("no-match-tag", func(t *testing.T) {
		got := idx.Search(MetadataQuery{Tags: []string{"missing"}})
		if len(got) != 0 {
			t.Fatalf("expected no results for missing tag, got %v", got)
		}
	})
}

func TestMetadataIndex_Search_NoFilterReturnsAllWithinTimeRange(t *testing.T) {
	idx := NewMetadataIndex()
	base := time.Date(2026, time.March, 17, 10, 0, 0, 0, time.UTC)

	idx.Add("doc-before", Metadata{Topic: "A", Timestamp: base.Add(-time.Hour)})
	idx.Add("doc-start", Metadata{Topic: "B", Timestamp: base})
	idx.Add("doc-mid", Metadata{Topic: "C", Timestamp: base.Add(time.Hour)})
	idx.Add("doc-end", Metadata{Topic: "D", Timestamp: base.Add(2 * time.Hour)})
	idx.Add("doc-after", Metadata{Topic: "E", Timestamp: base.Add(3 * time.Hour)})

	start := base
	end := base.Add(2 * time.Hour)

	got := idx.Search(MetadataQuery{TimeStart: &start, TimeEnd: &end})
	assertSetEqual(t, got, []string{"doc-start", "doc-mid", "doc-end"})
}

func TestMetadataIndex_Delete_RemovesAllIndexes(t *testing.T) {
	idx := NewMetadataIndex()
	ts := time.Date(2026, time.March, 17, 15, 0, 0, 0, time.UTC)

	idx.Add("doc-1", Metadata{
		Entities:  []string{"Alice"},
		Topic:     "Planning",
		Tags:      []string{"Team"},
		Timestamp: ts,
	})

	idx.Delete("doc-1")

	if got := idx.SearchByEntity("alice"); len(got) != 0 {
		t.Fatalf("expected entity index to be empty, got %v", got)
	}
	if got := idx.SearchByTopic("planning"); len(got) != 0 {
		t.Fatalf("expected topic index to be empty, got %v", got)
	}
	if got := idx.Search(MetadataQuery{Tags: []string{"team"}}); len(got) != 0 {
		t.Fatalf("expected tag query to return no results, got %v", got)
	}
	if got := idx.SearchByTimeRange(ts, ts); len(got) != 0 {
		t.Fatalf("expected time range query to return no results, got %v", got)
	}
	if meta := idx.GetMetadata("doc-1"); meta != nil {
		t.Fatalf("expected metadata to be deleted, got %+v", *meta)
	}
}

func TestMetadataIndex_GetMetadata(t *testing.T) {
	idx := NewMetadataIndex()
	ts := time.Date(2026, time.March, 17, 18, 30, 0, 0, time.UTC)
	want := Metadata{
		Entities:  []string{"Bob"},
		Topic:     "Notes",
		Tags:      []string{"x"},
		Timestamp: ts,
		Extra:     map[string]string{"k": "v"},
	}

	idx.Add("doc-1", want)

	got := idx.GetMetadata("doc-1")
	if got == nil {
		t.Fatal("expected metadata for doc-1, got nil")
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("unexpected metadata: got %+v want %+v", *got, want)
	}

	missing := idx.GetMetadata("does-not-exist")
	if missing != nil {
		t.Fatalf("expected nil metadata for missing doc, got %+v", *missing)
	}
}

func TestMetadataIndex_SearchByTimeRange_InclusiveBoundaries(t *testing.T) {
	idx := NewMetadataIndex()
	start := time.Date(2026, time.March, 17, 20, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	idx.Add("doc-before", Metadata{Timestamp: start.Add(-time.Second)})
	idx.Add("doc-start", Metadata{Timestamp: start})
	idx.Add("doc-mid", Metadata{Timestamp: start.Add(time.Hour)})
	idx.Add("doc-end", Metadata{Timestamp: end})
	idx.Add("doc-after", Metadata{Timestamp: end.Add(time.Second)})

	got := idx.SearchByTimeRange(start, end)
	assertSetEqual(t, got, []string{"doc-start", "doc-mid", "doc-end"})
}

func assertSetEqual(t *testing.T, got, want []string) {
	t.Helper()

	gotCopy := append([]string(nil), got...)
	wantCopy := append([]string(nil), want...)
	sort.Strings(gotCopy)
	sort.Strings(wantCopy)

	if !reflect.DeepEqual(gotCopy, wantCopy) {
		t.Fatalf("unexpected results: got %v want %v", gotCopy, wantCopy)
	}
}

func assertSameElements(t *testing.T, got, want []string) {
	t.Helper()

	gotCopy := append([]string(nil), got...)
	wantCopy := append([]string(nil), want...)
	sort.Strings(gotCopy)
	sort.Strings(wantCopy)

	if !reflect.DeepEqual(gotCopy, wantCopy) {
		t.Fatalf("unexpected elements: got %v want %v", gotCopy, wantCopy)
	}
}
