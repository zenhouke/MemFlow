package summarizer

import (
	"context"
	"fmt"
	"memflow/core/llm"
	"strings"
)

type Summarizer interface {
	Summarize(ctx context.Context, inputs []string) (string, error)
}

type LLMSummarizer struct {
	client llm.LLMClient
	config llm.LLMConfig
}

func NewLLMSummarizer(client llm.LLMClient, config llm.LLMConfig) *LLMSummarizer {

	if config.Temperature == 0 {
		config.Temperature = 0.3
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 500
	}

	return &LLMSummarizer{
		client: client,
		config: config,
	}
}

func (s *LLMSummarizer) Summarize(ctx context.Context, inputs []string) (string, error) {
	if len(inputs) == 0 {
		return "", fmt.Errorf("no inputs to summarize")
	}

	if len(inputs) == 1 {
		return inputs[0], nil
	}

	// 快速路径：如果输入条目很少且总字符数很短，不值得调用 LLM
	totalLen := 0
	for _, in := range inputs {
		totalLen += len(in)
	}
	if len(inputs) <= 2 && totalLen < 100 {
		return s.fallbackSummarize(inputs), nil
	}

	messages := s.buildSynthesisMessages(inputs)

	summary, err := s.client.Chat(ctx, messages)
	if err != nil {

		return s.fallbackSummarize(inputs), nil
	}

	return strings.TrimSpace(summary), nil
}

func (s *LLMSummarizer) buildSynthesisMessages(inputs []string) []llm.Message {
	var userPrompt strings.Builder

	userPrompt.WriteString("MEMORY FRAGMENTS:\n")
	for i, input := range inputs {
		userPrompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, input))
	}

	userPrompt.WriteString("\nPlease consolidate these memory fragments into a single, coherent statement. ")
	userPrompt.WriteString("Preserve all important details while eliminating redundancy.")

	return []llm.Message{
		{
			Role: "system",
			Content: "You are a memory consolidation system. Your task is to synthesize multiple related memory fragments into a single, coherent abstract representation.\n\n" +
				"INSTRUCTIONS:\n" +
				"1. Identify common themes and patterns across the memory fragments\n" +
				"2. Consolidate repetitive or redundant information\n" +
				"3. Preserve all important factual details\n" +
				"4. Create a concise, self-contained summary\n" +
				"5. Maintain temporal consistency if timestamps are present\n" +
				"6. Output ONLY the consolidated text, no explanations",
		},
		{
			Role:    "user",
			Content: userPrompt.String(),
		},
	}
}

func (s *LLMSummarizer) fallbackSummarize(inputs []string) string {

	seen := make(map[string]bool)
	var unique []string

	for _, input := range inputs {
		if !seen[input] {
			unique = append(unique, input)
			seen[input] = true
		}
	}

	if len(unique) == 1 {
		return unique[0]
	}

	return strings.Join(unique, "; ")
}

type SimpleSummarizer struct{}

func NewSimpleSummarizer() *SimpleSummarizer {
	return &SimpleSummarizer{}
}

func (s *SimpleSummarizer) Summarize(ctx context.Context, inputs []string) (string, error) {
	if len(inputs) == 0 {
		return "", fmt.Errorf("no inputs to summarize")
	}

	if len(inputs) == 1 {
		return inputs[0], nil
	}

	summary := "Summary: "
	for i, text := range inputs {
		if i > 0 {
			summary += "; "
		}
		summary += text
	}

	return summary, nil
}
