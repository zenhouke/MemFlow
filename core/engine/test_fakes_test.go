package engine

import (
	"context"
	"errors"
	"memflow/core/config"
	"memflow/core/llm"
	"memflow/core/vectorstore"
	"time"
)

type fakeEmbedder struct {
	vectors map[string][]float64
	fixed   []float64
	err     error
	calls   int
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if v, ok := f.vectors[text]; ok {
		return append([]float64(nil), v...), nil
	}
	if f.fixed != nil {
		return append([]float64(nil), f.fixed...), nil
	}
	return []float64{0.1, 0.1}, nil
}

type fakeEstimator struct {
	value float64
	err   error
	calls int
}

type fakeLLMClient struct {
	response      string
	err           error
	chatCallCount int
	lastMessages  []llm.Message
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	f.chatCallCount++
	f.lastMessages = append([]llm.Message(nil), messages...)
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func (f *fakeEstimator) Estimate(ctx context.Context, content string) (float64, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	return f.value, nil
}

type fakeVectorStore struct {
	added         []vectorstore.VectorRecord
	searchResults []vectorstore.SearchResult
	addErr        error
	searchErr     error
}

func (f *fakeVectorStore) Add(ctx context.Context, vectors []vectorstore.VectorRecord) error {
	if f.addErr != nil {
		return f.addErr
	}
	for _, v := range vectors {
		rec := vectorstore.VectorRecord{
			ID:     v.ID,
			Vector: append([]float32(nil), v.Vector...),
		}
		if v.Payload != nil {
			rec.Payload = make(map[string]interface{}, len(v.Payload))
			for k, val := range v.Payload {
				rec.Payload[k] = val
			}
		}
		f.added = append(f.added, rec)
	}
	return nil
}

func (f *fakeVectorStore) Search(ctx context.Context, query []float32, topK int, filter *vectorstore.Filter) ([]vectorstore.SearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if topK <= 0 || topK >= len(f.searchResults) {
		return append([]vectorstore.SearchResult(nil), f.searchResults...), nil
	}
	return append([]vectorstore.SearchResult(nil), f.searchResults[:topK]...), nil
}

func (f *fakeVectorStore) Delete(ctx context.Context, ids []string) error { return nil }
func (f *fakeVectorStore) Upsert(ctx context.Context, vectors []vectorstore.VectorRecord) error {
	return errors.New("not implemented in tests")
}
func (f *fakeVectorStore) Close() error { return nil }

func newTestConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.EnableHybridSearch = false
	cfg.TopK = 3
	cfg.CompactionThreshold = 1000
	cfg.Alpha = 1.0
	cfg.Beta = 0.0
	cfg.Gamma = 0.0
	return cfg
}

func newEngineWithConfigAndNow(cfg config.Config, embed *fakeEmbedder, now time.Time) *MemoryEngine {
	if embed == nil {
		embed = &fakeEmbedder{}
	}
	eng := New(cfg, embed)
	eng.nowFn = func() time.Time { return now }
	eng.disableAsyncCompaction = true
	return eng
}

func newTestEngineWithNow(embed *fakeEmbedder, now time.Time) *MemoryEngine {
	return newEngineWithConfigAndNow(newTestConfig(), embed, now)
}

func newHybridTestEngine(embed *fakeEmbedder, now time.Time) *MemoryEngine {
	if embed == nil {
		embed = &fakeEmbedder{}
	}
	cfg := newTestConfig()
	cfg.EnableHybridSearch = true
	cfg.HybridSearchConfig = config.HybridSearchConfig{
		SemanticWeight: 0.6,
		LexicalWeight:  0.3,
		SymbolicWeight: 0.1,
		EnableAdaptive: true,
		BaseK:          5,
		Delta:          2.0,
		MinK:           3,
		MaxK:           20,
	}
	return newEngineWithConfigAndNow(cfg, embed, now)
}
