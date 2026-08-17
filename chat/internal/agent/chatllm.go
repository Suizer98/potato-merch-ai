package storeagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type ChatCompletionsLLM struct {
	apiKey string
	baseURL string
	name   string
	client *http.Client
}

func NewChatCompletionsLLM(apiKey, baseURL, modelName string) (*ChatCompletionsLLM, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	if modelName == "" {
		return nil, fmt.Errorf("model name is required")
	}
	return &ChatCompletionsLLM{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		name:    modelName,
		client:  &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (m *ChatCompletionsLLM) Name() string {
	return m.name
}

func (m *ChatCompletionsLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		payload, err := buildChatPayload(m.name, req, stream)
		if err != nil {
			yield(nil, err)
			return
		}
		body, err := json.Marshal(payload)
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		if stream {
			httpReq.Header.Set("Accept", "text/event-stream")
		}

		resp, err := m.client.Do(httpReq)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			yield(nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(raw)))
			return
		}
		if stream {
			streamChatCompletions(ctx, resp.Body, yield)
			return
		}

		var out chatCompletion
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			yield(nil, err)
			return
		}
		llmResp, err := chatCompletionToLLM(out)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(llmResp, nil)
	}
}

type chatPayload struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Tools    []chatTool    `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall   `json:"tool_calls,omitempty"`
}

type chatTool struct {
	Type     string         `json:"type"`
	Function chatToolSchema `json:"function"`
}

type chatToolSchema struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatCompletion struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolDelta `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

type chatToolDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func buildChatPayload(modelName string, req *model.LLMRequest, stream bool) (chatPayload, error) {
	payload := chatPayload{
		Model:    modelName,
		Stream:   stream,
		Messages: make([]chatMessage, 0, 8),
	}
	if req == nil {
		return payload, fmt.Errorf("llm request is required")
	}
	if req.Model != "" {
		payload.Model = req.Model
	}
	if req.Config != nil {
		if system := contentText(req.Config.SystemInstruction); system != "" {
			payload.Messages = append(payload.Messages, chatMessage{Role: "system", Content: system})
		}
	}
	for _, content := range req.Contents {
		if content == nil {
			continue
		}
		msgs := contentToChatMessages(content)
		payload.Messages = append(payload.Messages, msgs...)
	}
	if req.Config != nil {
		for _, tool := range req.Config.Tools {
			if tool == nil {
				continue
			}
			for _, decl := range tool.FunctionDeclarations {
				if decl == nil || decl.Name == "" {
					continue
				}
				payload.Tools = append(payload.Tools, chatTool{
					Type: "function",
					Function: chatToolSchema{
						Name:        decl.Name,
						Description: decl.Description,
						Parameters:  toolParameters(decl),
					},
				})
			}
		}
	}
	return payload, nil
}

func contentText(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range content.Parts {
		if part != nil {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func toolParameters(decl *genai.FunctionDeclaration) map[string]any {
	if decl.ParametersJsonSchema != nil {
		if mapped := jsonSchemaMap(decl.ParametersJsonSchema); mapped != nil {
			return strictObjectSchema(mapped)
		}
	}
	if decl.Parameters != nil {
		return strictObjectSchema(genaiSchemaMap(decl.Parameters))
	}
	return defaultRequestSchema()
}

func defaultRequestSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"request": map[string]any{
				"type":        "string",
				"description": "Task for this specialist, including any order number or email the shopper already gave.",
			},
		},
		"required":             []string{"request"},
		"additionalProperties": false,
	}
}

func jsonSchemaMap(raw any) map[string]any {
	switch value := raw.(type) {
	case map[string]any:
		return value
	case json.RawMessage:
		var out map[string]any
		if json.Unmarshal(value, &out) == nil {
			return out
		}
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var out map[string]any
		if json.Unmarshal(encoded, &out) == nil {
			return out
		}
	}
	return nil
}

func genaiSchemaMap(schema *genai.Schema) map[string]any {
	if schema == nil {
		return defaultRequestSchema()
	}
	out := map[string]any{}
	kind := strings.ToLower(string(schema.Type))
	if kind == "" {
		kind = "object"
	}
	out["type"] = kind
	if schema.Description != "" {
		out["description"] = schema.Description
	}
	if len(schema.Properties) > 0 {
		props := make(map[string]any, len(schema.Properties))
		for key, child := range schema.Properties {
			props[key] = genaiSchemaMap(child)
		}
		out["properties"] = props
	} else if kind == "object" {
		out["properties"] = map[string]any{}
	}
	if len(schema.Required) > 0 {
		out["required"] = schema.Required
	}
	if schema.Items != nil {
		out["items"] = genaiSchemaMap(schema.Items)
	}
	return out
}

func strictObjectSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return defaultRequestSchema()
	}
	kind, _ := schema["type"].(string)
	if strings.EqualFold(kind, "object") || kind == "" {
		schema["type"] = "object"
		if _, ok := schema["properties"]; !ok {
			schema["properties"] = map[string]any{}
		}
		schema["additionalProperties"] = false
		if props, ok := schema["properties"].(map[string]any); ok && len(props) == 0 {
			return defaultRequestSchema()
		}
	}
	return schema
}

func contentToChatMessages(content *genai.Content) []chatMessage {
	role := "user"
	switch content.Role {
	case genai.RoleModel:
		role = "assistant"
	case "tool":
		role = "tool"
	}

	var text strings.Builder
	var calls []chatToolCall
	var msgs []chatMessage
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			calls = append(calls, chatToolCall{
				ID:   part.FunctionCall.ID,
				Type: "function",
			})
			calls[len(calls)-1].Function.Name = part.FunctionCall.Name
			calls[len(calls)-1].Function.Arguments = string(args)
		}
		if part.FunctionResponse != nil {
			raw, _ := json.Marshal(part.FunctionResponse.Response)
			msgs = append(msgs, chatMessage{
				Role:       "tool",
				ToolCallID: part.FunctionResponse.ID,
				Content:    string(raw),
			})
		}
	}
	if text.Len() > 0 || len(calls) > 0 {
		msgs = append([]chatMessage{{
			Role:      role,
			Content:   text.String(),
			ToolCalls: calls,
		}}, msgs...)
	}
	return msgs
}

func streamChatCompletions(ctx context.Context, body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	assembled := map[int]*chatToolCall{}
	var text strings.Builder

	flushCalls := func() bool {
		if len(assembled) == 0 {
			return true
		}
		parts := make([]*genai.Part, 0, len(assembled))
		for i := 0; i < len(assembled); i++ {
			call, ok := assembled[i]
			if !ok || call == nil {
				continue
			}
			var args map[string]any
			if call.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
			}
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   call.ID,
					Name: call.Function.Name,
					Args: args,
				},
			})
		}
		if len(parts) == 0 {
			return true
		}
		return yield(&model.LLMResponse{
			Content:      &genai.Content{Role: genai.RoleModel, Parts: parts},
			TurnComplete: true,
			FinishReason: genai.FinishReasonStop,
		}, nil)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			yield(&model.LLMResponse{ErrorMessage: ctx.Err().Error(), TurnComplete: true}, ctx.Err())
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			if !flushCalls() {
				return
			}
			if text.Len() > 0 {
				yield(&model.LLMResponse{
					Content:      &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: text.String()}}},
					TurnComplete: true,
					FinishReason: genai.FinishReasonStop,
				}, nil)
			}
			return
		}
		var chunk chatCompletion
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if delta := choice.Delta.Content; delta != "" {
			text.WriteString(delta)
			if !yield(&model.LLMResponse{
				Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: delta}}},
				Partial: true,
			}, nil) {
				return
			}
		}
		for _, call := range choice.Delta.ToolCalls {
			existing, ok := assembled[call.Index]
			if !ok {
				existing = &chatToolCall{ID: call.ID, Type: "function"}
				assembled[call.Index] = existing
			}
			if call.ID != "" {
				existing.ID = call.ID
			}
			if call.Function.Name != "" {
				existing.Function.Name = call.Function.Name
			}
			existing.Function.Arguments += call.Function.Arguments
		}
		if choice.FinishReason == "tool_calls" {
			if !flushCalls() {
				return
			}
			return
		}
		if choice.FinishReason != "" {
			if !flushCalls() {
				return
			}
			yield(&model.LLMResponse{
				Content:      &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: text.String()}}},
				TurnComplete: true,
				FinishReason: genai.FinishReasonStop,
			}, nil)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		yield(nil, err)
	}
}

func chatCompletionToLLM(out chatCompletion) (*model.LLMResponse, error) {
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("empty chat completion")
	}
	msg := out.Choices[0].Message
	if len(msg.ToolCalls) > 0 {
		parts := make([]*genai.Part, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			var args map[string]any
			if call.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
			}
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   call.ID,
					Name: call.Function.Name,
					Args: args,
				},
			})
		}
		return &model.LLMResponse{
			Content:      &genai.Content{Role: genai.RoleModel, Parts: parts},
			TurnComplete: true,
			FinishReason: genai.FinishReasonStop,
		}, nil
	}
	return &model.LLMResponse{
		Content:      &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{Text: msg.Content}}},
		TurnComplete: true,
		FinishReason: genai.FinishReasonStop,
	}, nil
}
