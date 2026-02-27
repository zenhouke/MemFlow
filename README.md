# SimpleMem-Go

SimpleMem-Go is a high-performance Go implementation of the [SimpleMem](https://github.com/aiming-lab/SimpleMem) paper. It provides an efficient lifelong memory system for LLM agents, featuring semantic compression, hybrid retrieval, and automated memory tiering.

## 🌟 Key Features

### 1. Deep Semantic Compression
- **Sliding Window Processing**: Efficiently handles dialogue streams (default WindowSize=10).
- **End-to-End Extraction**: Extracts memory units, resolves coreferences, and **simultaneously evaluates importance** in a single LLM call to drastically save tokens.
- **Redundancy Control**: References previous window entries to avoid duplicate memory generation.

### 2. Automated Memory Tiering
- **Hierarchical Storage**: Automatically routes information based on an Importance Score.
    - **Short-term Memory**: Resides in high-performance in-memory indexes with rapid decay.
    - **Long-term Memory**: Automatically synced to external vector databases (Qdrant, Milvus, LanceDB) for permanent storage.
- **Asynchronous Compaction**: A background Goroutine performs recursive consolidation, merging similar memories and extracting higher-level abstractions without blocking the main workflow.

### 3. High-Performance Hybrid Retrieval
- **Adaptive Intent Analysis**: The system analyzes query intent (factual, temporal, reasoning, aggregation) and dynamically adjusts retrieval depth (Top-K).
- **Token-Saving "Fast Path"**: Automatically triggers local rule-based analysis for short or simple keyword queries, skipping expensive LLM calls.
- **Triple-Layer Indexing**:
    - **Semantic**: Vector similarity search based on HNSW.
    - **Lexical**: Text search based on an optimized BM25 (O(1) incremental updates).
    - **Symbolic/Metadata**: Hard filtering for time ranges, entities, and topics.

### 4. Production-Ready Engineering
- **Fine-Grained Concurrency**: Namespace-level sharded RWMutex supports high-concurrency operations.
- **Persistence & Integrity**: Supports full state snapshots with **automatic index rebuilding** upon loading to ensure zero-downtime search readiness.
- **Pluggable Architecture**: Easily swap LLM clients, Embedders, and Vector Store backends.

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/zenhouke/simplemem-go
```

### Basic Usage (In-Memory Mode)

```go
package main

import (
    "context"
    "time"
    "simplemem"
)

func main() {
    ctx := context.Background()

    // 1. Initialize client (Implement the Embedder interface)
    client := simplemem.New(&MyEmbedder{})
    
    // 2. Configure LLM client and intelligent components
    llmClient := &MyLLMClient{}
    client.SetLLMClient(llmClient)
    
    // Enable LLM-based importance estimator
    client.SetImportanceEstimator(simplemem.NewImportanceEstimatorByLLM(llmClient))

    // 3. Add dialogues (System handles compression and tiering automatically)
    client.AddDialogue(ctx, "chat_001", "Alice", "My birthday is May 12th.", time.Now())
    client.AddDialogue(ctx, "chat_001", "Bob", "Got it, I'll remember that.", time.Now())

    // 4. Perform Retrieval-Augmented Generation (RAG)
    answer, _ := client.Ask(ctx, "chat_001", "When is Alice's birthday?")
    println("AI Answer:", answer)
}
```

### Advanced Usage (External Database Mode)

Configure `VectorStoreConfig` to automatically persist high-value memories to an external store like Qdrant:

```go
cfg := simplemem.Config{
    LongTermImportanceThreshold: 0.6, // Memories with score > 0.6 go to DB
    VectorStoreConfig: simplemem.VectorStoreConfig{
        Type:           "qdrant",
        Host:           "localhost",
        Port:           6334,
        CollectionName: "agent_memories",
        Dimension:      1536,
    },
}

client := simplemem.NewWithConfig(cfg, &MyEmbedder{})
```

---

## 🛠 Configuration Parameters

| Parameter | Default | Description |
| :--- | :--- | :--- |
| `ShortTermDecay` | 0.02 | Decay rate for memories in short-term storage. |
| `LongTermImportanceThreshold` | 0.7 | Score required to promote memory to long-term storage. |
| `CompactionThreshold` | 10 | Number of items that trigger background consolidation. |
| `EnableHybridSearch` | true | Enable combined semantic + lexical + symbolic search. |
| `VectorStoreConfig.Type` | "memory" | Storage backend type (memory, qdrant, milvus, lancedb). |

---

## 📜 Citation

If you use this project in your research, please cite the original SimpleMem paper:

```bibtex
@article{simplemem2026,
  title={SimpleMem: Efficient Lifelong Memory for LLM Agents},
  author={Aiming Lab},
  journal={arXiv preprint arXiv:2601.02553},
  year={2026}
}
```
