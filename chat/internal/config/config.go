package config

import (
	"os"
	"strings"
)

const defaultTwentyAPIKeyFile = "/state/twenty_api_key"

type Config struct {
	GRPCAddr      string
	HTTPAddr      string
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
	TwentyMCPURL  string
	TwentyAPIKey  string
	CRMURL        string
	CRMOrigin     string
	AdminEmail    string
	AdminPassword string
}

func Load() Config {
	return Config{
		GRPCAddr:      getenv("GRPC_ADDR", ":50051"),
		HTTPAddr:      getenv("CHAT_HTTP_ADDR", ":8081"),
		LLMProvider:   getenv("LLM_PROVIDER", "mock"),
		OpenAIAPIKey:  env("OPENAI_API_KEY"),
		OpenAIModel:   getenv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL: env("OPENAI_BASE_URL"),
		GroqAPIKey:    env("GROQ_API_KEY"),
		GroqModel:     getenv("GROQ_MODEL", "openai/gpt-oss-120b"),
		GroqBaseURL:   env("GROQ_BASE_URL"),
		GeminiAPIKey:  env("GEMINI_API_KEY"),
		GeminiModel:   getenv("GEMINI_MODEL", "gemini-3.5-flash"),
		GeminiBaseURL: env("GEMINI_BASE_URL"),
		TwentyMCPURL:  env("TWENTY_MCP_URL"),
		TwentyAPIKey:  twentyAPIKey(),
		CRMURL:        env("CRM_URL"),
		CRMOrigin:     env("SERVER_URL"),
		AdminEmail:    env("ADMIN_EMAIL"),
		AdminPassword: env("ADMIN_PASSWORD"),
	}
}

func env(key string) string {
	return strings.TrimSpace(strings.Trim(os.Getenv(key), "\r"))
}

func getenv(key, fallback string) string {
	if value := env(key); value != "" {
		return value
	}
	return fallback
}

func twentyAPIKey() string {
	path := getenv("TWENTY_API_KEY_FILE", defaultTwentyAPIKeyFile)
	if value := readTrimmedFile(path); value != "" {
		return value
	}
	return env("TWENTY_API_KEY")
}

func readTrimmedFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(string(data), "\r"))
}
