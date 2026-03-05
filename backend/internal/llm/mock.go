package llm

import (
	"context"
	"strings"
)

// MockClient provides predictable responses for testing without an API key.
// It classifies tasks based on simple keyword matching in the title.
type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) ClassifyTask(ctx context.Context, title, description string) (*TaskClassification, error) {
	combined := strings.ToLower(title + " " + description)

	classification := &TaskClassification{
		Tags:     []string{},
		Priority: "medium",
		Category: "feature",
		Summary:  "Task: " + title,
	}

	// Determine category based on keywords.
	// "improvement" is checked before "bug" because titles like
	// "Refactor error handling" express intent to improve, not report a bug.
	lowerTitle := strings.ToLower(title)
	switch {
	case containsAny(lowerTitle, "refactor", "clean", "improve", "optimize", "migrate"):
		classification.Category = "improvement"
		classification.Tags = append(classification.Tags, "improvement")
	case containsAny(combined, "bug", "fix", "error", "crash", "broken", "fail"):
		classification.Category = "bug"
		classification.Tags = append(classification.Tags, "bug")
	case containsAny(combined, "research", "investigate", "explore", "spike", "poc"):
		classification.Category = "research"
		classification.Tags = append(classification.Tags, "research")
	default:
		classification.Tags = append(classification.Tags, "feature")
	}

	// Determine priority based on keywords
	switch {
	case containsAny(combined, "urgent", "critical", "security", "vulnerability", "crash"):
		classification.Priority = "high"
	case containsAny(combined, "nice to have", "low priority", "when possible", "eventually"):
		classification.Priority = "low"
	}

	// Add topic tags
	if containsAny(combined, "api", "endpoint", "backend", "server", "database", "query") {
		classification.Tags = append(classification.Tags, "backend")
	}
	if containsAny(combined, "ui", "frontend", "component", "css", "style", "layout", "page") {
		classification.Tags = append(classification.Tags, "frontend")
	}
	if containsAny(combined, "docker", "deploy", "ci", "cd", "pipeline", "infra") {
		classification.Tags = append(classification.Tags, "devops")
	}
	if containsAny(combined, "auth", "security", "permission", "token", "jwt", "oauth") {
		classification.Tags = append(classification.Tags, "security")
	}
	if containsAny(combined, "test", "testing", "coverage", "spec") {
		classification.Tags = append(classification.Tags, "testing")
	}
	if containsAny(combined, "performance", "slow", "optimize", "cache", "speed") {
		classification.Tags = append(classification.Tags, "performance")
	}

	// Generate a better summary
	if len(classification.Summary) > 80 {
		classification.Summary = classification.Summary[:77] + "..."
	}

	return classification, nil
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
