package main

import (
	"context"
	"fmt"
	"log"
	"memflow"
	"memflow/core/embedder"
	"memflow/core/llm"
	"time"
)

func main() {
	// 1. LM Studio 配置 (默认地址 http://localhost:1234/v1)
	// 在 LM Studio 的 AI Chat -> Start Server 开启
	lmStudioBaseURL := "http://192.168.2.78:1234/v1"
	modelName := "qwen/qwen3-vl-8b" // 或者你加载的任何模型名

	// 2. 初始化嵌入客户端 (Embedding)
	// LM Studio 通常也支持嵌入端点
	emb := embedder.NewOpenAIEmbedder(lmStudioBaseURL, "not-needed", "text-embedding-qwen3-embedding-4b")

	// 3. 初始化 LLM 客户端
	llmClient := llm.NewOpenAILLMClient(lmStudioBaseURL, "not-needed", modelName)

	// 4. 配置 SimpleMem
	config := memflow.Config{
		LongTermImportanceThreshold: 0.7,  // 重要性高于 0.7 的对话会自动存入长期记忆
		EnableHybridSearch:          true, // 开启混合搜索
	}

	// 5. 创建 SimpleMem 客户端
	client := memflow.NewWithConfig(config, emb)

	// 设置真实 LLM，开启“基于 LLM 的重要性评估器”
	client.SetLLMClient(llmClient)
	client.SetImportanceEstimator(memflow.NewImportanceEstimatorByLLM(llmClient))

	ctx := context.Background()
	namespace := "lm_studio_test_user"

	fmt.Println("--- 集成测试开始 (LM Studio) ---")
	fmt.Printf("加载模型: %s\n", modelName)

	fmt.Println("步骤 0: 验证 Embedding 接口...")
	testVec, err := emb.Embed(ctx, "test")
	if err != nil {
		log.Fatalf("Embedding 失败: %v", err)
	}
	fmt.Printf("Embedding 成功，维度: %d, 前 3 位: %v\n", len(testVec), testVec[:3])

	fmt.Println("步骤 1: 模拟对话对话流...")

	dialogues := []memflow.Dialogue{
		{
			ID:        "d1",
			Speaker:   "用户",
			Content:   "你好，我明天的日程是什么？",
			Timestamp: time.Now(),
		},
		{
			ID:        "d2",
			Speaker:   "助手",
			Content:   "你明天下午 2 点在星巴克有一个关于项目的报告会议。",
			Timestamp: time.Now().Add(time.Second),
		},
	}

	// 添加对话。SimpleMem 会调用 LM Studio 评估每一行的重要性。
	err = client.AddDialogues(ctx, namespace, dialogues)
	if err != nil {
		log.Fatalf("添加对话失败，请检查 LM Studio 服务是否启动: %v", err)
	}
	client.RebuildIndex(namespace)

	fmt.Println("步骤 2: 验证记忆存储...")
	// 查看所有被识别为记忆的条目
	memories, err := client.Get(namespace)
	if err != nil {
		log.Printf("读取记忆失败: %v", err)
	} else {
		for _, m := range memories {
			fmt.Printf(">> 识别出的记忆: [%s] (重要性: %.2f) 内容: %s\n", m.ID, m.Importance, m.Content)
		}
	}

	fmt.Println("\n步骤 3: 验证记忆检索 (Search)...")
	question := "我明天要去哪里开会？目的是什么？"
	results, err := client.Search(ctx, namespace, question)
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
	} else {
		fmt.Printf("找到相关记忆数量: %d\n", len(results))
		for _, r := range results {
			fmt.Printf("- [%s] (重要性: %.2f) 内容: %s\n", r.ID, r.Importance, r.Content)
		}
	}

	fmt.Println("\n步骤 4: 提问 (基于记忆召回结果交给 LM Studio 总结)...")
	fmt.Printf("提问: %s\n", question)

	answer, err := client.Ask(ctx, namespace, question)
	if err != nil {
		log.Printf("问答失败: %v", err)
	} else {
		fmt.Printf("LM Studio 回复: %s\n", answer)
	}

	fmt.Println("\n步骤 5: 额外验证 (直接测试 Add 和 Search 方法)...")
	testNS := "manual_test_ns"
	client.AddWithImportance(ctx, testNS, "The secret code is 123456", 0.9)
	client.RebuildIndex(testNS)
	manualResults, _ := client.Search(ctx, testNS, "secret code")
	fmt.Printf("手动验证检索结果数量: %d\n", len(manualResults))
	for _, r := range manualResults {
		fmt.Printf("- 找到: %s\n", r.Content)
	}

	fmt.Println("\n--- 集成测试结束 ---")
}
