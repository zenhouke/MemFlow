# SimpleMem-Go

Go implementation of [SimpleMem](https://github.com/aiming-lab/SimpleMem) - Efficient Lifelong Memory for LLM Agents.

## Features

### Core Memory System
- ✅ **Stage 1: Semantic Structured Compression**
  - Sliding window processing (WindowSize=10, Overlap=2)
  - LLM-based memory unit extraction
  - Coreference resolution (eliminate pronouns)
  - Temporal normalization (ISO 8601)
  - Avoid duplication (reference previous window)

- ✅ **Stage 2: Recursive Memory Consolidation**
  - Time-semantic joint clustering
  - LLM-based semantic synthesis
  - Short-term / Long-term memory separation
  - Original dialogue preservation for traceability

- ✅ **Stage 3: Adaptive Query-Aware Retrieval**
  - LLM-based intent inference (factual/temporal/reasoning/aggregation)
  - Dynamic retrieval depth
  - Hybrid scoring (semantic + lexical + symbolic)
  - Three-layer indexing (HNSW + BM25 + Metadata)

### Additional Features
- Pluggable embedder interface
- Pluggable LLM client interface
- JSON persistence
- Concurrent-safe operations
- Configurable parameters
- Vector store abstraction (supports InMemory, Qdrant)

## Quick Start

### Installation

```bash
go get github.com/your-repo/simplemem-go
```

### Basic Usage

```go
package main

import (
    "context"
    "time"

    "github.com/sashabaranov/go-openai"
    "github.com/your-repo/simplemem-go"
)

// OpenAI Embedder
type OpenAIEmbedder struct {
    client *openai.Client
}

func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
    return &OpenAIEmbedder{
        client: openai.NewClient(apiKey),
    }
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
    resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
        Model: openai.Embedding3Small,
        Input:  []string{text},
    })
    if err != nil {
        return nil, err
    }
    return resp.Data[0].Embedding, nil
}

// OpenAI LLM
type OpenAILLM struct {
    client *openai.Client
}

func NewOpenAILLM(apiKey string) *OpenAILLM {
    return &OpenAILLM{
        client: openai.NewClient(apiKey),
    }
}

func (l *OpenAILLM) Chat(ctx context.Context, messages []simplemem.LLMMessage) (string, error) {
    var openaiMsgs []openai.ChatCompletionMessage
    for _, m := range messages {
        openaiMsgs = append(openaiMsgs, openai.ChatCompletionMessage{
            Role:    m.Role,
            Content: m.Content,
        })
    }
    resp, err := l.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT4oMini,
        Messages: openaiMsgs,
    })
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}

func main() {
    ctx := context.Background()

    // Initialize with real API clients
    mem := simplemem.New(NewOpenAIEmbedder("your-api-key"))
    mem.SetLLMClient(NewOpenAILLM("your-api-key"))

    // Add dialogue (with automatic compression)
    mem.AddDialogue(ctx, "user_alice", "Alice", "Let's meet at Starbucks tomorrow at 2pm", time.Now())
    mem.AddDialogue(ctx, "user_alice", "Bob", "Sure, I'll bring the report", time.Now())

    // Search
    results, _ := mem.Search(ctx, "user_alice", "meeting")
    for _, r := range results {
        println(r.Content)
    }

    // Ask (retrieve + generate answer)
    answer, _ := mem.Ask(ctx, "user_alice", "When and where is the meeting?")
    println(answer)
}
```

**Install dependencies:**
```bash
go get github.com/sashabaranov/go-openai
```

## Interfaces

### Embedder Interface

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float64, error)
}
```

### LLM Client Interface

```go
type LLMClient interface {
    Chat(ctx context.Context, messages []Message) (string, error)
}

type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
}
```

## Configuration

```go
// Memory Engine Config
cfg := simplemem.Config{
    Alpha: 0.6,  // Semantic similarity weight
    Beta:  0.3,  // Recency weight
    Gamma: 0.1,  // Importance weight
    
    ShortTermDecay: 0.02,   // Short-term memory decay rate
    LongTermDecay:  0.005,  // Long-term memory decay rate
    
    TopK: 5,  // Number of results to return
    
    LongTermImportanceThreshold: 0.7,  // Threshold for long-term storage
    CompactionThreshold:         10,   // Trigger compaction after N items
    
    EnableHybridSearch: true,
    HybridSearchConfig: HybridSearchConfig{
        SemanticWeight:  0.6,
        LexicalWeight:   0.3,
        SymbolicWeight:  0.1,
        BaseK:           5,
        MinK:            3,
        MaxK:            20,
    },

    CompressionConfig: CompressionConfig{
        WindowSize:  10,   // Sliding window size
        OverlapSize: 2,    // Overlap between windows
    },
}
```

**Note**: All configuration is centralized in `simplemem.Config`, including compression settings.


## References

- [SimpleMem Paper](https://arxiv.org/abs/2601.02553)
- [SimpleMem GitHub](https://github.com/aiming-lab/SimpleMem)
