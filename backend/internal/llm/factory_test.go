package llm_test

import (
	"testing"

	"github.com/KemenyStudio/task-manager/internal/llm"
)

// TestNewClient cannot use t.Parallel because subtests modify env vars with t.Setenv.
func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		envVars    map[string]string
		wantMock   bool
		wantReason string
	}{
		{
			name:       "defaults to mock when LLM_PROVIDER is unset",
			envVars:    map[string]string{"LLM_PROVIDER": ""},
			wantMock:   true,
			wantReason: "empty LLM_PROVIDER should default to mock",
		},
		{
			name:       "returns mock for explicit mock provider",
			envVars:    map[string]string{"LLM_PROVIDER": "mock"},
			wantMock:   true,
			wantReason: "LLM_PROVIDER=mock should return MockClient",
		},
		{
			name:       "falls back to mock when openai key is missing",
			envVars:    map[string]string{"LLM_PROVIDER": "openai", "OPENAI_API_KEY": ""},
			wantMock:   true,
			wantReason: "openai without API key should fall back to mock",
		},
		{
			name:       "falls back to mock when anthropic key is missing",
			envVars:    map[string]string{"LLM_PROVIDER": "anthropic", "ANTHROPIC_API_KEY": ""},
			wantMock:   true,
			wantReason: "anthropic without API key should fall back to mock",
		},
		{
			name:       "falls back to mock for unknown provider",
			envVars:    map[string]string{"LLM_PROVIDER": "unknown-provider"},
			wantMock:   true,
			wantReason: "unknown provider should fall back to mock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			client := llm.NewClient()
			_, isMock := client.(*llm.MockClient)

			if isMock != tt.wantMock {
				t.Errorf("NewClient() returned MockClient = %v, want %v (%s)", isMock, tt.wantMock, tt.wantReason)
			}
		})
	}
}

func TestLangChainClient_ImplementsLLMClient(t *testing.T) {
	t.Parallel()
	// Compile-time interface compliance check
	var _ llm.LLMClient = (*llm.LangChainClient)(nil)
}
