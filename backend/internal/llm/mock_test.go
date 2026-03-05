package llm_test

import (
	"context"
	"testing"

	"github.com/KemenyStudio/task-manager/internal/llm"
)

func TestMockClient_ClassifyTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		title        string
		description  string
		wantCategory string
		wantPriority string
		wantTag      string
	}{
		{
			name:         "bug with crash keyword gets high priority",
			title:        "Fix: Login page crashes on invalid email",
			description:  "When a user enters an email without @ symbol, the login page throws an unhandled exception and shows a white screen.",
			wantCategory: "bug",
			wantPriority: "high",
			wantTag:      "bug",
		},
		{
			name:         "feature with auth keywords gets security tag",
			title:        "Implement OAuth with Google",
			description:  "Add Google OAuth as an alternative authentication method. Support redirect flow, callback, and user creation.",
			wantCategory: "feature",
			wantPriority: "medium",
			wantTag:      "security",
		},
		{
			name:         "research with API keywords gets backend tag",
			title:        "Investigate rate limiting options for the API",
			description:  "Research token bucket vs sliding window, implementation options, Redis vs in-memory.",
			wantCategory: "research",
			wantPriority: "medium",
			wantTag:      "backend",
		},
		{
			name:         "refactor keyword classifies as improvement",
			title:        "Refactor error handling in backend",
			description:  "Clean up inconsistent error responses. Create centralized error handler with proper types.",
			wantCategory: "improvement",
			wantPriority: "medium",
			wantTag:      "improvement",
		},
		{
			name:         "CI/CD keywords get devops tag",
			title:        "Configure CI/CD pipeline with GitHub Actions",
			description:  "Set up pipeline for tests, Docker build, and deploy to staging.",
			wantCategory: "feature",
			wantPriority: "medium",
			wantTag:      "devops",
		},
	}

	client := llm.NewMockClient()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := client.ClassifyTask(context.Background(), tt.title, tt.description)
			if err != nil {
				t.Fatalf("ClassifyTask() error = %v, want nil", err)
			}

			if result.Category != tt.wantCategory {
				t.Errorf("ClassifyTask().Category = %q, want %q", result.Category, tt.wantCategory)
			}

			if result.Priority != tt.wantPriority {
				t.Errorf("ClassifyTask().Priority = %q, want %q", result.Priority, tt.wantPriority)
			}

			if !containsTag(result.Tags, tt.wantTag) {
				t.Errorf("ClassifyTask().Tags = %v, want to contain %q", result.Tags, tt.wantTag)
			}
		})
	}
}

func TestMockClient_SummaryTruncation(t *testing.T) {
	t.Parallel()

	client := llm.NewMockClient()
	longTitle := "This is a very long task title that exceeds the eighty character limit and should be truncated by the mock client"

	result, err := client.ClassifyTask(context.Background(), longTitle, "")
	if err != nil {
		t.Fatalf("ClassifyTask() error = %v, want nil", err)
	}

	if len(result.Summary) > 80 {
		t.Errorf("ClassifyTask().Summary length = %d, want <= 80", len(result.Summary))
	}
}

func TestMockClient_ImplementsLLMClient(t *testing.T) {
	t.Parallel()
	// Compile-time interface compliance check
	var _ llm.LLMClient = (*llm.MockClient)(nil)
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
