package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/llm"
	"aurago/internal/security"
)

const flowAIMaxTokens = 4096

// flowLLM performs the single LLM call of an AI step node, charged to the "flows" budget.
type flowLLM struct {
	s         *Server
	newClient func(cfg *config.Config, p config.ProviderEntry) llm.ChatClient
	blocked   func() bool
}

func newFlowLLM(s *Server) *flowLLM {
	return &flowLLM{s: s,
		newClient: func(cfg *config.Config, p config.ProviderEntry) llm.ChatClient {
			return llm.NewClientFromProviderWithConfig(cfg, p.Type, p.BaseURL, p.APIKey, p.AccountID)
		},
		blocked: func() bool { return s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("flows") },
	}
}

// Step implements flows.LLMStepper.
func (f *flowLLM) Step(ctx context.Context, req flows.LLMRequest) (flows.LLMResponse, error) {
	cfg := f.s.ConfigSnapshot()
	if cfg == nil {
		return flows.LLMResponse{}, flows.NewNodeError("FLOW_AI_UNAVAILABLE", "the configuration is not ready")
	}
	if f.blocked() {
		return flows.LLMResponse{}, flows.NewNodeError("FLOW_BUDGET_EXCEEDED", "the daily AI budget is used up")
	}
	client, model, mainRoute, err := f.route(cfg, req.Model)
	if err != nil {
		return flows.LLMResponse{}, err
	}
	messages := make([]openai.ChatCompletionMessage, 0, 2)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: system})
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: req.Prompt})
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = flowAIMaxTokens
	}
	request := openai.ChatCompletionRequest{Model: model, Messages: messages, Temperature: 0.2, MaxTokens: maxTokens}
	if req.JSONSchema != nil && mainRoute {
		request.ResponseFormat = llm.JSONResponseFormat(llm.ResolveConfigProviderCapabilities(cfg).StructuredOutputs)
	}
	resp, err := client.CreateChatCompletion(ctx, request)
	if f.s.BudgetTracker != nil && (err == nil || resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0) {
		used := resp.Model
		if used == "" {
			used = model
		}
		f.s.BudgetTracker.RecordForCategory("flows", used, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	if err != nil {
		return flows.LLMResponse{}, fmt.Errorf("the AI request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return flows.LLMResponse{}, errors.New("the AI model returned no answer")
	}
	out := flows.LLMResponse{Model: resp.Model, InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens}
	if out.Model == "" {
		out.Model = model
	}
	plain := strings.TrimSpace(security.StripThinkingTags(resp.Choices[0].Message.Content))
	if req.JSONSchema == nil {
		out.Text = plain
		return out, nil
	}
	content, jerr := llm.JSONContentFromResponse(resp)
	if jerr != nil {
		// The AI node validates the answer and asks once more.
		out.Text = plain
		return out, nil
	}
	out.Text = content
	var obj map[string]any
	if json.Unmarshal([]byte(content), &obj) == nil {
		out.JSON = obj
	}
	return out, nil
}

// route picks client and model: the node's provider id, else flows.ai_provider, else the
// main model. mainRoute reports whether the main client is used.
func (f *flowLLM) route(cfg *config.Config, providerID string) (llm.ChatClient, string, bool, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		providerID = strings.TrimSpace(cfg.Flows.AIProvider)
	}
	if providerID != "" {
		for _, p := range cfg.Providers {
			if p.ID == providerID {
				return f.newClient(cfg, p), p.Model, false, nil
			}
		}
		return nil, "", false, flows.NewNodeError("FLOW_AI_UNAVAILABLE", "the AI model %q is not configured", providerID)
	}
	if f.s.LLMClient == nil {
		return nil, "", false, flows.NewNodeError("FLOW_AI_UNAVAILABLE", "no AI model is configured")
	}
	return f.s.LLMClient, cfg.LLM.Model, true, nil
}

// Flow secrets live in the vault as "easydrag_<name>". The editor manages them through
// /api/desktop/flows/secrets. Flows can never read other vault entries.
const flowSecretPrefix = "easydrag_"

var flowSecretNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

// flowSecrets resolves secret_ref parameters and registers the values with the scrubber.
type flowSecrets struct{ s *Server }

// ReadSecret implements flows.SecretReader.
func (f flowSecrets) ReadSecret(name string) (string, error) {
	if !flowSecretNamePattern.MatchString(name) {
		return "", fmt.Errorf("%q is not a valid flow secret name", name)
	}
	if f.s.Vault == nil {
		return "", errors.New("the vault is not available")
	}
	value, err := f.s.Vault.ReadSecret(flowSecretPrefix + name)
	if errors.Is(err, security.ErrSecretNotFound) {
		return "", fmt.Errorf("the flow secret %q does not exist; add it in EasyDrag", name)
	}
	if err != nil {
		return "", fmt.Errorf("the flow secret %q cannot be read: %w", name, err)
	}
	security.RegisterSensitive(value)
	return value, nil
}
