package llm

import (
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// NewClient creates an LLMClient based on the LLM_PROVIDER environment variable.
// Supported providers: "openai", "ollama", "mock" (default).
func NewClient() LLMClient {
	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "mock"
	}

	switch provider {
	case "openai":
		client, err := newOpenAIClient()
		if err != nil {
			log.Printf("Failed to create OpenAI client: %v — falling back to mock", err)
			return NewMockClient()
		}
		log.Printf("LLM provider: openai (model: %s)", getEnvOrDefault("LLM_MODEL", "gpt-4o-mini"))
		return client

	case "ollama":
		client, err := newOllamaClient()
		if err != nil {
			log.Printf("Failed to create Ollama client: %v — falling back to mock", err)
			return NewMockClient()
		}
		log.Printf("LLM provider: ollama (model: %s)", getEnvOrDefault("LLM_MODEL", "llama3.2"))
		return client

	default:
		log.Printf("LLM provider: mock")
		return NewMockClient()
	}
}

func newOpenAIClient() (*LangChainClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
	}

	model := getEnvOrDefault("LLM_MODEL", "gpt-4o-mini")

	llm, err := openai.New(openai.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("openai.New: %w", err)
	}

	return NewLangChainClient(llm), nil
}

func newOllamaClient() (*LangChainClient, error) {
	model := getEnvOrDefault("LLM_MODEL", "llama3.2")
	serverURL := getEnvOrDefault("OLLAMA_SERVER_URL", "http://localhost:11434")

	llm, err := ollama.New(
		ollama.WithModel(model),
		ollama.WithServerURL(serverURL),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama.New: %w", err)
	}

	return NewLangChainClient(llm), nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
