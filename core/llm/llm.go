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
	MaxRetries  int // 新增：最大重试次数
}

// WithRetry 是一个包装器，为 LLMClient 增加重试逻辑
type RetryLLMClient struct {
	base       LLMClient
	maxRetries int
}

func NewRetryLLMClient(base LLMClient, maxRetries int) *RetryLLMClient {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &RetryLLMClient{
		base:       base,
		maxRetries: maxRetries,
	}
}

func (c *RetryLLMClient) Chat(ctx context.Context, messages []Message) (string, error) {
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		res, err := c.base.Chat(ctx, messages)
		if err == nil {
			return res, nil
		}
		lastErr = err
		// 这里可以根据错误类型（如 Rate Limit）增加退避重试逻辑（Backoff）
		// 为了简单起见，这里直接重试
	}
	return "", lastErr
}
