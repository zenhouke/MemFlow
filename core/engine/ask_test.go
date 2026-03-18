package engine

import (
	"context"
	"encoding/json"
	"errors"
	"memflow/core/vectorstore"
	"strings"
	"testing"
	"time"
)

func newAskTestEngineSimple(embed *fakeEmbedder) *MemoryEngine {
	cfg := newTestConfig()
	cfg.EnableHybridSearch = false
	return newEngineWithConfigAndNow(cfg, embed, time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC))
}

func TestAsk_NoLLMClient_ReturnsError(t *testing.T) {
	eng := newAskTestEngineSimple(&fakeEmbedder{})

	_, err := eng.Ask(context.Background(), "ns", "question")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got, want := err.Error(), "LLM client not set"; got != want {
		t.Fatalf("unexpected error message: got %q want %q", got, want)
	}
}

func TestAsk_SearchError_Propagates(t *testing.T) {
	searchErr := errors.New("search failed")
	eng := newAskTestEngineSimple(&fakeEmbedder{err: searchErr})
	llmClient := &fakeLLMClient{response: "ok"}
	eng.SetLLMClient(llmClient)

	_, err := eng.Ask(context.Background(), "ns", "question")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != searchErr {
		t.Fatalf("expected exact search error passthrough, got %v", err)
	}
	if llmClient.chatCallCount != 0 {
		t.Fatalf("expected llm chat not called, got %d", llmClient.chatCallCount)
	}
}

func TestAsk_NoRelevantMemories_ReturnsFallback(t *testing.T) {
	eng := newAskTestEngineSimple(&fakeEmbedder{vectors: map[string][]float64{
		"question": {1, 0},
	}})
	llmClient := &fakeLLMClient{response: "should not be used"}
	eng.SetLLMClient(llmClient)

	got, err := eng.Ask(context.Background(), "ns", "question")
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if want := "No relevant memories found."; got != want {
		t.Fatalf("unexpected fallback text: got %q want %q", got, want)
	}
	if llmClient.chatCallCount != 0 {
		t.Fatalf("expected llm chat not called, got %d", llmClient.chatCallCount)
	}
}

func TestAsk_Success_ReturnsLLMAnswer(t *testing.T) {
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"question":        {1, 0},
		"matching memory": {1, 0},
	}}
	eng := newAskTestEngineSimple(emb)
	llmResponse := "Direct LLM answer with punctuation: yes."
	llmClient := &fakeLLMClient{response: llmResponse}
	eng.SetLLMClient(llmClient)

	if err := eng.Add(context.Background(), "ns", "matching memory", 0.3); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got, err := eng.Ask(context.Background(), "ns", "question")
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if got != llmResponse {
		t.Fatalf("expected unchanged llm answer passthrough: got %q want %q", got, llmResponse)
	}
	if llmClient.chatCallCount != 1 {
		t.Fatalf("expected llm chat called once, got %d", llmClient.chatCallCount)
	}
}

func TestAsk_PromptContainsRequiredFragments(t *testing.T) {
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"what should I remember?": {1.0, 0.0},
		"first memory summary":    {1.0, 0.0},
		"second memory summary":   {0.4, 0.0},
	}}
	eng := newAskTestEngineSimple(emb)
	llmClient := &fakeLLMClient{response: "ok"}
	eng.SetLLMClient(llmClient)

	if err := eng.Add(context.Background(), "ns", "first memory summary", 0.3); err != nil {
		t.Fatalf("Add first memory failed: %v", err)
	}
	if err := eng.Add(context.Background(), "ns", "second memory summary", 0.3); err != nil {
		t.Fatalf("Add second memory failed: %v", err)
	}

	if _, err := eng.Ask(context.Background(), "ns", "what should I remember?"); err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	assertAskPromptMessageContract(t, llmClient)

	userPrompt := llmClient.lastMessages[1].Content
	required := []string{
		"Based on the following memory context, answer the question.",
		"Memory Context:",
		"[1] first memory summary",
		"[2] second memory summary",
		"Question: what should I remember?",
		"Answer:",
	}
	for _, fragment := range required {
		if !strings.Contains(userPrompt, fragment) {
			t.Fatalf("user prompt missing required fragment %q\nprompt: %q", fragment, userPrompt)
		}
	}
}

func TestAsk_PromptIncludesSourceWhenOriginalContentPresent(t *testing.T) {
	emb := &fakeEmbedder{vectors: map[string][]float64{
		"where did this come from?": {1.0, 0.0},
	}}
	eng := newAskTestEngineSimple(emb)
	llmClient := &fakeLLMClient{response: "ok"}
	eng.SetLLMClient(llmClient)

	now := time.Date(2026, time.March, 18, 10, 0, 0, 0, time.UTC)
	itemOne := &MemoryItem{
		ID:              "mem-1",
		Content:         "first memory summary",
		OriginalContent: "Original source sentence for memory one.",
		Embedding:       []float64{1.0, 0.0},
		Importance:      0.3,
		CreatedAt:       now,
		LastAccessedAt:  now,
	}
	itemOneJSON, err := json.Marshal(itemOne)
	if err != nil {
		t.Fatalf("marshal first memory item: %v", err)
	}

	itemTwo := &MemoryItem{
		ID:             "mem-2",
		Content:        "second memory summary",
		Embedding:      []float64{0.4, 0.0},
		Importance:     0.3,
		CreatedAt:      now,
		LastAccessedAt: now,
	}
	itemTwoJSON, err := json.Marshal(itemTwo)
	if err != nil {
		t.Fatalf("marshal second memory item: %v", err)
	}

	eng.store = &fakeVectorStore{
		searchResults: []vectorstore.SearchResult{
			{
				ID:    "mem-1",
				Score: 0.99,
				Payload: map[string]interface{}{
					"item_json": string(itemOneJSON),
					"content":   itemOne.Content,
				},
			},
			{
				ID:    "mem-2",
				Score: 0.65,
				Payload: map[string]interface{}{
					"item_json": string(itemTwoJSON),
					"content":   itemTwo.Content,
				},
			},
		},
	}

	if _, err := eng.Ask(context.Background(), "ns", "where did this come from?"); err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	assertAskPromptMessageContract(t, llmClient)

	userPrompt := llmClient.lastMessages[1].Content
	required := []string{
		"[1] first memory summary",
		"Source: Original source sentence for memory one.",
		"[2] second memory summary",
	}
	for _, fragment := range required {
		if !strings.Contains(userPrompt, fragment) {
			t.Fatalf("user prompt missing required fragment %q\nprompt: %q", fragment, userPrompt)
		}
	}
}

func assertAskPromptMessageContract(t *testing.T, llmClient *fakeLLMClient) {
	t.Helper()

	if llmClient.chatCallCount != 1 {
		t.Fatalf("expected llm chat called once, got %d", llmClient.chatCallCount)
	}
	if got := len(llmClient.lastMessages); got != 2 {
		t.Fatalf("expected exactly 2 llm messages, got %d", got)
	}
	if got, want := llmClient.lastMessages[0].Role, "system"; got != want {
		t.Fatalf("unexpected first message role: got %q want %q", got, want)
	}
	if !strings.Contains(llmClient.lastMessages[0].Content, "answers questions based on the provided memory context") {
		t.Fatalf("unexpected system message content: %q", llmClient.lastMessages[0].Content)
	}
	if got, want := llmClient.lastMessages[1].Role, "user"; got != want {
		t.Fatalf("unexpected second message role: got %q want %q", got, want)
	}
}
