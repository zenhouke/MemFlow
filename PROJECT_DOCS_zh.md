# MemFlow 项目技术文档

## 概述

MemFlow 是一个高性能的 Go SDK，用于 LLM 代理的记忆管理。它从 [SimpleMem](https://github.com/aiming-lab/SimpleMem) 研究演进而来，提供了一种"生产就绪"的终身记忆实现，具有自动分层和语义压缩功能。

### 核心特性

1. **深度语义压缩** - 自动提取"记忆单元"并解决共指，可节省高达 80% 的上下文 token
2. **自动化分层** - 热记忆（内存中的 HNSW 索引）和冷记忆（Qdrant、Milvus 或 LanceDB）
3. **混合检索** - 结合语义（向量）、字面（BM25）和元数据过滤以获得最大召回率
4. **并发优先** - 优化的分片 RWMutex 设计，针对高吞吐量代理工作流

## 架构设计

```
[对话流] --> [MemFlow SDK]
    ↓
[语义压缩器]
    ↓
[重要性评分器] --> [长期存储: 向量数据库]
    ↓
[短期存储: HNSW/内存]
    ↓
[混合检索] --> [精炼上下文]
```

## 核心组件

### 1. 客户端接口 (client.go)
```go
// 主要 API 接口
func New(embedder Embedder) *Client
func (c *Client) AddDialogue(ctx context.Context, namespace, speaker, content string, timestamp time.Time) error
func (c *Client) Ask(ctx context.Context, namespace, question string) (string, error)
func (c *Client) Search(ctx context.Context, namespace, query string) ([]*MemoryItem, error)
```

### 2. 核心引擎 (core/engine/engine.go)
- 内存空间管理
- 对话存储和检索
- 压缩和分层逻辑
- 查询处理

### 3. 语义压缩 (core/compression/compressor.go)
- 对话窗口化处理
- 共指消除
- 记忆单元提取
- 重要性评分

### 4. 检索系统 (core/retrieval/)
- 查询意图分析
- 混合检索策略
- 时间范围解析
- 复杂度评估

### 5. 向量存储 (core/vectorstore/)
- 多种向量数据库支持 (Qdrant, Milvus, LanceDB)
- 内存存储模式
- 本地持久化

## 使用示例

```go
package main

import (
    "context"
    "time"
    "memflow"
)

func main() {
    ctx := context.Background()
    
    // 初始化客户端
    client := memflow.New(&MyEmbedder{})
    client.SetLLMClient(&MyLLMClient{})

    // 添加对话 - MemFlow 自动处理压缩和分层
    client.AddDialogue(ctx, "session_001", "Alice", "I'm planning a trip to Tokyo next May.", time.Now())

    // 上下文感知检索
    answer, _ := client.Ask(ctx, "session_001", "Where is Alice going?")
    println(answer)
}
```

## 配置选项

### 压缩配置 (CompressionConfig)
- WindowSize: 对话窗口大小
- OverlapSize: 窗口重叠大小

### 向量存储配置 (VectorStoreConfig)
- Type: 存储类型 (memory, qdrant, milvus, lancedb)
- Address: 数据库地址
- CollectionName: 集合名称

## 性能优势

| 特性 | 原始向量数据库 | MemFlow |
|------|----------------|---------|
| Token 使用量 | 高 (未压缩) | 低 (提取单元) |
| 噪音 | 高 (包含 'ums' 和 'errs') | 低 (纯语义内容) |
| 扩展性 | 随规模增长而变慢 | O(1) 本地缓存 + 快速数据库同步 |
| 设置难度 | 复杂管道 | 单个 Go 客户端 |

## 技术栈

- Go 1.24+
- HNSW 索引 (coder/hnsw)
- Qdrant 客户端 (github.com/qdrant/go-client)
- Milvus SDK (github.com/milvus-io/milvus-sdk-go/v2)
- UUID 库 (github.com/google/uuid)

## 部署说明

1. 安装依赖:
   ```bash
   go get github.com/zenhouke/memflow-go
   ```

2. 配置向量数据库 (可选):
   - Qdrant
   - Milvus
   - LanceDB

3. 实现嵌入器接口:
   ```go
   type MyEmbedder struct{}
   
   func (e *MyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
       // 实现嵌入逻辑
   }
   ```

4. 实现 LLM 客户端接口 (可选):
   ```go
   type MyLLMClient struct{}
   
   func (c *MyLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
       // 实现聊天逻辑
   }
   ```

## 工作流程

1. 对话输入 → 语义压缩器
2. 压缩后的内容 → 重要性评分器
3. 重要性评分决定 → 存储位置 (热/冷)
4. 查询请求 → 混合检索
5. 返回精炼上下文

## 最佳实践

### 1. 对话管理
- 使用 AddDialogue 方法添加对话
- 确保时间戳准确性
- 利用命名空间区分不同上下文

### 2. 压缩参数优化
- 根据对话复杂度调整窗口大小
- 调整重叠大小避免信息丢失
- 监控重要性评分阈值

### 3. 存储策略
- 热记忆：常用内容保持在内存中
- 冷记忆：通过向量数据库持久化
- 定期清理已过时记忆

## 故障排除

### 常见问题
1. **性能问题** - 检查是否正确设置了向量存储
2. **内存泄漏** - 确保释放所有资源和连接
3. **压缩效果差** - 调整压缩参数和 LLM 提示词

### 日志监控
- 记录压缩成功率
- 监控向量存储连接状态
- 追踪查询性能指标