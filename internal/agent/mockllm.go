package storeagent

import (
	"context"
	"iter"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type MockLLM struct{}

func NewMockLLM() *MockLLM {
	return &MockLLM{}
}

func (m *MockLLM) Name() string {
	return "mock"
}

func (m *MockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		user := lastUserText(req)
		reply := "Potato concierge (mock) here. You said: " + user
		switch classifyRoute(user) {
		case ShopAgentName:
			reply = "Shop agent (mock): Couch Potato Tee is in stock in S–XXL, on sale at 25.20 SGD."
		case BillingAgentName:
			reply = "Billing agent (mock): Share an order number like ORD-1042 and I can look it up in CRM."
		case SupportAgentName:
			reply = "Support agent (mock): I can open a ticket in Twenty. What went wrong with the order?"
		}

		if stream {
			for _, token := range strings.Fields(reply) {
				select {
				case <-ctx.Done():
					yield(&model.LLMResponse{ErrorMessage: ctx.Err().Error(), TurnComplete: true}, ctx.Err())
					return
				default:
				}
				if !yield(&model.LLMResponse{
					Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: token + " "}}},
					Partial: true,
				}, nil) {
					return
				}
				time.Sleep(25 * time.Millisecond)
			}
		}

		yield(&model.LLMResponse{
			Content:      &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: reply}}},
			TurnComplete: true,
			FinishReason: genai.FinishReasonStop,
		}, nil)
	}
}

func lastUserText(req *model.LLMRequest) string {
	if req == nil {
		return ""
	}
	for i := len(req.Contents) - 1; i >= 0; i-- {
		c := req.Contents[i]
		if c == nil || c.Role != genai.RoleUser {
			continue
		}
		var b strings.Builder
		for _, p := range c.Parts {
			if p != nil {
				b.WriteString(p.Text)
			}
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}
