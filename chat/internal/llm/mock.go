package llm

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) StreamChat(ctx context.Context, model string, messages []Message) (<-chan Chunk, error) {
	lastUser := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			lastUser = messages[i].Content
			break
		}
	}

	reply := "mock reply"
	if model != "" {
		reply = "mock (" + model + ") reply"
	}
	if lastUser != "" {
		reply += ": " + lastUser
	}

	out := make(chan Chunk, 8)
	go func() {
		defer close(out)
		for _, token := range tokenize(reply) {
			select {
			case <-ctx.Done():
				out <- Chunk{Err: ctx.Err(), Done: true}
				return
			case out <- Chunk{Delta: token}:
			}
			time.Sleep(40 * time.Millisecond)
		}
		out <- Chunk{Done: true}
	}()
	return out, nil
}

func tokenize(text string) []string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return []string{text}
	}
	tokens := make([]string, 0, len(parts))
	for i, part := range parts {
		if i == 0 {
			tokens = append(tokens, part)
			continue
		}
		tokens = append(tokens, " "+part)
	}
	if utf8.RuneCountInString(text) == 0 {
		return []string{""}
	}
	return tokens
}
