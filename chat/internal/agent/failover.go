package storeagent

import (
	"context"
	"iter"
	"log"

	"google.golang.org/adk/v2/model"
)

type FailoverLLM struct {
	primary      model.LLM
	fallback     model.LLM
	primaryName  string
	fallbackName string
}

func NewFailoverLLM(primary, fallback model.LLM, primaryName, fallbackName string) *FailoverLLM {
	return &FailoverLLM{
		primary:      primary,
		fallback:     fallback,
		primaryName:  primaryName,
		fallbackName: fallbackName,
	}
}

func (f *FailoverLLM) Name() string {
	return f.primary.Name()
}

func (f *FailoverLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		buffered, err := collectLLM(ctx, f.primary, req, stream)
		if err == nil {
			for _, resp := range buffered {
				if !yield(resp, nil) {
					return
				}
			}
			return
		}
		if ctx.Err() != nil {
			yield(nil, err)
			return
		}
		log.Printf("llm %s failed (%v); retrying with %s", f.primaryName, err, f.fallbackName)
		for resp, fallbackErr := range f.fallback.GenerateContent(ctx, req, stream) {
			if !yield(resp, fallbackErr) {
				return
			}
		}
	}
}

func collectLLM(ctx context.Context, llm model.LLM, req *model.LLMRequest, stream bool) ([]*model.LLMResponse, error) {
	var out []*model.LLMResponse
	for resp, err := range llm.GenerateContent(ctx, req, stream) {
		if err != nil {
			return nil, err
		}
		out = append(out, resp)
	}
	return out, nil
}
