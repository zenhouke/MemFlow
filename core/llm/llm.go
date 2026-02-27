package llm

import "context"

type Message struct {
	Role    string
	Content string
}

type LLMClient interface {
	Chat(ctx context.Context, messages []Message) (string, error)
}

type LLMConfig struct {
	Model       string
	Temperature float64
	MaxTokens   int
}
