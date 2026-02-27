package compression

import (
	"context"
	"fmt"
	"simplemem/core/config"
	"simplemem/core/llm"
	"strings"
	"time"
)

type Dialogue struct {
	ID        string
	Speaker   string
	Content   string
	Timestamp time.Time
}

type MemoryUnit struct {
	ID                  string
	Content             string
	OriginalContent     string
	Keywords            []string
	Timestamp           *time.Time
	Location            string
	Persons             []string
	Entities            []string
	Topic               string
	Salience            string
	SourceDialogueIDs   []string
	SourceDialogueCount int
}

type SemanticCompressor struct {
	config    config.CompressionConfig
	llmClient llm.LLMClient

	previousUnits []MemoryUnit
}

func NewSemanticCompressor(cfg config.CompressionConfig, llmClient llm.LLMClient) *SemanticCompressor {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 10
	}
	if cfg.OverlapSize == 0 {
		cfg.OverlapSize = 2
	}

	return &SemanticCompressor{
		config:    cfg,
		llmClient: llmClient,
	}
}

func (c *SemanticCompressor) ProcessDialogues(ctx context.Context, dialogues []Dialogue) ([]MemoryUnit, error) {
	if len(dialogues) == 0 {
		return nil, nil
	}

	var units []MemoryUnit
	stepSize := c.config.WindowSize - c.config.OverlapSize
	if stepSize < 1 {
		stepSize = 1
	}

	for i := 0; i < len(dialogues); i += stepSize {
		end := i + c.config.WindowSize
		if end > len(dialogues) {
			end = len(dialogues)
		}

		window := dialogues[i:end]
		if len(window) < 2 {
			continue
		}

		unit, err := c.extractUnit(ctx, window, i)
		if err != nil {
			continue
		}

		if unit != nil {
			units = append(units, *unit)
			c.previousUnits = append(c.previousUnits, *unit)
			if len(c.previousUnits) > 10 {
				c.previousUnits = c.previousUnits[len(c.previousUnits)-10:]
			}
		}
	}

	return units, nil
}

func (c *SemanticCompressor) extractUnit(ctx context.Context, window []Dialogue, windowIndex int) (*MemoryUnit, error) {
	if c.llmClient == nil {
		return c.simpleExtract(window), nil
	}

	return c.llmExtract(ctx, window, windowIndex)
}

func (c *SemanticCompressor) simpleExtract(window []Dialogue) *MemoryUnit {
	var content strings.Builder
	var original strings.Builder
	var persons []string
	var timestamp *time.Time

	for _, d := range window {
		original.WriteString(fmt.Sprintf("[%s] %s: %s\n", d.Timestamp.Format("2006-01-02T15:04:05"), d.Speaker, d.Content))
		content.WriteString(fmt.Sprintf("%s: %s; ", d.Speaker, d.Content))

		persons = append(persons, d.Speaker)

		if timestamp == nil || d.Timestamp.After(*timestamp) {
			timestamp = &d.Timestamp
		}
	}

	return &MemoryUnit{
		ID:                  fmt.Sprintf("unit-%d", time.Now().UnixNano()),
		Content:             content.String(),
		OriginalContent:     original.String(),
		Timestamp:           timestamp,
		Persons:             uniqueStrings(persons),
		Salience:            "medium",
		SourceDialogueCount: len(window),
	}
}

func (c *SemanticCompressor) llmExtract(ctx context.Context, window []Dialogue, windowIndex int) (*MemoryUnit, error) {
	var dialogueText strings.Builder
	var baseTimestamp time.Time

	for _, d := range window {
		dialogueText.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			d.Timestamp.Format("2006-01-02T15:04:05"),
			d.Speaker,
			d.Content))
		if baseTimestamp.IsZero() {
			baseTimestamp = d.Timestamp
		}
	}

	var contextText string
	if len(c.previousUnits) > 0 {
		contextText = "\n[Previous Window Memory Entries]\n"
		for _, u := range c.previousUnits[len(c.previousUnits)-3:] {
			contextText += fmt.Sprintf("- %s\n", u.Content)
		}
	}

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a memory extraction system. Your task is to convert dialogue into structured, atomic memory units.

REQUIREMENTS:
1. Complete Coverage: Generate enough memory entries to ensure ALL information is captured
2. Force Disambiguation: PROHIBIT pronouns (he, she, it, they) and relative time (yesterday, today, last week, tomorrow)
3. Lossless Information: Each entry must be complete and independent
4. Avoid Duplication: Reference previous window entries to avoid extracting redundant information

OUTPUT FORMAT (JSON):
{
  "content": "Complete unambiguous restatement",
  "keywords": ["keyword1", "keyword2"],
  "timestamp": "YYYY-MM-DDTHH:MM:SS or null",
  "location": "location or null",
  "persons": ["name1", "name2"],
  "entities": ["entity1", "entity2"],
  "topic": "topic phrase",
  "salience": "high|medium|low"
}`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Base timestamp: %s\n\n%s\n\nExtract the memory unit:", baseTimestamp.Format(time.RFC3339), contextText),
		},
	}

	response, err := c.llmClient.Chat(ctx, messages)
	if err != nil {
		return c.simpleExtract(window), nil
	}

	return c.parseResponse(response, window, windowIndex)
}

func (c *SemanticCompressor) parseResponse(response string, window []Dialogue, windowIndex int) (*MemoryUnit, error) {
	var content, timestamp, location, topic, salience string
	var keywords, persons, entities []string

	lines := strings.Split(response, "\n")
	inArray := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "[") || inArray {
			if strings.HasPrefix(line, "[") && !inArray {
				inArray = true
			}
			if strings.HasPrefix(line, "}") {
				inArray = false
				continue
			}

			if strings.Contains(line, `"content"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					content = strings.Trim(parts[1], `",`)
				}
			}
			if strings.Contains(line, `"keywords"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					kw := strings.Trim(parts[1], `[]",`)
					if kw != "" {
						keywords = append(keywords, kw)
					}
				}
			}
			if strings.Contains(line, `"timestamp"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					ts := strings.Trim(parts[1], `",`)
					if ts != "" && ts != "null" {
						timestamp = ts
					}
				}
			}
			if strings.Contains(line, `"location"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					loc := strings.Trim(parts[1], `",`)
					if loc != "" && loc != "null" {
						location = loc
					}
				}
			}
			if strings.Contains(line, `"persons"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					p := strings.Trim(parts[1], `[]",`)
					if p != "" {
						persons = append(persons, p)
					}
				}
			}
			if strings.Contains(line, `"entities"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					e := strings.Trim(parts[1], `[]",`)
					if e != "" {
						entities = append(entities, e)
					}
				}
			}
			if strings.Contains(line, `"topic"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					topic = strings.Trim(parts[1], `",`)
				}
			}
			if strings.Contains(line, `"salience"`) {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					salience = strings.Trim(parts[1], `",`)
				}
			}
		}
	}

	var parsedTime *time.Time
	if timestamp != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", timestamp); err == nil {
			parsedTime = &t
		}
	}

	var dialogueIDs []string
	for _, d := range window {
		dialogueIDs = append(dialogueIDs, d.ID)
	}

	if content == "" {
		return c.simpleExtract(window), nil
	}

	return &MemoryUnit{
		ID:                  fmt.Sprintf("unit-%d-%d", windowIndex, time.Now().UnixNano()),
		Content:             content,
		OriginalContent:     extractOriginal(window),
		Keywords:            keywords,
		Timestamp:           parsedTime,
		Location:            location,
		Persons:             uniqueStrings(append(persons, extractSpeakers(window)...)),
		Entities:            uniqueStrings(entities),
		Topic:               topic,
		Salience:            salience,
		SourceDialogueIDs:   dialogueIDs,
		SourceDialogueCount: len(window),
	}, nil
}

func extractOriginal(window []Dialogue) string {
	var b strings.Builder
	for _, d := range window {
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", d.Timestamp.Format("2006-01-02T15:04:05"), d.Speaker, d.Content))
	}
	return b.String()
}

func extractSpeakers(window []Dialogue) []string {
	var speakers []string
	for _, d := range window {
		speakers = append(speakers, d.Speaker)
	}
	return speakers
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
