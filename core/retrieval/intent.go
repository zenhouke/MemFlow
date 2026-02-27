package retrieval

import (
	"context"
	"encoding/json"
	"simplemem/core/llm"
	"strings"
	"time"
)

type QueryIntent struct {
	Type           string     `json:"type"`
	Keywords       []string   `json:"keywords"`
	Entities       []string   `json:"entities"`
	TimeRange      *TimeRange `json:"time_range,omitempty"`
	Complexity     float64    `json:"complexity"`
	Reasoning      string     `json:"reasoning"`
	RetrievalDepth int        `json:"retrieval_depth"`
}

type TimeRange struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

type QueryAnalyzer struct {
	llmClient llm.LLMClient
	baseK     int
}

func NewQueryAnalyzer(llmClient llm.LLMClient, baseK int) *QueryAnalyzer {
	if baseK <= 0 {
		baseK = 5
	}
	return &QueryAnalyzer{
		llmClient: llmClient,
		baseK:     baseK,
	}
}

func (qa *QueryAnalyzer) Analyze(ctx context.Context, query string) (*QueryIntent, error) {
	if qa.llmClient == nil {
		return qa.ruleBasedAnalyze(query), nil
	}

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a query intent analyzer for a memory retrieval system.

Analyze the user query to determine:
1. Type: factual, temporal, reasoning, aggregation
2. Keywords: important search terms
3. Entities: specific entities mentioned (names, places, etc.)
4. Time range: if temporal, extract time constraints
5. Complexity: 0-1 score
6. Reasoning: explain your analysis

OUTPUT FORMAT (JSON):
{
  "type": "factual|temporal|reasoning|aggregation",
  "keywords": ["keyword1", "keyword2"],
  "entities": ["entity1", "entity2"],
  "time_range": {"start": "2025-01-01T00:00:00Z", "end": "2025-12-31T23:59:59Z"} or null,
  "complexity": 0.8,
  "reasoning": "explanation",
  "retrieval_depth": 10
}

Rules for retrieval_depth:
- factual: 5
- temporal: 8
- reasoning: 15
- aggregation: 20`,
		},
		{
			Role:    "user",
			Content: query,
		},
	}

	response, err := qa.llmClient.Chat(ctx, messages)
	if err != nil {
		return qa.ruleBasedAnalyze(query), nil
	}

	return qa.parseIntentResponse(response, query)
}

func (qa *QueryAnalyzer) parseIntentResponse(response, query string) (*QueryIntent, error) {
	var intent QueryIntent

	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 {
		return qa.ruleBasedAnalyze(query), nil
	}

	jsonStr := response[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), &intent); err != nil {
		return qa.ruleBasedAnalyze(query), nil
	}

	if intent.RetrievalDepth == 0 {
		intent.RetrievalDepth = qa.baseK
	}

	return &intent, nil
}

func (qa *QueryAnalyzer) ruleBasedAnalyze(query string) *QueryIntent {
	queryLower := strings.ToLower(query)

	intent := &QueryIntent{
		Keywords:       extractKeywords(query),
		Entities:       extractEntities(query),
		Complexity:     0.5,
		RetrievalDepth: qa.baseK,
	}

	if strings.HasPrefix(queryLower, "what") || strings.HasPrefix(queryLower, "who") {
		intent.Type = "factual"
		intent.Reasoning = "Direct fact lookup"
		intent.RetrievalDepth = qa.baseK
	} else if strings.HasPrefix(queryLower, "when") || containsTimeKeywords(queryLower) {
		intent.Type = "temporal"
		intent.Reasoning = "Time-based query"
		intent.RetrievalDepth = qa.baseK + 3
		intent.TimeRange = &TimeRange{
			Start: nil,
			End:   nil,
		}
	} else if strings.HasPrefix(queryLower, "why") || strings.HasPrefix(queryLower, "how") {
		intent.Type = "reasoning"
		intent.Reasoning = "Requires reasoning"
		intent.Complexity = 0.7
		intent.RetrievalDepth = qa.baseK * 2
	} else if containsAggregationKeywords(queryLower) {
		intent.Type = "aggregation"
		intent.Reasoning = "Requires aggregation"
		intent.Complexity = 0.8
		intent.RetrievalDepth = qa.baseK * 3
	} else {
		intent.Type = "factual"
		intent.Reasoning = "Default fact lookup"
	}

	return intent
}

func extractKeywords(query string) []string {
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "to": true, "of": true,
		"in": true, "for": true, "on": true, "with": true, "at": true,
		"by": true, "from": true, "as": true, "into": true, "through": true,
		"about": true, "what": true, "which": true, "who": true, "whom": true,
		"this": true, "that": true, "these": true, "those": true,
	}

	words := strings.Fields(query)
	var keywords []string
	for _, word := range words {
		wordLower := strings.ToLower(word)
		if len(word) > 2 && !stopWords[wordLower] {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

func extractEntities(query string) []string {
	words := strings.Fields(query)
	var entities []string

	for i, word := range words {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			entity := strings.Trim(word, ".,!?;:")
			if len(entity) > 1 && (i == 0 || (i > 0 && words[i-1] != "'s")) {
				entities = append(entities, entity)
			}
		}
	}
	return entities
}

func containsTimeKeywords(query string) bool {
	timeKeywords := []string{
		"before", "after", "between", "during", "since",
		"until", "when", "earlier", "later", "first", "last",
		"yesterday", "today", "tomorrow", "week", "month", "year",
	}
	for _, kw := range timeKeywords {
		if strings.Contains(query, kw) {
			return true
		}
	}
	return false
}

func containsAggregationKeywords(query string) bool {
	aggKeywords := []string{
		"how many", "how much", "count", "total", "sum",
		"average", "all", "every", "most", "list", "summarize",
	}
	for _, kw := range aggKeywords {
		if strings.Contains(query, kw) {
			return true
		}
	}
	return false
}

func CalculateDynamicK(baseK int, complexity float64, delta float64) int {
	if delta == 0 {
		delta = 2.0
	}
	k := float64(baseK) * (1.0 + delta*complexity)
	return int(k)
}
