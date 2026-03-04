package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"simplemem"
	"simplemem/core/embedder"
	"simplemem/core/llm"
	"strings"
	"time"
)

func main() {
	// 1. LM Studio 配置
	lmStudioBaseURL := "http://192.168.2.78:1234/v1"
	modelName := "qwen/qwen3-vl-8b"
	embeddingModel := "text-embedding-qwen3-embedding-4b"

	// 2. 初始化客户端
	emb := embedder.NewOpenAIEmbedder(lmStudioBaseURL, "not-needed", embeddingModel)
	llmClient := llm.NewOpenAILLMClient(lmStudioBaseURL, "not-needed", modelName)

	config := simplemem.Config{
		LongTermImportanceThreshold: 0.7,
		EnableHybridSearch:          true,
	}

	client := simplemem.NewWithConfig(config, emb)
	client.SetLLMClient(llmClient)
	client.SetImportanceEstimator(simplemem.NewImportanceEstimatorByLLM(llmClient))

	ctx := context.Background()
	namespace := "terminal_chat_user"

	fmt.Println("============================================")
	fmt.Println("🚀 SimpleMem 终端交互式记忆测试 (LM Studio)")
	fmt.Println("输入 'exit' 或 'quit' 退出对话")
	fmt.Println("============================================")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n👤 你: ")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()

		if strings.ToLower(userInput) == "exit" || strings.ToLower(userInput) == "quit" {
			fmt.Println("👋 再见！")
			break
		}

		if strings.TrimSpace(userInput) == "" {
			continue
		}

		// 3. 使用 Ask 获取带记忆的回答
		startTime := time.Now()
		answer, err := client.Ask(ctx, namespace, userInput)
		if err != nil {
			fmt.Printf("❌ 错误: %v\n", err)
			continue
		}
		duration := time.Since(startTime)

		fmt.Printf("\n🤖 助手: %s\n", answer)
		fmt.Printf("⏱️  耗时: %v\n", duration.Round(time.Millisecond))

		// 4. 将对话存入记忆
		// 将用户的问题和助手回答作为一组对话存入
		err = client.AddDialogue(ctx, namespace, "User", userInput, time.Now())
		if err != nil {
			log.Printf("存储对话失败: %v", err)
		}
		err = client.AddDialogue(ctx, namespace, "Assistant", answer, time.Now())
		if err != nil {
			log.Printf("存储对话失败: %v", err)
		}

		// 手动触发索引重建确保下次能搜到最新结果
		client.RebuildIndex(namespace)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "读取输入数据出错:", err)
	}
}
