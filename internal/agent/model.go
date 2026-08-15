package storeagent

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/genai"

	"github.com/Suizer98/protobuf-ai-potato/internal/config"
)

func NewLLM(ctx context.Context, cfg config.Config) (model.LLM, error) {
	switch cfg.LLMProvider {
	case "mock":
		return NewMockLLM(), nil
	case "gemini":
		return newGeminiLLM(ctx, cfg)
	case "groq":
		return newGroqLLM(ctx, cfg)
	case "openai":
		return NewChatCompletionsLLM(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel)
	default:
		return nil, fmt.Errorf("unsupported LLM_PROVIDER %q (use mock, openai, groq, or gemini)", cfg.LLMProvider)
	}
}

func newGeminiLLM(ctx context.Context, cfg config.Config) (model.LLM, error) {
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}
	return gemini.NewModel(ctx, cfg.GeminiModel, &genai.ClientConfig{APIKey: cfg.GeminiAPIKey})
}

func newGroqLLM(ctx context.Context, cfg config.Config) (model.LLM, error) {
	primary, err := NewChatCompletionsLLM(cfg.GroqAPIKey, cfg.GroqBaseURL, cfg.GroqModel)
	if cfg.GeminiAPIKey == "" {
		return primary, err
	}
	fallback, fallbackErr := newGeminiLLM(ctx, cfg)
	if fallbackErr != nil {
		log.Printf("gemini fallback unavailable: %v", fallbackErr)
		return primary, err
	}
	if err != nil {
		log.Printf("groq unavailable (%v); using gemini", err)
		return fallback, nil
	}
	log.Printf("llm groq with gemini fallback")
	return NewFailoverLLM(primary, fallback, "groq", "gemini"), nil
}
