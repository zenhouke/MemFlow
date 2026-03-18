package retrieval

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"memflow/core/llm"
)

type fakeLLMClient struct {
	response      string
	err           error
	chatCallCount int
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	f.chatCallCount++
	return f.response, f.err
}

func assertReasoningContains(t *testing.T, reasoning string, terms ...string) {
	t.Helper()
	if strings.TrimSpace(reasoning) == "" {
		t.Fatalf("Reasoning is empty")
	}
	lower := strings.ToLower(reasoning)
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			return
		}
	}
	t.Fatalf("Reasoning = %q, want to contain one of %v", reasoning, terms)
}

func TestNewQueryAnalyzer_DefaultBaseK(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 0)
	if qa.baseK != 5 {
		t.Fatalf("baseK = %d, want 5", qa.baseK)
	}

	qa = NewQueryAnalyzer(nil, -10)
	if qa.baseK != 5 {
		t.Fatalf("baseK = %d, want 5", qa.baseK)
	}
}

func TestAnalyze_NoLLM_UsesRuleBased(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 6)

	got, err := qa.Analyze(context.Background(), "why did deployment fail")
	if err != nil {
		t.Fatalf("Analyze error = %v, want nil", err)
	}
	if got.Type != "reasoning" {
		t.Fatalf("Type = %q, want %q", got.Type, "reasoning")
	}
	if got.RetrievalDepth != 12 {
		t.Fatalf("RetrievalDepth = %d, want 12", got.RetrievalDepth)
	}
}

func TestAnalyze_ShortQuery_DoesNotCallLLM(t *testing.T) {
	fake := &fakeLLMClient{
		response: `{"type":"aggregation","keywords":["x"],"entities":[],"complexity":0.8,"reasoning":"llm","retrieval_depth":20}`,
	}
	qa := NewQueryAnalyzer(fake, 5)

	got, err := qa.Analyze(context.Background(), "project status")
	if err != nil {
		t.Fatalf("Analyze error = %v, want nil", err)
	}
	if fake.chatCallCount != 0 {
		t.Fatalf("chatCallCount = %d, want 0", fake.chatCallCount)
	}
	if got.Type != "factual" {
		t.Fatalf("Type = %q, want %q", got.Type, "factual")
	}
}

func TestAnalyze_NonShortQuery_CallsLLMOnce(t *testing.T) {
	fake := &fakeLLMClient{
		response: `{"type":"aggregation","keywords":["memory","updates"],"entities":["Alice"],"complexity":0.8,"reasoning":"Requires aggregation","retrieval_depth":20}`,
	}
	qa := NewQueryAnalyzer(fake, 5)

	got, err := qa.Analyze(context.Background(), "count all memory updates from alice")
	if err != nil {
		t.Fatalf("Analyze error = %v, want nil", err)
	}
	if fake.chatCallCount != 1 {
		t.Fatalf("chatCallCount = %d, want 1", fake.chatCallCount)
	}
	if got.Type != "aggregation" {
		t.Fatalf("Type = %q, want %q", got.Type, "aggregation")
	}
	if got.RetrievalDepth != 20 {
		t.Fatalf("RetrievalDepth = %d, want 20", got.RetrievalDepth)
	}
}

func TestAnalyze_LLMError_FallsBackToRuleBased(t *testing.T) {
	fake := &fakeLLMClient{err: errors.New("llm unavailable")}
	qa := NewQueryAnalyzer(fake, 4)

	got, err := qa.Analyze(context.Background(), "what happened yesterday in production")
	if err != nil {
		t.Fatalf("Analyze error = %v, want nil", err)
	}
	if fake.chatCallCount != 1 {
		t.Fatalf("chatCallCount = %d, want 1", fake.chatCallCount)
	}
	if got.Type != "factual" {
		t.Fatalf("Type = %q, want %q", got.Type, "factual")
	}
	assertReasoningContains(t, got.Reasoning, "fact", "lookup")
	if got.RetrievalDepth != 4 {
		t.Fatalf("RetrievalDepth = %d, want 4", got.RetrievalDepth)
	}
}

func TestParseIntentResponse_ValidJSON(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 5)

	resp := `{"type":"reasoning","keywords":["root","cause"],"entities":["DB"],"complexity":0.7,"reasoning":"Needs analysis","retrieval_depth":15}`
	got, err := qa.parseIntentResponse(resp, "why did DB fail")
	if err != nil {
		t.Fatalf("parseIntentResponse error = %v, want nil", err)
	}
	if got.Type != "reasoning" {
		t.Fatalf("Type = %q, want %q", got.Type, "reasoning")
	}
	if got.RetrievalDepth != 15 {
		t.Fatalf("RetrievalDepth = %d, want 15", got.RetrievalDepth)
	}
	if !reflect.DeepEqual(got.Keywords, []string{"root", "cause"}) {
		t.Fatalf("Keywords = %v, want %v", got.Keywords, []string{"root", "cause"})
	}
}

func TestParseIntentResponse_EmbeddedJSON(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 5)

	resp := `analysis:
{"type":"temporal","keywords":["release"],"entities":["MemFlow"],"complexity":0.6,"reasoning":"time query","retrieval_depth":8}
end`
	got, err := qa.parseIntentResponse(resp, "when was MemFlow released")
	if err != nil {
		t.Fatalf("parseIntentResponse error = %v, want nil", err)
	}
	if got.Type != "temporal" {
		t.Fatalf("Type = %q, want %q", got.Type, "temporal")
	}
	if got.RetrievalDepth != 8 {
		t.Fatalf("RetrievalDepth = %d, want 8", got.RetrievalDepth)
	}
}

func TestParseIntentResponse_InvalidJSON_FallsBack(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 5)

	got, err := qa.parseIntentResponse("this is not json", "when did it happen")
	if err != nil {
		t.Fatalf("parseIntentResponse error = %v, want nil", err)
	}
	if got.Type != "temporal" {
		t.Fatalf("Type = %q, want %q", got.Type, "temporal")
	}
	assertReasoningContains(t, got.Reasoning, "time", "temporal")
	if got.RetrievalDepth != 8 {
		t.Fatalf("RetrievalDepth = %d, want 8", got.RetrievalDepth)
	}
}

func TestParseIntentResponse_ZeroRetrievalDepth_UsesBaseK(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 9)

	resp := `{"type":"factual","keywords":["status"],"entities":[],"complexity":0.4,"reasoning":"lookup","retrieval_depth":0}`
	got, err := qa.parseIntentResponse(resp, "status update")
	if err != nil {
		t.Fatalf("parseIntentResponse error = %v, want nil", err)
	}
	if got.RetrievalDepth != 9 {
		t.Fatalf("RetrievalDepth = %d, want 9", got.RetrievalDepth)
	}
}

func TestRuleBasedAnalyze_Branches(t *testing.T) {
	qa := NewQueryAnalyzer(nil, 5)

	tests := []struct {
		name      string
		query     string
		wantType  string
		wantDepth int
	}{
		{name: "factual", query: "what is MemFlow", wantType: "factual", wantDepth: 5},
		{name: "temporal", query: "events before launch", wantType: "temporal", wantDepth: 8},
		{name: "reasoning", query: "why did it fail", wantType: "reasoning", wantDepth: 10},
		{name: "aggregation", query: "count all deployments", wantType: "aggregation", wantDepth: 15},
		{name: "default", query: "deployment notes", wantType: "factual", wantDepth: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qa.ruleBasedAnalyze(tt.query)
			if got.Type != tt.wantType {
				t.Fatalf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.RetrievalDepth != tt.wantDepth {
				t.Fatalf("RetrievalDepth = %d, want %d", got.RetrievalDepth, tt.wantDepth)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	got := extractKeywords("what is Memory Graph architecture design")
	want := []string{"Memory", "Graph", "architecture", "design"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractKeywords() = %v, want %v", got, want)
	}
}

func TestExtractEntities(t *testing.T) {
	got := extractEntities("met Alice in Paris with Bob.")
	want := []string{"Alice", "Paris", "Bob"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractEntities() = %v, want %v", got, want)
	}
}

func TestContainsTimeKeywords(t *testing.T) {
	if !containsTimeKeywords("events before launch") {
		t.Fatalf("containsTimeKeywords() = false, want true")
	}
	if containsTimeKeywords("project overview") {
		t.Fatalf("containsTimeKeywords() = true, want false")
	}
}

func TestContainsAggregationKeywords(t *testing.T) {
	if !containsAggregationKeywords("count total items") {
		t.Fatalf("containsAggregationKeywords() = false, want true")
	}
	if containsAggregationKeywords("explain architecture") {
		t.Fatalf("containsAggregationKeywords() = true, want false")
	}
}

func TestCalculateDynamicK(t *testing.T) {
	tests := []struct {
		name       string
		baseK      int
		complexity float64
		delta      float64
		want       int
	}{
		{name: "default delta when zero", baseK: 5, complexity: 0.5, delta: 0, want: 10},
		{name: "custom delta", baseK: 10, complexity: 0.25, delta: 1.2, want: 13},
		{name: "zero complexity", baseK: 7, complexity: 0, delta: 2.0, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDynamicK(tt.baseK, tt.complexity, tt.delta)
			if got != tt.want {
				t.Fatalf("CalculateDynamicK() = %d, want %d", got, tt.want)
			}
		})
	}
}
