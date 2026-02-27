package engine

import (
	"context"
	"fmt"
	"simplemem/core/llm"
	"strconv"
	"strings"
)

type ImportanceEstimatorByLLM struct {
	client llm.LLMClient
}

func NewImportanceEstimatorByLLM(client llm.LLMClient) *ImportanceEstimatorByLLM {
	return &ImportanceEstimatorByLLM{
		client: client,
	}
}

func (e *ImportanceEstimatorByLLM) Estimate(ctx context.Context, content string) (float64, error) {
	if e.client == nil {
		return 0.0, fmt.Errorf("LLM client is not set")
	}

	prompt := fmt.Sprintf(`On a scale of 0.0 to 1.0, how important is it to remember the following information for a long-term conversation? 
Higher score means the information is critical (e.g., identity, secret, task, significant event). 
Lower score means it is trivial or temporary (e.g., greetings, small talk, transient state).

Information: "%s"

Output ONLY the numerical score (e.g., 0.85).`, content)

	messages := []llm.Message{
		{Role: "system", Content: "You are a logical memory management assistant. You only output numbers."},
		{Role: "user", Content: prompt},
	}

	resp, err := e.client.Chat(ctx, messages)
	if err != nil {
		return 0.0, err
	}

	cleanResp := strings.TrimSpace(resp)
	score, err := strconv.ParseFloat(cleanResp, 64)
	if err != nil {

		return 0.0, fmt.Errorf("failed to parse importance score: %s", resp)
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score, nil
}
