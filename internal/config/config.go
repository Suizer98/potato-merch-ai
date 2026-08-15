package config

import "os"

type Config struct {
	GRPCAddr      string
	LLMProvider   string
	OpenAIAPIKey  string
	OpenAIModel   string
	OpenAIBaseURL string
	GroqAPIKey    string
	GroqModel     string
	GroqBaseURL   string
	GeminiAPIKey  string
	GeminiModel   string
	GeminiBaseURL string
}

func Load() Config {
	return Config{
		GRPCAddr:      getenv("GRPC_ADDR", ":50051"),
		LLMProvider:   getenv("LLM_PROVIDER", "mock"),
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:   getenv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL: getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		GroqAPIKey:    os.Getenv("GROQ_API_KEY"),
		GroqModel:     getenv("GROQ_MODEL", "openai/gpt-oss-120b"),
		GroqBaseURL:   getenv("GROQ_BASE_URL", "https://api.groq.com/openai/v1"),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		GeminiModel:   getenv("GEMINI_MODEL", "gemini-3.5-flash"),
		GeminiBaseURL: getenv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta/openai/"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
