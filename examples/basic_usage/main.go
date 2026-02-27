package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"simplemem"
)

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

type MockLLM struct{}

func (l *MockLLM) Chat(ctx context.Context, messages []simplemem.LLMMessage) (string, error) {

	var lastMsg string
	for _, m := range messages {
		if m.Role == "user" {
			lastMsg = m.Content
		}
	}

	if strings.Contains(lastMsg, "scale of 0.0 to 1.0") {
		return "0.85", nil
	}
	if strings.Contains(lastMsg, "Alice") {
		return "Alice and Bob are meeting at Starbucks tomorrow at 2 PM to discuss a report.", nil
	}
	return "I summarized the dialogue fragment for you.", nil
}

func main() {
	ctx := context.Background()

	client := simplemem.New(&SimpleEmbedder{})

	llmClient := &MockLLM{}
	client.SetLLMClient(llmClient)

	// 启用 LLM 重要性评估器
	client.SetImportanceEstimator(simplemem.NewImportanceEstimatorByLLM(llmClient))

	fmt.Println(">>> Adding dialogues...")
	namespace := "demo_user"

	dialogues := []simplemem.Dialogue{
		{
			ID:        "d1",
			Speaker:   "Alice",
			Content:   "Hi Bob, are we still meeting tomorrow at Starbucks?",
			Timestamp: time.Now(),
		},
		{
			ID:        "d2",
			Speaker:   "Bob",
			Content:   "Yes, 2 PM works for me. I will bring the report.",
			Timestamp: time.Now().Add(time.Minute),
		},
	}

	err := client.AddDialogues(ctx, namespace, dialogues)
	if err != nil {
		fmt.Printf("AddDialogues failed: %v\n", err)
		return
	}

	fmt.Println("\n>>> Searching for 'Starbucks'...")
	results, err := client.Search(ctx, namespace, "Starbucks")
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
	} else {
		for _, item := range results {
			fmt.Printf("- Found Item: %s (Importance: %.2f)\n", item.Content, item.Importance)
		}
	}

	fmt.Println("\n>>> Asking: 'What is Alice doing tomorrow?'")
	answer, err := client.Ask(ctx, namespace, "What is Alice doing tomorrow?")
	if err != nil {
		fmt.Printf("Ask failed: %v\n", err)
	} else {
		fmt.Printf("Answer: %s\n", answer)
	}

	fmt.Println("\n>>> Managing memory segments...")
	items, _ := client.Get(namespace)
	fmt.Printf("Total items in namespace: %d\n", len(items))

	if len(items) > 0 {
		targetID := items[0].ID
		fmt.Printf("Deleting item: %s\n", targetID)
		err = client.Delete(namespace, targetID)
		if err == nil {
			fmt.Println("Delete successful.")
		}
	}
}
