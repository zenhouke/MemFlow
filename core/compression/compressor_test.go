package compression

import (
	"context"
	"errors"
	"fmt"
	"memflow/core/config"
	"memflow/core/llm"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeLLMClient struct {
	response string
	err      error
	calls    int
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func testDialogues(n int, start time.Time) []Dialogue {
	d := make([]Dialogue, n)
	for i := 0; i < n; i++ {
		d[i] = Dialogue{
			ID:        "d" + strconv.Itoa(i+1),
			Speaker:   []string{"alice", "bob"}[i%2],
			Content:   "content-" + strconv.Itoa(i+1),
			Timestamp: start.Add(time.Duration(i) * time.Minute),
		}
	}
	return d
}

func TestNewSemanticCompressor_DefaultConfig(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{}, nil)
	if c.config.WindowSize != 10 {
		t.Fatalf("WindowSize = %d, want 10", c.config.WindowSize)
	}
	if c.config.OverlapSize != 2 {
		t.Fatalf("OverlapSize = %d, want 2", c.config.OverlapSize)
	}
}

func TestProcessDialogues_EmptyInput(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	units, err := c.ProcessDialogues(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if units != nil {
		t.Fatalf("units = %v, want nil", units)
	}
}

func TestProcessDialogues_StepSizeFloorAndSmallWindowSkip(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 1, OverlapSize: 5}, nil)
	dialogues := testDialogues(3, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	units, err := c.ProcessDialogues(context.Background(), dialogues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(units) != 0 {
		t.Fatalf("units len = %d, want 0", len(units))
	}
}

func TestProcessDialogues_TrimsPreviousUnitsToTen(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	dialogues := testDialogues(15, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	units, err := c.ProcessDialogues(context.Background(), dialogues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(units) <= 10 {
		t.Fatalf("units len = %d, expected >10 for trim test", len(units))
	}
	if len(c.previousUnits) != 10 {
		t.Fatalf("previousUnits len = %d, want 10", len(c.previousUnits))
	}

	if c.previousUnits[0].Content != units[len(units)-10].Content {
		t.Fatalf("trimmed head mismatch: got %q want %q", c.previousUnits[0].Content, units[len(units)-10].Content)
	}
}

func TestSimpleExtract_BasicBehavior(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	t0 := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)
	window := []Dialogue{
		{ID: "1", Speaker: "alice", Content: "hello", Timestamp: t0},
		{ID: "2", Speaker: "bob", Content: "world", Timestamp: t0.Add(2 * time.Minute)},
		{ID: "3", Speaker: "alice", Content: "again", Timestamp: t0.Add(1 * time.Minute)},
	}

	u := c.simpleExtract(window)
	if u == nil {
		t.Fatal("expected unit")
	}
	if !strings.HasPrefix(u.ID, "unit-") {
		t.Fatalf("unexpected ID: %q", u.ID)
	}
	if u.Salience != "medium" {
		t.Fatalf("Salience = %q, want medium", u.Salience)
	}
	if u.SourceDialogueCount != 3 {
		t.Fatalf("SourceDialogueCount = %d, want 3", u.SourceDialogueCount)
	}
	if u.Timestamp == nil || !u.Timestamp.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("Timestamp = %v, want %v", u.Timestamp, t0.Add(2*time.Minute))
	}
	if len(u.Persons) != 2 || u.Persons[0] != "alice" || u.Persons[1] != "bob" {
		t.Fatalf("Persons = %v, want [alice bob]", u.Persons)
	}
	if !strings.Contains(u.Content, "alice: hello;") || !strings.Contains(u.Content, "bob: world;") {
		t.Fatalf("unexpected Content: %q", u.Content)
	}
	if !strings.Contains(u.OriginalContent, "alice: hello") || !strings.Contains(u.OriginalContent, "bob: world") {
		t.Fatalf("unexpected OriginalContent: %q", u.OriginalContent)
	}
}

func TestExtractUnit_LLMErrorFallsBackToSimple(t *testing.T) {
	fake := &fakeLLMClient{err: errors.New("llm down")}
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, fake)
	window := testDialogues(2, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	u, err := c.extractUnit(context.Background(), window, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", fake.calls)
	}
	if u == nil || u.Content == "" {
		t.Fatal("expected fallback simple unit")
	}
	if !strings.HasPrefix(u.ID, "unit-") {
		t.Fatalf("unexpected fallback ID: %q", u.ID)
	}
}

func TestParseResponse_EmbeddedJSONSuccess(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	t0 := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)
	window := []Dialogue{
		{ID: "d1", Speaker: "alice", Content: "first", Timestamp: t0},
		{ID: "d2", Speaker: "bob", Content: "second", Timestamp: t0.Add(time.Minute)},
	}

	resp := "model says ```json {\"content\":\"summary\",\"keywords\":[\"k1\"],\"timestamp\":\"2026-03-18T09:01:00Z\",\"location\":\"room\",\"persons\":[\"alice\"],\"entities\":[\"project\"],\"topic\":\"status\",\"salience\":\"high\",\"importance\":0.9} ```"

	u, err := c.parseResponse(resp, window, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u == nil {
		t.Fatal("expected unit")
	}
	if !strings.HasPrefix(u.ID, "unit-7-") {
		t.Fatalf("unexpected ID: %q", u.ID)
	}
	if u.Content != "summary" || u.Topic != "status" || u.Salience != "high" {
		t.Fatalf("unexpected parsed fields: %+v", *u)
	}
	if u.Timestamp == nil || u.Timestamp.Format(time.RFC3339) != "2026-03-18T09:01:00Z" {
		t.Fatalf("unexpected timestamp: %v", u.Timestamp)
	}
	if len(u.SourceDialogueIDs) != 2 || u.SourceDialogueIDs[0] != "d1" || u.SourceDialogueIDs[1] != "d2" {
		t.Fatalf("unexpected SourceDialogueIDs: %v", u.SourceDialogueIDs)
	}
}

func TestParseResponse_InvalidJSONFallsBackAndErrors(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	window := testDialogues(2, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	u, err := c.parseResponse("{invalid-json}", window, 1)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if u == nil || u.Content == "" {
		t.Fatal("expected fallback simple unit")
	}
	if !strings.HasPrefix(u.ID, "unit-") {
		t.Fatalf("unexpected fallback ID: %q", u.ID)
	}
}

func TestParseResponse_EmptyContentFallsBackNoError(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	window := testDialogues(2, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	u, err := c.parseResponse("{\"content\":\"\"}", window, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u == nil || u.Content == "" {
		t.Fatal("expected fallback simple unit")
	}
}

func TestParseResponse_TimestampFormats(t *testing.T) {
	c := NewSemanticCompressor(config.CompressionConfig{WindowSize: 2, OverlapSize: 1}, nil)
	window := testDialogues(2, time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC))

	tests := []string{
		"2026-03-18T10:11:12Z",
		"2026-03-18T10:11:12",
		"2026-03-18 10:11:12",
	}

	for _, ts := range tests {
		t.Run(ts, func(t *testing.T) {
			resp := fmt.Sprintf("{\"content\":\"ok\",\"timestamp\":\"%s\"}", ts)
			u, err := c.parseResponse(resp, window, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u == nil || u.Timestamp == nil {
				t.Fatalf("expected parsed timestamp for %q", ts)
			}
		})
	}
}

func TestHelpers_UniqueStringsAndExtractors(t *testing.T) {
	in := []string{"alice", "", "bob", "alice", "bob", "charlie"}
	got := uniqueStrings(in)
	if len(got) != 3 || got[0] != "alice" || got[1] != "bob" || got[2] != "charlie" {
		t.Fatalf("uniqueStrings=%v, want [alice bob charlie]", got)
	}

	t0 := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)
	window := []Dialogue{{ID: "1", Speaker: "alice", Content: "x", Timestamp: t0}, {ID: "2", Speaker: "bob", Content: "y", Timestamp: t0.Add(time.Minute)}}
	orig := extractOriginal(window)
	if !strings.Contains(orig, "alice: x") || !strings.Contains(orig, "bob: y") {
		t.Fatalf("unexpected extractOriginal: %q", orig)
	}
	speakers := extractSpeakers(window)
	if len(speakers) != 2 || speakers[0] != "alice" || speakers[1] != "bob" {
		t.Fatalf("unexpected extractSpeakers: %v", speakers)
	}
}
