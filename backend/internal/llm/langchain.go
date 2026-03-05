package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

const classifyPrompt = `You are a task classifier. Given a task title and description, classify it and respond ONLY with valid JSON.

Task Title: %s
Task Description: %s

Respond with this exact JSON format:
{
  "tags": ["tag1", "tag2"],
  "priority": "low|medium|high|urgent",
  "category": "bug|feature|improvement|research",
  "summary": "One-line summary of the task"
}

Rules:
- tags: Choose from: backend, frontend, bug, feature, devops, security, performance, documentation, testing. Max 4 tags.
- priority: Based on urgency and impact.
- category: Choose exactly one.
- summary: Max 80 characters.`

// LangChainClient implements LLMClient using langchaingo for multi-provider support.
type LangChainClient struct {
	llm llms.Model
}

// NewLangChainClient creates a client wrapping any langchaingo-compatible model.
func NewLangChainClient(model llms.Model) *LangChainClient {
	return &LangChainClient{llm: model}
}

func (c *LangChainClient) ClassifyTask(ctx context.Context, title, description string) (*TaskClassification, error) {
	prompt := fmt.Sprintf(classifyPrompt, title, description)

	resp, err := llms.GenerateFromSinglePrompt(ctx, c.llm, prompt,
		llms.WithTemperature(0.2),
		llms.WithMaxTokens(256),
	)
	if err != nil {
		return nil, fmt.Errorf("llm generation failed: %w", err)
	}

	classification, err := parseClassification(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse llm response: %w", err)
	}

	return classification, nil
}

func parseClassification(raw string) (*TaskClassification, error) {
	// Extract JSON from response (LLMs sometimes wrap in markdown code blocks)
	cleaned := extractJSON(raw)

	var result TaskClassification
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON in response: %w", err)
	}

	if err := validateClassification(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func extractJSON(s string) string {
	// Strip markdown code fences if present
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+7:]
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+3:]
	}
	if idx := strings.LastIndex(s, "```"); idx != -1 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func validateClassification(c *TaskClassification) error {
	validCategories := map[string]bool{
		"bug": true, "feature": true, "improvement": true, "research": true,
	}
	if !validCategories[c.Category] {
		c.Category = "feature"
	}

	validPriorities := map[string]bool{
		"low": true, "medium": true, "high": true, "urgent": true,
	}
	if !validPriorities[c.Priority] {
		c.Priority = "medium"
	}

	validTags := map[string]bool{
		"backend": true, "frontend": true, "bug": true, "feature": true,
		"devops": true, "security": true, "performance": true,
		"documentation": true, "testing": true,
	}
	filtered := make([]string, 0, len(c.Tags))
	for _, tag := range c.Tags {
		if validTags[tag] && len(filtered) < 4 {
			filtered = append(filtered, tag)
		}
	}
	c.Tags = filtered

	if len(c.Summary) > 80 {
		c.Summary = c.Summary[:77] + "..."
	}

	return nil
}
