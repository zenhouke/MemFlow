# MemFlow Project Technical Documentation

## Overview

MemFlow is a high-performance Go SDK for LLM agent memory management. Evolved from the [SimpleMem](https://github.com/aiming-lab/SimpleMem) research, it provides a "production-ready" implementation of lifelong memory with automated tiering and semantic compression.

### Key Features

1. **Deep Semantic Compression** - Automatically extracts "Memory Units" and resolves coreferences to save up to 80% of context tokens
2. **Automated Tiering** - 
   - **Hot Memory**: HNSW-indexed in-memory storage for immediate context
   - **Cold Memory**: Seamlessly synced to Qdrant, Milvus, or LanceDB
3. **Hybrid Retrieval** - Combines Semantic (Vector), Lexical (BM25), and Metadata filtering for maximum recall
4. **Concurrency-First** - Sharded RWMutex design optimized for high-throughput Agentic workflows

## Architecture Design

```
[Dialogue Stream] --> [MemFlow SDK]
    ↓
[Semantic Compressor]
    ↓
[Importance Scorer] --> [Long-term: Vector DB]
    ↓
[Short-term: HNSW/Memory]
    ↓
[Hybrid Search] --> [Condensed Context]
```

## Core Components

### 1. Client Interface (client.go)
```go
// Main API interface
func New(embedder Embedder) *Client
func (c *Client) AddDialogue(ctx context.Context, namespace, speaker, content string, timestamp time.Time) error
func (c *Client) Ask(ctx context.Context, namespace, question string) (string, error)
func (c *Client) Search(ctx context.Context, namespace, query string) ([]*MemoryItem, error)
```

### 2. Core Engine (core/engine/engine.go)
- Memory space management
- Dialog storage and retrieval
- Compression and tiering logic
- Query processing

### 3. Semantic Compression (core/compression/compressor.go)
- Dialog windowing processing
- Coreference resolution
- Memory unit extraction
- Importance scoring

### 4. Retrieval System (core/retrieval/)
- Query intent analysis
- Hybrid retrieval strategy
- Time range parsing
- Complexity evaluation

### 5. Vector Store (core/vectorstore/)
- Multiple vector database support (Qdrant, Milvus, LanceDB)
- Memory storage mode
- Local persistence

## Usage Example

```go
package main

import (
    "context"
    "time"
    "memflow"
)

func main() {
    ctx := context.Background()
    
    // Initialize client
    client := memflow.New(&MyEmbedder{})
    client.SetLLMClient(&MyLLMClient{})

    // Add dialogue - MemFlow handles compression & tiering behind the scenes
    client.AddDialogue(ctx, "session_001", "Alice", "I'm planning a trip to Tokyo next May.", time.Now())

    // Context-aware retrieval
    answer, _ := client.Ask(ctx, "session_001", "Where is Alice going?")
    println(answer)
}
```

## Configuration Options

### Compression Configuration (CompressionConfig)
- WindowSize: Dialog window size
- OverlapSize: Window overlap size

### Vector Store Configuration (VectorStoreConfig)
- Type: Storage type (memory, qdrant, milvus, lancedb)
- Address: Database address
- CollectionName: Collection name

## Performance Advantages

| Feature | Raw Vector DB | MemFlow |
| :--- | :--- | :--- |
| **Token Usage** | High (Uncompressed) | Low (Extracted Units) |
| **Noise** | High (Includes 'ums' and 'errs') | Low (Pure Semantic Content) |
| **Scaling** | Search slows with size | O(1) Local Cache + Fast DB Sync |
| **Setup** | Complex Pipeline | Single Go Client |

## Technology Stack

- Go 1.24+
- HNSW indexing (coder/hnsw)
- Qdrant client (github.com/qdrant/go-client)
- Milvus SDK (github.com/milvus-io/milvus-sdk-go/v2)
- UUID library (github.com/google/uuid)

## Deployment Instructions

1. Install dependencies:
   ```bash
   go get github.com/zenhouke/memflow-go
   ```

2. Configure vector database (optional):
   - Qdrant
   - Milvus
   - LanceDB

3. Implement embedder interface:
   ```go
   type MyEmbedder struct{}
   
   func (e *MyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
       // Implement embedding logic
   }
   ```

4. Implement LLM client interface (optional):
   ```go
   type MyLLMClient struct{}
   
   func (c *MyLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
       // Implement chat logic
   }
   ```

## Workflow

1. Dialog input → Semantic compressor
2. Compressed content → Importance scorer
3. Importance score determines → Storage location (hot/cold)
4. Query request → Hybrid search
5. Return condensed context

## Best Practices

### 1. Dialog Management
- Use AddDialogue method for dialogs
- Ensure accurate timestamps
- Utilize namespaces to distinguish different contexts

### 2. Compression Parameter Optimization
- Adjust window size based on dialog complexity
- Tune overlap size to avoid information loss
- Monitor importance score thresholds

### 3. Storage Strategy
- Hot memory: Keep frequently accessed content in memory
- Cold memory: Persist to vector database
- Regular cleanup of outdated memories

## Troubleshooting

### Common Issues
1. **Performance problems** - Check vector store configuration
2. **Memory leaks** - Ensure proper resource and connection cleanup
3. **Poor compression** - Adjust compression parameters and LLM prompts

### Logging Monitoring
- Log compression success rates
- Monitor vector store connection status
- Track query performance metrics