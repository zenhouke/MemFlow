package main

import (
	"context"
	"fmt"

	"simplemem"
)

// SimpleEmbedder 模拟向量化（1536维，对应 OpenAI 默认维度）
type SimpleEmbedder struct{}

func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vec := make([]float64, 1536)
	for i, r := range text {
		if i < 1536 {
			vec[i] = float64(r) / 255.0
		}
	}
	return vec, nil
}

// MockLLM 模拟 LLM
type MockLLM struct{}

func (l *MockLLM) Chat(ctx context.Context, messages []simplemem.LLMMessage) (string, error) {
	return "Mock Answer based on database records.", nil
}

func main() {
	ctx := context.Background()

	// 1. 配置数据库驱动 (以 Qdrant 为例)
	// 确保你已经启动了 Qdrant: docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
	cfg := simplemem.Config{
		LongTermImportanceThreshold: 0.6, // 降低阈值，让更多条目进入数据库
		VectorStoreConfig: simplemem.VectorStoreConfig{
			Type:           "qdrant",
			Host:           "localhost",
			Port:           6334,
			CollectionName: "my_memories",
			Dimension:      1536,
		},
	}

	// 2. 初始化客户端
	// 注意：内部会尝试连接 Qdrant，如果连接失败，engine.store 将为 nil，并降级为内存模式
	client := simplemem.NewWithConfig(cfg, &SimpleEmbedder{})
	client.SetLLMClient(&MockLLM{})

	fmt.Println(">>> Database Mode: Initialized with Qdrant config")

	// 3. 写入具有高重要性的记忆
	fmt.Println(">>> Adding high-importance memory (will sync to Qdrant)...")
	namespace := "db_user_1"

	// 手动指定高重要性 (0.8 > 0.6)
	err := client.AddWithImportance(ctx, namespace, "The secret password for the vault is 8888.", 0.8)
	if err != nil {
		fmt.Printf("Add failed: %v (Is Qdrant running?)\n", err)
	}

	// 4. 模拟检索
	// 搜索逻辑会先查内存，如果内存中该 namespace 没被加载（如重启后），
	// 或者内存结果不足，则会触发外部数据库查询。
	fmt.Println("\n>>> Searching (Retrieving from db if not in memory)...")
	results, err := client.Search(ctx, namespace, "password")
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
	} else {
		for _, item := range results {
			fmt.Printf("- Found: %s [ID: %s]\n", item.Content, item.ID)
		}
	}

	fmt.Println("\n>>> Tip: You can check Qdrant Dashboard (http://localhost:6333/dashboard) to see the stored points.")
}
