package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	defaultModel string
	client     *http.Client
}

func NewOpenAIProvider(apiKey, baseURL, defaultModel string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:       apiKey,
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: defaultModel,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, model string, messages []Message) (<-chan Chunk, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	if model == "" {
		model = p.defaultModel
	}

	payload := openAIRequest{
		Model:    model,
		Stream:   true,
		Messages: make([]openAIMessage, 0, len(messages)),
	}
	for _, message := range messages {
		payload.Messages = append(payload.Messages, openAIMessage{
			Role:    string(message.Role),
			Content: message.Content,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(raw))
	}

	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				out <- Chunk{Done: true}
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				select {
				case <-ctx.Done():
					out <- Chunk{Err: ctx.Err(), Done: true}
					return
				case out <- Chunk{Delta: delta}:
				}
			}
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != "" {
				out <- Chunk{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Chunk{Err: err, Done: true}
			return
		}
		out <- Chunk{Done: true}
	}()

	return out, nil
}
