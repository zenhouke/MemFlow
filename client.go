package simplemem

import (
	"context"
	"simplemem/core/compression"
	"simplemem/core/config"
	"simplemem/core/embedder"
	"simplemem/core/engine"
	"simplemem/core/index"
	"simplemem/core/llm"
	"simplemem/core/summarizer"
	"time"
)

type MemoryItem = engine.MemoryItem

type Embedder = embedder.Embedder

type Config = config.Config

type ImportanceEstimator = engine.ImportanceEstimator

type Summarizer = summarizer.Summarizer

type Dialogue = compression.Dialogue

type Metadata = index.Metadata

type CompressionConfig = config.CompressionConfig

type LLMClient = llm.LLMClient

type LLMMessage = llm.Message

type Client struct {
	engine *engine.MemoryEngine
}

func New(embedder Embedder) *Client {
	cfg := config.DefaultConfig()
	return NewWithConfig(cfg, embedder)
}

func NewWithConfig(cfg config.Config, embedder Embedder) *Client {
	return &Client{
		engine: engine.New(cfg, embedder),
	}
}

func NewSemanticCompressor(cfg CompressionConfig, llmClient LLMClient) *compression.SemanticCompressor {
	return compression.NewSemanticCompressor(cfg, llmClient)
}

func (c *Client) SetLLMClient(client LLMClient) {
	c.engine.SetLLMClient(client)
}

func (c *Client) SetImportanceEstimator(e ImportanceEstimator) {
	c.engine.SetImportanceEstimator(e)
}

func (c *Client) SetSummarizer(s Summarizer) {
	c.engine.SetSummarizer(s)
}

func (c *Client) SetCompressor(compressor *compression.SemanticCompressor) {
	c.engine.SetCompressor(compressor)
}

func (c *Client) Add(ctx context.Context, namespace, content string) error {
	return c.engine.Add(ctx, namespace, content, 0)
}

func (c *Client) AddWithImportance(ctx context.Context, namespace, content string, importance float64) error {
	return c.engine.Add(ctx, namespace, content, importance)
}

func (c *Client) AddDialogue(ctx context.Context, namespace, speaker, content string, timestamp time.Time) error {
	return c.engine.AddDialogue(ctx, namespace, speaker, content, timestamp)
}

func (c *Client) AddDialogues(ctx context.Context, namespace string, dialogues []Dialogue) error {
	return c.engine.AddDialogues(ctx, namespace, dialogues)
}

func (c *Client) Search(ctx context.Context, namespace, query string) ([]*MemoryItem, error) {
	return c.engine.Search(ctx, namespace, query)
}

func (c *Client) SearchWithLimit(ctx context.Context, namespace, query string, limit int) ([]*MemoryItem, error) {
	results, err := c.engine.Search(ctx, namespace, query)
	if err != nil {
		return nil, err
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *Client) Ask(ctx context.Context, namespace, question string) (string, error) {
	return c.engine.Ask(ctx, namespace, question)
}

func (c *Client) Get(namespace string) ([]*MemoryItem, error) {
	return nil, nil
}

func (c *Client) Delete(namespace, id string) error {
	return nil
}

func (c *Client) Save(path string) error {
	return c.engine.SaveToFile(path)
}

func (c *Client) Load(path string) error {
	return c.engine.LoadFromFile(path)
}
