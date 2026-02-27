package engine

import (
	"simplemem/core/compression"
	"simplemem/core/config"
	"simplemem/core/embedder"
	"simplemem/core/llm"
	"simplemem/core/summarizer"
	"sync"
)

type MemoryEngine struct {
	spaces     map[string]*MemorySpace
	embedder   embedder.Embedder
	llmClient  llm.LLMClient
	estimator  ImportanceEstimator
	summarizer summarizer.Summarizer
	compressor *compression.SemanticCompressor
	config     config.Config
	mu         sync.RWMutex
}

func New(cfg config.Config, embedder embedder.Embedder) *MemoryEngine {
	return &MemoryEngine{
		spaces:   make(map[string]*MemorySpace),
		embedder: embedder,
		config:   cfg,
	}
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
