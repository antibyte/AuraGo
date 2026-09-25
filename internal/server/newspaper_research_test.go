package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/newspaper"

	openai "github.com/sashabaranov/go-openai"
)

type newspaperStoryTestClient struct {
	request  openai.ChatCompletionRequest
	response openai.ChatCompletionResponse
}

func (c *newspaperStoryTestClient) CreateChatCompletion(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.request = request
	return c.response, nil
}

func (*newspaperStoryTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, fmt.Errorf("unexpected streamed story request")
}

func TestNewspaperStoryRequestsEnoughJSONOutput(t *testing.T) {
	auto := false
	cfg := &config.Config{}
	cfg.Newspaper.Enabled = true
	cfg.LLM.Provider = "stepfun"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "step-5-preview"
	cfg.Providers = []config.ProviderEntry{{
		ID: "stepfun", Type: "openai", Model: "step-5-preview",
		ContextWindow: 256000, MaxOutputTokens: 256000,
		Capabilities: config.ProviderCapabilities{Auto: &auto, StructuredOutputs: true},
	}}
	client := &newspaperStoryTestClient{response: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		FinishReason: openai.FinishReasonStop,
		Message:      openai.ChatCompletionMessage{Content: `{"headline":"New library approved","deck":"The council voted today.","paragraphs":[{"text":"A new library was approved.","evidence_quote":"The council approved a new library on Tuesday."}]}`},
	}}}}
	s := &Server{Cfg: cfg, LLMClient: client}
	source := newspaper.Source{ID: "src-1", Title: "New library", Publisher: "Example News", Excerpt: "The council approved a new library on Tuesday. Construction starts next year."}
	story, err := s.newspaperWriteStory(context.Background(), newspaper.DefaultProfile(), "culture", source)
	if err != nil {
		t.Fatal(err)
	}
	if story.Headline != "New library approved" || len(story.Paragraphs) != 1 {
		t.Fatalf("unexpected story: %+v", story)
	}
	if client.request.MaxTokens != llm.ReasoningOutputTokens {
		t.Fatalf("output budget = %d, want %d", client.request.MaxTokens, llm.ReasoningOutputTokens)
	}
	if client.request.ResponseFormat == nil || client.request.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
		t.Fatalf("JSON response format not requested: %+v", client.request.ResponseFormat)
	}
	client.response.Choices[0].FinishReason = openai.FinishReasonLength
	if _, err := s.newspaperWriteStory(context.Background(), newspaper.DefaultProfile(), "culture", source); !errors.Is(err, llm.ErrJSONCompletionTruncated) {
		t.Fatalf("truncated completion error = %v", err)
	}
	client.response.Choices[0].FinishReason = openai.FinishReasonStop
	client.response.Choices[0].Message.Content = `[]`
	if _, err := s.newspaperWriteStory(context.Background(), newspaper.DefaultProfile(), "culture", source); !errors.Is(err, llm.ErrJSONCompletionInvalid) {
		t.Fatalf("invalid story schema error = %v", err)
	}
	cfg.LLM.Model = "ordinary-editor"
	cfg.Providers[0].Model = cfg.LLM.Model
	cfg.Providers[0].ContextWindow = 8192
	cfg.Providers[0].MaxOutputTokens = 8192
	client.response.Choices[0].Message.Content = `{"headline":"New library approved","paragraphs":[{"text":"A new library was approved.","evidence_quote":"The council approved a new library on Tuesday."}]}`
	if _, err := s.newspaperWriteStory(context.Background(), newspaper.DefaultProfile(), "culture", source); err != nil {
		t.Fatalf("small-context provider: %v", err)
	}
	if client.request.MaxTokens <= 1500 || client.request.MaxTokens >= 8192 {
		t.Fatalf("small-context output budget = %d", client.request.MaxTokens)
	}
}

func TestNewspaperCandidateOrderBalancesSections(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science", "culture"}
	p.Interests = []string{"local theatre", "energy policy"}
	queries := []newspaperQuery{
		{Section: "science"}, {Section: "culture"},
		{Section: "interests"}, {Section: "interests"},
		{Section: "science"}, {Section: "science"}, {Section: "culture"},
	}
	got := newspaperCandidateOrder(p, queries, 4)
	want := []int{4, 6, 2, 5, 1, 3, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate order = %v, want %v", got, want)
	}
}
