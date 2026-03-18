package engine

import (
	"memflow/core/compression"
	"memflow/core/config"
	"memflow/core/embedder"
	"memflow/core/llm"
	"memflow/core/summarizer"
	"memflow/core/vectorstore"
	"sync"
	"time"
)

type MemoryEngine struct {
	spaces                 map[string]*MemorySpace
	embedder               embedder.Embedder
	llmClient              llm.LLMClient
	estimator              ImportanceEstimator
	summarizer             summarizer.Summarizer
	compressor             *compression.SemanticCompressor
	config                 config.Config
	store                  vectorstore.VectorStore // 新增：外部向量存储
	nowFn                  func() time.Time
	disableAsyncCompaction bool
	mu                     sync.RWMutex
}

func New(cfg config.Config, embedder embedder.Embedder) *MemoryEngine {
	engine := &MemoryEngine{
		spaces:   make(map[string]*MemorySpace),
		embedder: embedder,
		config:   cfg,
		nowFn:    time.Now,
	}

	// 初始化外部存储（如果配置了且不是内存模式）
	if cfg.VectorStoreConfig.Type != "" && cfg.VectorStoreConfig.Type != "memory" {
		if store, err := vectorstore.New(cfg.VectorStoreConfig); err == nil {
			engine.store = store
		}
	}

	return engine
}

func (m *MemoryEngine) SetLLMClient(client llm.LLMClient) {
	m.llmClient = client
}

func (m *MemoryEngine) GetLLMClient() llm.LLMClient {
	return m.llmClient
}

func (m *MemoryEngine) SetImportanceEstimator(e ImportanceEstimator) {
	m.estimator = e
}

func (m *MemoryEngine) SetSummarizer(s summarizer.Summarizer) {
	m.summarizer = s
}

func (m *MemoryEngine) SetCompressor(c *compression.SemanticCompressor) {
	m.compressor = c
}

func (m *MemoryEngine) RebuildIndex(namespace string) {
	if namespace == "" {
		namespace = "default"
	}

	m.mu.RLock()
	space, ok := m.getSpace(namespace)
	m.mu.RUnlock()

	if ok {
		space.mu.Lock()
		defer space.mu.Unlock()
		space.RebuildIndex()
	}
}

func (m *MemoryEngine) getSpace(namespace string) (*MemorySpace, bool) {
	space, ok := m.spaces[namespace]
	return space, ok
}
