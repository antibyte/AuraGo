package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/newspaper"
	"aurago/internal/prompts"
	"aurago/internal/security"

	openai "github.com/sashabaranov/go-openai"
)

func (s *Server) newspaperWriteStory(ctx context.Context, p newspaper.Profile, section string, source newspaper.Source) (newspaper.Story, error) {
	latest := s.ConfigSnapshot()
	if latest == nil || !latest.Newspaper.Enabled || latest.Newspaper.ReadOnly || s.LLMClient == nil {
		return newspaper.Story{}, errors.New("research permission revoked")
	}
	topic := section
	if section == "interests" {
		topic = strings.Join(p.Interests, ", ")
	}
	return writeNewspaperStory(ctx, p, section, topic, source, func(ctx context.Context, guide, input string) (string, error) {
		return s.newspaperCompletion(ctx, latest, s.LLMClient, guide, input)
	})
}

type newspaperCompletionFunc func(context.Context, string, string) (string, error)

func writeNewspaperStory(ctx context.Context, p newspaper.Profile, section, topic string, source newspaper.Source, complete newspaperCompletionFunc) (newspaper.Story, error) {
	return writeNewspaperStoryRepair(ctx, p, section, topic, source, complete, "")
}

func writeNewspaperStoryRepair(ctx context.Context, p newspaper.Profile, section, topic string, source newspaper.Source, complete newspaperCompletionFunc, reason string) (newspaper.Story, error) {
	guide := newspaper.Skill + "\nReturn exactly one JSON object with headline, deck and paragraphs (1-4, only as the evidence supports). Each paragraph has text and evidence_quote. The evidence_quote must be an exact consecutive substring of 20-500 characters from the decoded source text that supports that paragraph. If the page lacks enough substantiated news, return {\"declined\":true,\"headline\":\"\",\"deck\":\"\",\"paragraphs\":[]}."
	if reason != "" {
		guide += "\nRegenerate from the same source. The previous response failed validation: " + reason + ". Preserve exact evidence quotes and return complete JSON; do not invent facts."
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	payload, _ := json.Marshal(map[string]any{"language": p.Language, "section": section, "topic": topic, "publication_date": time.Now().In(loc).Format("2006-01-02"), "source": source})
	content, err := complete(ctx, guide, security.IsolateExternalData(string(payload)))
	if err != nil {
		return newspaper.Story{}, err
	}
	var raw struct {
		Declined   bool   `json:"declined"`
		Headline   string `json:"headline"`
		Deck       string `json:"deck"`
		Paragraphs []struct {
			Text          string `json:"text"`
			EvidenceQuote string `json:"evidence_quote"`
		} `json:"paragraphs"`
	}
	if strings.TrimSpace(content) == "" {
		return newspaper.Story{}, llm.ErrJSONCompletionEmpty
	}
	if err = json.Unmarshal([]byte(content), &raw); err != nil {
		return newspaper.Story{}, fmt.Errorf("%w: newspaper story schema", llm.ErrJSONCompletionInvalid)
	}
	if raw.Declined || (raw.Headline == "" && len(raw.Paragraphs) == 0 && strings.Contains(content, "\"paragraphs\"")) {
		return newspaper.Story{}, errNewspaperDeclined
	}
	story := newspaper.Story{Headline: newspaperBound(raw.Headline, 180), Deck: newspaperBound(raw.Deck, 350), Paragraphs: []newspaper.Paragraph{}}
	for _, para := range raw.Paragraphs {
		story.Paragraphs = append(story.Paragraphs, newspaper.Paragraph{Text: newspaperBound(para.Text, 1400), EvidenceQuote: para.EvidenceQuote})
	}
	return story, nil
}

func (s *Server) newspaperCompletion(ctx context.Context, cfg *config.Config, client llm.ChatClient, guide, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("newspaper") {
		return "", errors.New("provider spending policy blocks research")
	}
	route := llm.ModelRoute{ProviderID: cfg.LLM.Provider, ProviderType: cfg.LLM.ProviderType, BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model, Primary: true}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil {
		route.ContextWindowOverride, route.MaxOutputTokensOverride = provider.ContextWindow, provider.MaxOutputTokens
	}
	request := openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: guide}, {Role: openai.ChatMessageRoleUser, Content: input},
	}, Temperature: 0.2, ResponseFormat: llm.JSONResponseFormat(llm.ResolveConfigProviderCapabilities(cfg).StructuredOutputs)}
	routes := []llm.ModelRoute{route}
	if provider, ok := client.(llm.RequestRouteProvider); ok {
		if candidates := provider.CandidateRoutes(request); len(candidates) > 0 {
			routes = candidates
		}
	}
	if strings.Contains(guide, "Task: PLAN RESEARCH") {
		input = newspaperTrimPlanInput(guide, input, routes, cfg.Agent.ContextWindow)
		request.Messages[1].Content = input
	}
	request.MaxTokens = min(llm.ReasoningOutputTokens, 8192)
	for _, candidate := range routes {
		limits := llm.ResolveModelLimitsCached(candidate, cfg.Agent.ContextWindow)
		inputTokens := prompts.CountTokensForModel(guide, candidate.Model) + prompts.CountTokensForModel(input, candidate.Model) + 32
		reserve, err := llm.JSONCompletionOutputBudget(limits, min(request.MaxTokens, limits.ContextWindow-inputTokens-256), inputTokens)
		if err != nil {
			return "", fmt.Errorf("budget newspaper output: %w", err)
		}
		request.MaxTokens = min(request.MaxTokens, reserve)
	}
	response, err := client.CreateChatCompletion(ctx, request)
	// Completed responses are charged once, before JSON/structure rejection.
	if s.BudgetTracker != nil && (err == nil || response.Usage.TotalTokens > 0 || response.Usage.PromptTokens > 0 || response.Usage.CompletionTokens > 0) {
		model := response.Model
		if model == "" {
			model = cfg.LLM.Model
		}
		s.BudgetTracker.RecordForCategory("newspaper", model, response.Usage.PromptTokens, response.Usage.CompletionTokens)
	}
	if err != nil {
		return "", fmt.Errorf("complete newspaper request: %w", err)
	}
	return llm.JSONContentFromResponse(response)
}

// Trim overview metadata evenly across topics before allocating model output.
// Other trusted planning fields are retained; normal context validation still
// rejects a model too small for the resulting base request.
func newspaperTrimPlanInput(guide, input string, routes []llm.ModelRoute, contextWindow int) string {
	var payload map[string]json.RawMessage
	if json.Unmarshal([]byte(newspaperUnwrap(input)), &payload) != nil {
		return input
	}
	var leads []newspaperOverviewLead
	var topics []newspaperTopic
	if json.Unmarshal(payload["overview_leads"], &leads) != nil || json.Unmarshal(payload["topics"], &topics) != nil {
		return input
	}
	ceiling := 24 * 1024
	for len(leads) > 0 {
		fits := true
		for _, route := range routes {
			limits := llm.ResolveModelLimitsCached(route, contextWindow)
			if prompts.CountTokensForModel(guide, route.Model)+prompts.CountTokensForModel(input, route.Model)+32 > limits.ContextWindow/2 {
				fits = false
				break
			}
		}
		if fits {
			break
		}
		ceiling = ceiling * 3 / 4
		leads = newspaperOverviewPromptLeads(leads, topics, ceiling)
		payload["overview_leads"], _ = json.Marshal(leads)
		body, _ := json.Marshal(payload)
		input = security.IsolateExternalData(string(body))
	}
	return input
}
