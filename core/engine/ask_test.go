package engine

import (
	"context"
	"errors"
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
