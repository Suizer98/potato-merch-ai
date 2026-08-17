package storeagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func classifyRouteWithLLM(ctx context.Context, llm model.LLM, text, currentRoute string) (string, error) {
	if llm == nil {
		return "", fmt.Errorf("router model is unavailable")
	}
	if llm.Name() == "mock" {
		return classifyRoute(text), nil
	}

	prompt := fmt.Sprintf("%s\n\nCurrent agent: %s\nUser message: %s", routerInstruction, currentRoute, text)
	request := &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText(prompt, genai.RoleUser)},
	}

	routerCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var answer strings.Builder
	for response, err := range llm.GenerateContent(routerCtx, request, false) {
		if err != nil {
			return "", err
		}
		if response == nil {
			continue
		}
		if response.ErrorMessage != "" {
			return "", fmt.Errorf("%s", response.ErrorMessage)
		}
		if response.Content == nil {
			continue
		}
		for _, part := range response.Content.Parts {
			if part != nil {
				answer.WriteString(part.Text)
			}
		}
	}

	route := strings.ToLower(strings.Trim(strings.TrimSpace(answer.String()), "`\"'.: "))
	if knownAgent(route) {
		return route, nil
	}
	return "", fmt.Errorf("invalid router response %q", answer.String())
}
