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
