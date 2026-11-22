package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/budget"
	"aurago/internal/chibisensei"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"
	"aurago/internal/security"
)

const (
	// chibiBudgetCategory is the budget category generation and cleaning calls are checked against and charged to.
	chibiBudgetCategory = "chibi_sensei"
	// chibiSchemaName names the json_schema response format of a call.
	chibiSchemaName = "chibi_sensei_answer"
	// chibiMaxAnswerBytes bounds the answer content a call accepts.
	chibiMaxAnswerBytes = 1 << 20
)

var (
	errChibiProviderUnavailable = errors.New("the generator model is not available")
	errChibiBudget              = errors.New("the daily AI budget is used up")
	errChibiWithheld            = errors.New("the prompt was withheld by the security scan")
	errChibiOutputCut           = errors.New("the answer was cut off at the token limit")
	errChibiBadAnswer           = errors.New("the model returned no usable answer")
	errChibiContext             = errors.New("the prompt exceeds the configured model context budget")
)

// chibiRoute is the generator route of one job: client, model, structured-output capability, limits and the
// credentials the client may send (an error text must never echo them).
type chibiRoute struct {
	client      llm.ChatClient
	model       string
	structured  bool
	limits      llm.ModelLimits
	credentials []string
	release     func()
}

// chibiGeneratorConfig resolves the configuration of the generator route without building a client:
//
//   - chibi_sensei.generator.provider names a provider entry that must allow general chat and have credentials;
//     its client is built for the job and the model is the entry's (or generator.model);
//   - without a provider the main route is used, with the main client and its failover; generator.model alone
//     builds a client for the main provider with that model and no failover;
//   - an empty model, or no client at all, is errChibiProviderUnavailable.
//
// ownClient says that the job builds its own client. GET /meta reports availability from the same function,
// so the UI never offers a job the route would refuse.
func (s *Server) chibiGeneratorConfig(cfg *config.Config) (gcfg *config.Config, ownClient bool, err error) {
	if cfg == nil {
		return nil, false, errChibiProviderUnavailable
	}
	providerID := strings.TrimSpace(cfg.ChibiSensei.Generator.Provider)
	modelID := strings.TrimSpace(cfg.ChibiSensei.Generator.Model)
	if providerID != "" {
		found := cfg.FindProvider(providerID)
		if found == nil {
			return nil, false, errChibiProviderUnavailable
		}
		entry := *found
		if modelID != "" {
			entry.Model = modelID
		}
		if ok, _ := flowChatProviderEntryOK(&entry); !ok || entry.HasMaskedCredentials() ||
			(entry.AuthType == "oauth2" && entry.RuntimeAPIKey() == "") {
			return nil, false, errChibiProviderUnavailable
		}
		gcfg, ownClient = llm.TaskProviderConfig(cfg, entry), true
	} else {
		gcfg = cfg.Clone()
		if modelID != "" {
			gcfg.LLM.Model = modelID
			ownClient = true
		}
	}
	if strings.TrimSpace(gcfg.LLM.Model) == "" {
		return nil, false, errChibiProviderUnavailable
	}
	if !ownClient && s.LLMClient == nil && s.chibiSenseiLLMOverride == nil {
		return nil, false, errChibiProviderUnavailable
	}
	return gcfg, ownClient, nil
}

// chibiGeneratorStatus reports whether generation jobs can run and which model they would use.
func (s *Server) chibiGeneratorStatus(cfg *config.Config) (available bool, model string) {
	gcfg, _, err := s.chibiGeneratorConfig(cfg)
	if err != nil {
		return false, ""
	}
	return true, gcfg.LLM.Model
}

// chibiGeneratorRoute builds the route of one job (copy of storymakerAgentResolveRoute with the credential
// handling of flowLLM.route). s.chibiSenseiLLMOverride replaces the client in tests. The caller must call
// release when the job is over.
func (s *Server) chibiGeneratorRoute(cfg *config.Config) (*chibiRoute, error) {
	gcfg, ownClient, err := s.chibiGeneratorConfig(cfg)
	if err != nil {
		return nil, err
	}
	applyGameMakerStreamIdle(gcfg)
	var client llm.ChatClient
	switch {
	case s.chibiSenseiLLMOverride != nil:
		client = s.chibiSenseiLLMOverride
	case ownClient:
		// The global failover client overwrites request.Model with its system route.
		gcfg.FallbackLLM.Enabled = false
		client = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(gcfg, gcfg.LLM.ProviderType, gcfg.LLM.BaseURL, gcfg.LLM.APIKey, gcfg.LLM.AccountID, gcfg.LLM.GatewayID))
	default:
		client = s.LLMClient
	}
	if client == nil {
		return nil, errChibiProviderUnavailable
	}
	keys := []string{gcfg.LLM.APIKey}
	if !ownClient {
		keys = append(keys, gcfg.FallbackLLM.APIKey) // the main client can fail over
	}
	for _, k := range keys {
		registerStaticLLMSecret(k)
	}
	registerStaticLLMSecret(gcfg.AIGateway.Token)
	return &chibiRoute{client: client, model: gcfg.LLM.Model, structured: llm.ResolveConfigProviderCapabilities(gcfg).StructuredOutputs,
		limits: gameMakerRouteLimits(gcfg), credentials: keys, release: flowCredentialScope(append(append([]string{}, keys...), gcfg.AIGateway.Token)...)}, nil
}

// chibiLLM makes the bounded one-shot calls of generation and cleaning.
type chibiLLM struct{ s *Server }

// chibiBudgetBlocked reports a used-up daily budget.
func (s *Server) chibiBudgetBlocked() bool {
	return s.BudgetTracker != nil && s.BudgetTracker.IsBlocked(chibiBudgetCategory)
}

// Complete makes one call: budget check, injection scan of the composed user prompt, one strict json_schema
// request (with one json_object retry when the provider refuses the format) and a bounded, normalized answer.
// The error is one of the errChibi* values, ctx's own error, or an error whose text is scrubbed and bounded.
func (c chibiLLM) Complete(ctx context.Context, rt *chibiRoute, system, user string, schema map[string]any, temp float32, maxTokens int) (string, error) {
	s := c.s
	// This only checks the budget before a call. Reserving budget inside the tracker (so that parallel jobs
	// cannot overshoot it together) is a general AuraGo matter in internal/budget and out of scope here; the
	// number of concurrent model jobs is bounded by the runner's two job slots instead.
	if s.chibiBudgetBlocked() {
		return "", errChibiBudget
	}
	guardian := flowPromptGuardian
	if s.Guardian != nil {
		guardian = s.Guardian
	}
	scan, withhold := security.GateScanComposedPrompt(guardian, user)
	if withhold {
		if s.Logger != nil {
			s.Logger.Warn("[ChibiSensei] Prompt withheld by the injection scan", "threat", scan.Level.String(), "patterns", scan.Patterns, "incomplete", scan.Incomplete)
		}
		return "", errChibiWithheld
	}
	if security.ComposedPromptNeedsReview(scan) && s.Logger != nil {
		s.Logger.Info("[ChibiSensei] Prompt has advisory injection findings", "threat", scan.Level.String(), "patterns", scan.Patterns)
	}

	request := openai.ChatCompletionRequest{Model: rt.model, Temperature: temp, MaxTokens: flowAIMaxTokensFor(maxTokens, rt.limits),
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		}}
	if rt.structured && schema != nil {
		if data, err := json.Marshal(schema); err == nil {
			request.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{Name: chibiSchemaName, Schema: json.RawMessage(data), Strict: true}}
		}
	}
	// go-openai refuses max_tokens and a temperature other than 1 for the o1, o3, o4 and gpt-5 families.
	if errors.Is(openai.NewReasoningValidator().Validate(request), openai.ErrReasoningModelMaxTokensDeprecated) {
		request.MaxTokens, request.MaxCompletionTokens, request.Temperature = 0, request.MaxTokens, 0
	}

	ctx = budget.WithTracking(ctx, s.BudgetTracker, chibiBudgetCategory)
	limit := max(request.MaxTokens, request.MaxCompletionTokens)
	resp, err := budget.CompleteChat(ctx, rt.client, request, rt.limits.Route, chibiBudgetCategory, limit, prompts.CountTokensForModelContext)
	if err != nil && ctx.Err() == nil && flowAIFormatRejected(request, err) {
		request.ResponseFormat = llm.JSONResponseFormat(true)
		resp, err = budget.CompleteChat(ctx, rt.client, request, rt.limits.Route, chibiBudgetCategory, limit, prompts.CountTokensForModelContext)
	}
	switch {
	case err == nil:
	case ctx.Err() != nil:
		return "", ctx.Err()
	case errors.Is(err, budget.ErrCompletionBudgetBlocked):
		return "", errChibiBudget
	case errors.Is(err, budget.ErrCompletionBudgetUnavailable):
		return "", errChibiContext
	default:
		return "", flowScrubbedError("the AI request failed: ", err, rt.credentials...)
	}
	if len(resp.Choices) == 0 {
		return "", errChibiBadAnswer
	}
	choice := resp.Choices[0]
	if strings.EqualFold(strings.TrimSpace(string(choice.FinishReason)), string(openai.FinishReasonLength)) {
		return "", errChibiOutputCut
	}
	if len(choice.Message.Content) > chibiMaxAnswerBytes {
		return "", errChibiBadAnswer
	}
	text := strings.TrimSpace(security.StripThinkingTags(choice.Message.Content))
	out, err := llm.NormalizeJSONContent(text)
	if err != nil || strings.TrimSpace(out) == "" {
		return "", errChibiBadAnswer
	}
	return out, nil
}

// chibiJobError maps a call error to the *chibisensei.JobError the UI translates. The job result carries only
// the fixed message of the code; the scrubbed cause of a provider failure goes to the log.
func (s *Server) chibiJobError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, errChibiProviderUnavailable):
		return &chibisensei.JobError{Code: "provider_unavailable", Message: "The generator model is not available."}
	case errors.Is(err, errChibiBudget):
		return &chibisensei.JobError{Code: "budget_exceeded", Message: "The daily AI budget is used up."}
	case errors.Is(err, errChibiWithheld):
		return &chibisensei.JobError{Code: "prompt_withheld", Message: "The security scan withheld the request."}
	case errors.Is(err, errChibiOutputCut):
		return &chibisensei.JobError{Code: "output_cut", Message: "The answer was cut off at the token limit."}
	case errors.Is(err, errChibiBadAnswer):
		return &chibisensei.JobError{Code: "invalid_answer", Message: "The model returned an answer that could not be used."}
	case errors.Is(err, errChibiContext):
		return &chibisensei.JobError{Code: "model_error", Message: "The request is too long for the model."}
	}
	if s != nil && s.Logger != nil {
		s.Logger.Warn("[ChibiSensei] Generator call failed", "error", err)
	}
	return &chibisensei.JobError{Code: "model_error", Message: "The model request failed."}
}
