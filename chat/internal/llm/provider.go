package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type Chunk struct {
	Delta string
	Done  bool
	Err   error
}

type Provider interface {
	StreamChat(ctx context.Context, model string, messages []Message) (<-chan Chunk, error)
}
