package decisions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

const (
	openRouterOrigin                     = "https://openrouter.ai"
	openRouterModelListPath              = "/api/v1/models"
	openRouterDecisionsPath              = "/api/alpha/decisions"
	openRouterHTTPTimeout                = 30 * time.Second
	openRouterModelCacheTTL              = 5 * time.Minute
	openRouterMaxCatalogBytes            = 8 << 20
	openRouterMaxResponseBytes           = 1 << 20
	openRouterMaxRequestBytes            = 3 << 20
	openRouterMaxImages                  = 4
	openRouterMaxImageBytes              = 300 << 10
	openRouterMaxTotalImageBytes         = openRouterMaxImages * openRouterMaxImageBytes
	openRouterMaxContextTokens     int64 = 1 << 20
	openRouterProbabilityTolerance       = 0.01
	openRouterScoreTolerance             = 0.1
)

// DecisionModel describes a model returned by OpenRouter's Decisions catalog.
// Pricing values are kept as strings because that is how the catalog returns
// decimal prices.
type DecisionModel struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	InputModalities   []string              `json:"input_modalities"`
	ContextLength     int                   `json:"context_length"`
	Pricing           *DecisionModelPricing `json:"pricing,omitempty"`
	SupportsDecisions bool                  `json:"supports_decisions"`
	SupportsImages    bool                  `json:"supports_images"`
}

// DecisionModelPricing contains the catalog's input and output prices.
type DecisionModelPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

// OpenRouterClient talks only to OpenRouter's canonical HTTPS API origin.
// Models may be listed without a key; Evaluate always requires one.
type OpenRouterClient struct {
	apiKey            string
	httpClient        *http.Client
	releaseCredential func()
	closeOnce         sync.Once

	cacheMu  sync.RWMutex
	models   []DecisionModel
	modelsAt time.Time
}

// NewOpenRouterClient validates the optional base URL and creates a bounded
// HTTP client. A blank key permits public model-catalog browsing only.
func NewOpenRouterClient(baseURL, apiKey string) (*OpenRouterClient, error) {
	return newOpenRouterClient(baseURL, apiKey, false)
}

// NewOpenRouterClientScoped keeps a short-lived credential in the exact-match
// scrubber registry only while its owner retains this client. Call Close after
// requests using the client have drained.
func NewOpenRouterClientScoped(baseURL, apiKey string) (*OpenRouterClient, error) {
	return newOpenRouterClient(baseURL, apiKey, true)
}

func newOpenRouterClient(baseURL, apiKey string, scopedCredential bool) (*OpenRouterClient, error) {
	if _, err := canonicalOpenRouterBase(baseURL); err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	var releaseCredential func()
	if apiKey != "" {
		if scopedCredential {
			releaseCredential = security.RegisterScopedSensitiveExact(apiKey)
		} else {
			security.RegisterSensitive(apiKey)
		}
	}
	return &OpenRouterClient{
		apiKey: apiKey, releaseCredential: releaseCredential,
		httpClient: &http.Client{
			Timeout: openRouterHTTPTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// Close releases any scoped credential registration held by the client.
func (client *OpenRouterClient) Close() {
	if client == nil {
		return
	}
	client.closeOnce.Do(func() {
		if client.releaseCredential != nil {
			client.releaseCredential()
		}
	})
}

func canonicalOpenRouterBase(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return openRouterOrigin, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host != "openrouter.ai" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", NewError(CodeInvalidRequest, errors.New("OpenRouter base URL must use the canonical HTTPS origin"))
	}
	if parsed.Path != "" && parsed.Path != "/api/v1" && parsed.Path != "/api/v1/" {
		return "", NewError(CodeInvalidRequest, errors.New("OpenRouter base URL may only include the /api/v1 base path"))
	}
	return openRouterOrigin, nil
}

// Models lists Decisions-capable models. A successful response is cached for a
// short period and returned as detached data so callers cannot mutate the cache.
func (client *OpenRouterClient) Models(ctx context.Context) ([]DecisionModel, error) {
	if client == nil || client.httpClient == nil {
		return nil, NewError(CodeInvalidRequest, errors.New("OpenRouter client is not configured"))
	}
	if ctx == nil {
		return nil, NewError(CodeInvalidRequest, errors.New("context is required"))
	}
	if err := ctx.Err(); err != nil {
		return nil, NewError(CodeCancelled, err)
	}
	if models, ok := client.cachedModels(time.Now()); ok {
		return models, nil
	}

	requestURL := openRouterOrigin + openRouterModelListPath + "?output_modalities=decisions"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, NewError(CodeInvalidRequest, err)
	}
	request.Header.Set("Accept", "application/json")
	client.authorize(request)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, NewError(CodeExternalProviderFailed, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, NewError(openRouterFailureCode(response.StatusCode, CodeExternalProviderFailed), nil)
	}
	body, err := readOpenRouterBody(response.Body, openRouterMaxCatalogBytes)
	if err != nil {
		return nil, NewError(CodeExternalProviderFailed, err)
	}
	var catalog openRouterModelList
	if err := json.Unmarshal(body, &catalog); err != nil || catalog.Data == nil {
		return nil, NewError(CodeExternalProviderFailed, errors.New("OpenRouter model catalog response is malformed"))
	}

	models, err := decisionModels(catalog.Data)
	if err != nil {
		return nil, NewError(CodeExternalProviderFailed, err)
	}
	client.cacheMu.Lock()
	client.models = cloneDecisionModels(models)
	client.modelsAt = time.Now()
	client.cacheMu.Unlock()
	return cloneDecisionModels(models), nil
}

func (client *OpenRouterClient) cachedModels(now time.Time) ([]DecisionModel, bool) {
	client.cacheMu.RLock()
	defer client.cacheMu.RUnlock()
	if client.modelsAt.IsZero() || now.Sub(client.modelsAt) >= openRouterModelCacheTTL {
		return nil, false
	}
	return cloneDecisionModels(client.models), true
}

func cloneDecisionModels(models []DecisionModel) []DecisionModel {
	cloned := make([]DecisionModel, len(models))
	for index, model := range models {
		cloned[index] = model
		cloned[index].InputModalities = append([]string(nil), model.InputModalities...)
		if model.Pricing != nil {
			pricing := *model.Pricing
			cloned[index].Pricing = &pricing
		}
	}
	return cloned
}

// Evaluate sends one Decisions request, validates the complete upstream
// response, and maps every probability back to the caller's original options.
func (client *OpenRouterClient) Evaluate(ctx context.Context, model string, request Request) (Response, error) {
	if client == nil || client.httpClient == nil {
		return Response{}, NewError(CodeInvalidRequest, errors.New("OpenRouter client is not configured"))
	}
	if ctx == nil {
		return Response{}, NewError(CodeInvalidRequest, errors.New("context is required"))
	}
	if strings.TrimSpace(model) == "" || strings.TrimSpace(model) != model {
		return Response{}, NewError(CodeInvalidRequest, errors.New("a model id is required"))
	}
	if client.apiKey == "" {
		return Response{}, NewError(CodeInvalidRequest, errors.New("an OpenRouter API key is required to evaluate"))
	}
	if err := ValidateRequest(request); err != nil {
		return Response{}, NewError(CodeInvalidRequest, err)
	}
	if err := ctx.Err(); err != nil {
		return Response{}, NewError(CodeCancelled, err)
	}

	models, err := client.Models(ctx)
	if err != nil {
		return Response{}, err
	}
	selected, ok := findDecisionModel(models, model)
	if !ok || !selected.SupportsDecisions {
		return Response{}, NewError(CodeInvalidRequest, errors.New("selected model is not verified for Decisions"))
	}
	if len(request.Images) > 0 && !selected.SupportsImages {
		return Response{}, NewError(CodeInvalidRequest, errors.New("selected model does not advertise image input"))
	}

	wireQuestions, candidates, err := makeOpenRouterQuestions(request.Questions)
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, err)
	}
	state, imageTokenEstimate, err := makeOpenRouterState(request.State, request.Images)
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, err)
	}
	wire := openRouterDecisionRequest{Model: model, State: state, Questions: wireQuestions}
	body, err := json.Marshal(wire)
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, errors.New("encode OpenRouter Decisions request"))
	}
	if len(body) > openRouterMaxRequestBytes {
		return Response{}, NewError(CodeInvalidRequest, errors.New("OpenRouter Decisions request exceeds the byte limit"))
	}
	if selected.ContextLength <= 0 {
		return Response{}, NewError(CodeInputTokenBudgetExceeded, errors.New("selected model has no verified context limit"))
	}
	contextLimit := int64(selected.ContextLength)
	if contextLimit > openRouterMaxContextTokens {
		contextLimit = openRouterMaxContextTokens
	}
	textState, err := openRouterTextState(request.State)
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, errors.New("estimate OpenRouter Decisions request context"))
	}
	textRequest, err := json.Marshal(openRouterDecisionRequest{Model: model, State: textState, Questions: wireQuestions})
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, errors.New("estimate OpenRouter Decisions request context"))
	}
	contextEstimate := int64(len(textRequest)) + imageTokenEstimate + 512
	if isOpenRouterClefModel(model) && int64(len(textRequest))+512 > 2000 {
		return Response{}, NewError(CodeInputTokenBudgetExceeded, errors.New("OpenRouter Clef Decisions requests are limited to 2000 conservative text tokens"))
	}
	if contextEstimate > contextLimit {
		return Response{}, NewError(CodeInputTokenBudgetExceeded, errors.New("OpenRouter Decisions request exceeds the conservative context limit"))
	}
	if err := ctx.Err(); err != nil {
		return Response{}, NewError(CodeCancelled, err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterOrigin+openRouterDecisionsPath, strings.NewReader(string(body)))
	if err != nil {
		return Response{}, NewError(CodeInvalidRequest, err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")
	client.authorize(httpRequest)
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return Response{}, NewError(CodeExternalOutcomeUnknown, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Response{}, NewError(openRouterFailureCode(response.StatusCode, CodeExternalOutcomeUnknown), nil)
	}
	responseBody, err := readOpenRouterBody(response.Body, openRouterMaxResponseBytes)
	if err != nil {
		return Response{}, NewError(CodeExternalOutcomeUnknown, err)
	}
	var decoded openRouterDecisionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		partial := partialOpenRouterResponse(model, request, candidates, responseBody)
		return partial, NewError(CodeExternalOutcomeUnknown, errors.New("OpenRouter Decisions response is malformed"))
	}
	result, err := mapOpenRouterResponse(model, request, candidates, decoded)
	if err != nil {
		return result, NewError(CodeExternalOutcomeUnknown, err)
	}
	return result, nil
}

func isOpenRouterClefModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "clef")
}

func (client *OpenRouterClient) authorize(request *http.Request) {
	if client.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+client.apiKey)
	}
}

// Explicit refusals have useful, safe classifications. All other failed POSTs
// retain an unknown outcome and are never automatically replayed.
func openRouterFailureCode(status int, fallback string) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return CodeProviderAuthFailed
	case http.StatusPaymentRequired:
		return CodeProviderCreditExhausted
	case http.StatusTooManyRequests:
		return CodeProviderRateLimited
	default:
		return fallback
	}
}

func findDecisionModel(models []DecisionModel, id string) (DecisionModel, bool) {
	for _, model := range models {
		if model.ID == id {
			return model, true
		}
	}
	return DecisionModel{}, false
}

func readOpenRouterBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, errors.New("read OpenRouter response")
	}
	if int64(len(data)) > limit {
		return nil, errors.New("OpenRouter response exceeds the byte limit")
	}
	return data, nil
}

type openRouterModelList struct {
	Data []openRouterCatalogModel `json:"data"`
}

type openRouterCatalogModel struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	ContextLength int                    `json:"context_length"`
	Architecture  openRouterArchitecture `json:"architecture"`
	Pricing       *DecisionModelPricing  `json:"pricing"`
	TopProvider   struct {
		ContextLength int `json:"context_length"`
	} `json:"top_provider"`
}

type openRouterArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

func decisionModels(entries []openRouterCatalogModel) ([]DecisionModel, error) {
	models := make([]DecisionModel, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			return nil, errors.New("OpenRouter catalog contains a model without an id")
		}
		if _, exists := seen[entry.ID]; exists {
			return nil, errors.New("OpenRouter catalog contains a duplicate model id")
		}
		seen[entry.ID] = struct{}{}
		if !containsModality(entry.Architecture.OutputModalities, "decisions") {
			continue
		}
		name := entry.Name
		if strings.TrimSpace(name) == "" {
			name = entry.ID
		}
		modalities := append([]string(nil), entry.Architecture.InputModalities...)
		contextLength := entry.ContextLength
		if limit := entry.TopProvider.ContextLength; limit > 0 && (contextLength <= 0 || limit < contextLength) {
			contextLength = limit
		}
		models = append(models, DecisionModel{
			ID:                entry.ID,
			Name:              name,
			InputModalities:   modalities,
			ContextLength:     contextLength,
			Pricing:           entry.Pricing,
			SupportsDecisions: true,
			SupportsImages:    containsModality(modalities, "image"),
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

func containsModality(modalities []string, expected string) bool {
	for _, modality := range modalities {
		if strings.EqualFold(strings.TrimSpace(modality), expected) {
			return true
		}
	}
	return false
}

type openRouterDecisionRequest struct {
	Model     string                        `json:"model"`
	State     any                           `json:"state"`
	Questions map[string]openRouterQuestion `json:"questions"`
}

type openRouterQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria"`
}

func makeOpenRouterQuestions(questions []Question) (map[string]openRouterQuestion, map[string][]Option, error) {
	wire := make(map[string]openRouterQuestion, len(questions))
	candidates := make(map[string][]Option, len(questions))
	for _, question := range questions {
		options := optionsForQuestion(question)
		var criteria any
		switch question.Type {
		case "choice":
			choiceCriteria := make(map[string]string, len(options))
			for _, option := range options {
				choiceCriteria[option.ID] = option.Label
			}
			criteria = choiceCriteria
		case "noul":
			noulCriteria := map[string]string{"false": "No", "true": "Yes"}
			for _, option := range options {
				if option.ID == "no" {
					noulCriteria["false"] = option.Label
				}
				if option.ID == "yes" {
					noulCriteria["true"] = option.Label
				}
			}
			criteria = noulCriteria
		case "score":
			scoreCriteria := make([]string, len(options))
			for index, option := range options {
				scoreCriteria[index] = option.Label
			}
			criteria = scoreCriteria
		default:
			return nil, nil, fmt.Errorf("unsupported question type %q", question.Type)
		}
		wire[question.ID] = openRouterQuestion{Type: question.Type, Instructions: question.Instructions, Criteria: criteria}
		candidates[question.ID] = options
	}
	return wire, candidates, nil
}

type openRouterImageURL struct {
	URL string `json:"url"`
}

type openRouterImagePart struct {
	Type     string             `json:"type"`
	ImageURL openRouterImageURL `json:"image_url"`
}

func makeOpenRouterState(state any, images []Image) (any, int64, error) {
	if len(images) > openRouterMaxImages {
		return nil, 0, fmt.Errorf("OpenRouter accepts at most %d images per request", openRouterMaxImages)
	}
	stateText, err := openRouterTextState(state)
	if err != nil {
		return nil, 0, errors.New("encode request state")
	}
	if len(images) == 0 {
		return stateText, 0, nil
	}
	parts := make([]any, 0, len(images)+1)
	parts = append(parts, stateText)
	var totalBytes int
	var imageTokens int64
	for index, image := range images {
		mediaType, _, err := mime.ParseMediaType(image.MIME)
		if err != nil {
			return nil, 0, fmt.Errorf("image %d has an invalid MIME type", index)
		}
		mediaType = strings.ToLower(mediaType)
		if mediaType != "image/jpeg" && mediaType != "image/png" {
			return nil, 0, fmt.Errorf("OpenRouter Decisions supports JPEG and PNG image inputs only")
		}
		if len(image.Data) == 0 || len(image.Data) > openRouterMaxImageBytes {
			return nil, 0, fmt.Errorf("image %d exceeds the OpenRouter size limit", index)
		}
		totalBytes += len(image.Data)
		if totalBytes > openRouterMaxTotalImageBytes {
			return nil, 0, errors.New("OpenRouter images exceed the total size limit")
		}
		encoded := base64.StdEncoding.EncodeToString(image.Data)
		imageTokens += int64((len(encoded) + 3) / 4)
		parts = append(parts, openRouterImagePart{
			Type:     "image_url",
			ImageURL: openRouterImageURL{URL: "data:" + mediaType + ";base64," + encoded},
		})
	}
	return parts, imageTokens, nil
}

// openRouterTextState keeps caller-provided structured state as plain text.
// Only image parts built from server-authorized bytes may activate multimodal
// input in OpenRouter's state array.
func openRouterTextState(state any) (string, error) {
	if text, ok := state.(string); ok {
		return text, nil
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type openRouterDecisionResponse struct {
	Answers  map[string]openRouterAnswer `json:"answers"`
	Model    string                      `json:"model"`
	Provider string                      `json:"provider"`
	Usage    *openRouterResponseUsage    `json:"usage"`
}

type openRouterAnswer struct {
	Type          string             `json:"type"`
	Choice        *string            `json:"choice"`
	Noul          *float64           `json:"noul"`
	Score         *float64           `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Legend        map[string]string  `json:"legend"`
}

type openRouterResponseUsage struct {
	InputTokens  *int     `json:"input_tokens"`
	OutputTokens *int     `json:"output_tokens"`
	Cost         *float64 `json:"cost"`
}

func mapOpenRouterResponse(requestedModel string, request Request, candidates map[string][]Option, decoded openRouterDecisionResponse) (Response, error) {
	result := openRouterResponseBase(requestedModel, request, candidates, decoded.Model, decoded.Provider, decoded.Usage)
	if strings.TrimSpace(decoded.Model) == "" || decoded.Usage == nil {
		return result, errors.New("OpenRouter response is missing model or usage metadata")
	}
	if decoded.Usage.InputTokensInvalid() {
		return result, errors.New("OpenRouter response has invalid usage metadata")
	}
	if len(decoded.Answers) != len(request.Questions) {
		return result, errors.New("OpenRouter response question ids do not match the request")
	}
	result.Answers = make(map[string]Answer, len(request.Questions))
	for _, question := range request.Questions {
		raw, exists := decoded.Answers[question.ID]
		if !exists {
			return result, errors.New("OpenRouter response omitted a question id")
		}
		answer, err := mapOpenRouterAnswer(question, candidates[question.ID], raw)
		if err != nil {
			return result, err
		}
		result.Answers[question.ID] = answer
	}
	return result, nil
}

func (usage *openRouterResponseUsage) InputTokensInvalid() bool {
	return usage == nil || usage.InputTokens == nil || usage.OutputTokens == nil ||
		*usage.InputTokens < 0 || *usage.OutputTokens < 0 ||
		*usage.InputTokens > math.MaxInt-*usage.OutputTokens || !validReportedCost(usage.Cost)
}

func validReportedCost(cost *float64) bool {
	return cost == nil || (!math.IsNaN(*cost) && !math.IsInf(*cost, 0) && *cost >= 0)
}

func openRouterResponseBase(requestedModel string, request Request, candidates map[string][]Option, modelRevision, provider string, usage *openRouterResponseUsage) Response {
	result := Response{
		Model:         requestedModel,
		ModelRevision: modelRevision,
		Calibration:   "uncalibrated",
		Execution:     &ExternalExecution{Provider: provider, ProviderID: "openrouter"},
		Usage: Usage{
			Questions: len(request.Questions),
			Images:    len(request.Images),
		},
	}
	if candidates != nil {
		result.Usage.Candidates = countCandidates(candidates)
	}
	if usage == nil {
		return result
	}
	if usage.InputTokens != nil && *usage.InputTokens >= 0 {
		result.Usage.PromptTokens = *usage.InputTokens
	}
	if usage.OutputTokens != nil && *usage.OutputTokens >= 0 {
		result.Usage.CompletionTokens = *usage.OutputTokens
	}
	if usage.InputTokens != nil && usage.OutputTokens != nil && *usage.InputTokens >= 0 && *usage.OutputTokens >= 0 &&
		*usage.InputTokens <= math.MaxInt-*usage.OutputTokens {
		result.Usage.ContextTokens = *usage.InputTokens + *usage.OutputTokens
	}
	if validReportedCost(usage.Cost) {
		result.Usage.CostUSD = cloneFloat64(usage.Cost)
	}
	return result
}

func partialOpenRouterResponse(requestedModel string, request Request, candidates map[string][]Option, body []byte) Response {
	var metadata struct {
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	_ = json.Unmarshal(body, &metadata)

	var envelope struct {
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Usage) == 0 || string(envelope.Usage) == "null" {
		return openRouterResponseBase(requestedModel, request, candidates, metadata.Model, metadata.Provider, nil)
	}
	usage := &openRouterResponseUsage{}
	var rawUsage map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Usage, &rawUsage); err == nil {
		if raw, ok := rawUsage["input_tokens"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			var value *int
			if json.Unmarshal(raw, &value) == nil {
				usage.InputTokens = value
			}
		}
		if raw, ok := rawUsage["output_tokens"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			var value *int
			if json.Unmarshal(raw, &value) == nil {
				usage.OutputTokens = value
			}
		}
		if raw, ok := rawUsage["cost"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			var value *float64
			if json.Unmarshal(raw, &value) == nil {
				usage.Cost = value
			}
		}
	}
	return openRouterResponseBase(requestedModel, request, candidates, metadata.Model, metadata.Provider, usage)
}

func countCandidates(candidates map[string][]Option) int {
	total := 0
	for _, options := range candidates {
		total += len(options)
	}
	return total
}

func mapOpenRouterAnswer(question Question, options []Option, raw openRouterAnswer) (Answer, error) {
	if raw.Type != question.Type {
		return Answer{}, fmt.Errorf("OpenRouter response type does not match question %q", question.ID)
	}
	answer := Answer{Type: question.Type}
	providerConfidence, err := validatedProviderConfidence(raw.Confidence)
	if err != nil {
		return Answer{}, fmt.Errorf("OpenRouter response confidence is invalid for question %q", question.ID)
	}
	answer.ProviderConfidence = providerConfidence
	switch question.Type {
	case "noul":
		if raw.Noul == nil || !probabilityValid(*raw.Noul) || raw.Choice != nil || raw.Score != nil {
			return Answer{}, fmt.Errorf("OpenRouter response has an invalid noul answer for question %q", question.ID)
		}
		if len(raw.Probabilities) > 0 {
			if err := validateProbabilityKeys(raw.Probabilities, []string{"no", "yes"}); err != nil || math.Abs(raw.Probabilities["yes"]-*raw.Noul) > openRouterProbabilityTolerance {
				return Answer{}, fmt.Errorf("OpenRouter response has invalid noul probabilities for question %q", question.ID)
			}
		}
		noProbability := 1 - *raw.Noul
		answer.Probabilities = map[string]float64{"no": noProbability, "yes": *raw.Noul}
		answer.Confidence = math.Max(noProbability, *raw.Noul)
		if *raw.Noul > noProbability {
			answer.Decision = "yes"
		} else {
			answer.Decision = "no"
		}
		yes := *raw.Noul
		answer.Noul = &yes
	case "choice":
		if raw.Choice == nil || raw.Score != nil {
			return Answer{}, fmt.Errorf("OpenRouter response has an invalid choice answer for question %q", question.ID)
		}
		keys := make([]string, len(options))
		optionByID := make(map[string]Option, len(options))
		for index, option := range options {
			keys[index] = option.ID
			optionByID[option.ID] = option
		}
		probabilities, err := normalizedProbabilityMap(raw.Probabilities, keys)
		if err != nil {
			return Answer{}, fmt.Errorf("OpenRouter response has invalid choice probabilities for question %q", question.ID)
		}
		selected, ok := optionByID[*raw.Choice]
		if !ok {
			return Answer{}, fmt.Errorf("OpenRouter response selected an unknown choice for question %q", question.ID)
		}
		maximum, _ := maximumProbability(options, probabilities)
		if math.Abs(probabilities[selected.ID]-maximum) > 1e-9 {
			return Answer{}, fmt.Errorf("OpenRouter response choice is not a most likely option for question %q", question.ID)
		}
		answer.Decision = selected.ID
		answer.Choice = selected.Label
		answer.Probabilities = probabilities
		answer.Confidence = maximum
	case "score":
		if raw.Score == nil || math.IsNaN(*raw.Score) || math.IsInf(*raw.Score, 0) || raw.Choice != nil || *raw.Score < 0 || *raw.Score > float64(len(options)-1)+openRouterScoreTolerance {
			return Answer{}, fmt.Errorf("OpenRouter response has an invalid score answer for question %q", question.ID)
		}
		keys := make([]string, len(options))
		for index := range options {
			keys[index] = strconv.Itoa(index)
		}
		probabilities, err := normalizedProbabilityMap(raw.Probabilities, keys)
		if err != nil {
			return Answer{}, fmt.Errorf("OpenRouter response has invalid score probabilities for question %q", question.ID)
		}
		if raw.Legend != nil {
			if err := validateScoreLegend(options, raw.Legend); err != nil {
				return Answer{}, fmt.Errorf("OpenRouter response has an invalid score legend for question %q", question.ID)
			}
		}
		mapped := make(map[string]float64, len(options))
		expected := 0.0
		ordinalExpected := 0.0
		for index, option := range options {
			probability := probabilities[strconv.Itoa(index)]
			mapped[option.ID] = probability
			value := float64(index)
			if option.Value != nil {
				value = *option.Value
			}
			expected += probability * value
			ordinalExpected += probability * float64(index)
		}
		if math.IsNaN(expected) || math.IsInf(expected, 0) {
			return Answer{}, fmt.Errorf("OpenRouter response expected score is not finite for question %q", question.ID)
		}
		if math.Abs(ordinalExpected-*raw.Score) > openRouterScoreTolerance {
			return Answer{}, fmt.Errorf("OpenRouter response score disagrees with its probabilities for question %q", question.ID)
		}
		maximum, best := maximumProbability(options, mapped)
		answer.Decision = options[best].ID
		answer.Probabilities = mapped
		answer.Confidence = maximum
		answer.Score = &expected
	default:
		return Answer{}, fmt.Errorf("unsupported question type %q", question.Type)
	}
	return answer, nil
}

func validatedProviderConfidence(confidence *float64) (*float64, error) {
	if confidence == nil {
		return nil, nil
	}
	if !probabilityValid(*confidence) {
		return nil, errors.New("confidence must be finite and between zero and one")
	}
	return cloneFloat64(confidence), nil
}

func probabilityValid(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validateProbabilityKeys(probabilities map[string]float64, expected []string) error {
	if len(probabilities) != len(expected) {
		return errors.New("probability keys do not match candidates")
	}
	var total float64
	for _, key := range expected {
		probability, exists := probabilities[key]
		if !exists || !probabilityValid(probability) {
			return errors.New("probability is missing or invalid")
		}
		total += probability
	}
	if math.Abs(total-1) > openRouterProbabilityTolerance {
		return errors.New("probabilities are not normalized")
	}
	return nil
}

func normalizedProbabilityMap(probabilities map[string]float64, expected []string) (map[string]float64, error) {
	if err := validateProbabilityKeys(probabilities, expected); err != nil {
		return nil, err
	}
	var total float64
	for _, key := range expected {
		total += probabilities[key]
	}
	normalized := make(map[string]float64, len(expected))
	for _, key := range expected {
		normalized[key] = probabilities[key] / total
	}
	return normalized, nil
}

func maximumProbability(options []Option, probabilities map[string]float64) (float64, int) {
	maximum := math.Inf(-1)
	selectedIndex := 0
	for index, option := range options {
		probability := probabilities[option.ID]
		if probability > maximum {
			maximum = probability
			selectedIndex = index
		}
	}
	return maximum, selectedIndex
}

func cloneProbabilities(probabilities map[string]float64) map[string]float64 {
	cloned := make(map[string]float64, len(probabilities))
	for key, value := range probabilities {
		cloned[key] = value
	}
	return cloned
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func validateScoreLegend(options []Option, legend map[string]string) error {
	if len(legend) != len(options) {
		return errors.New("legend keys do not match score levels")
	}
	for index, option := range options {
		if legend[strconv.Itoa(index)] != option.Label {
			return errors.New("legend labels do not match score options")
		}
	}
	return nil
}
