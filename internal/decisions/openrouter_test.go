package decisions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/security"
)

type openRouterRoundTripFunc func(*http.Request) (*http.Response, error)

type nonRetryableMarker bool

func (marker nonRetryableMarker) Error() string { return "marked provider error" }

func (marker nonRetryableMarker) IsNonRetryable() bool { return bool(marker) }

func (roundTrip openRouterRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func openRouterHTTPResponse(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    &http.Request{},
	}
}

func openRouterTestClient(t *testing.T, key string, roundTrip openRouterRoundTripFunc) *OpenRouterClient {
	t.Helper()
	client, err := NewOpenRouterClient("https://openrouter.ai/api/v1", key)
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = roundTrip
	return client
}

const textDecisionCatalog = `{"data":[
	{"id":"typesafe/jev-1.13","name":"Jev Decisions","context_length":8192,"architecture":{"input_modalities":["text"],"output_modalities":["decisions"]},"pricing":{"prompt":"0.000001","completion":"0.000002"}},
	{"id":"openai/text-model","name":"Text model","context_length":4096,"architecture":{"input_modalities":["text"],"output_modalities":["text"]},"pricing":{"prompt":"0","completion":"0"}}
]}`

func TestCanonicalOpenRouterBase(t *testing.T) {
	for _, base := range []string{"", "https://openrouter.ai", "https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1/"} {
		if _, err := canonicalOpenRouterBase(base); err != nil {
			t.Errorf("canonicalOpenRouterBase(%q): %v", base, err)
		}
	}
	for _, base := range []string{
		"http://openrouter.ai",
		"https://openrouter.ai.evil.test",
		"https://openrouter.ai:443",
		"https://user@openrouter.ai",
		"https://openrouter.ai/api/v1/models",
		"https://openrouter.ai/api/v1?x=1",
		"https://openrouter.ai/#fragment",
	} {
		if _, err := canonicalOpenRouterBase(base); err == nil {
			t.Errorf("canonicalOpenRouterBase(%q) accepted an unsafe/noncanonical URL", base)
		}
	}
}

func TestOpenRouterScopedCredentialReleasesRedactionAfterClose(t *testing.T) {
	key := "oauth-scoped-decision-credential-20261013"
	client, err := NewOpenRouterClientScoped("https://openrouter.ai/api/v1", key)
	if err != nil {
		t.Fatal(err)
	}
	if got := security.Scrub(key); got == key {
		t.Fatal("scoped credential was not redacted while its client was active")
	}
	client.Close()
	client.Close()
	if got := security.Scrub(key); got != key {
		t.Fatal("scoped credential remained registered after close")
	}
}

func TestOpenRouterExplicitRefusalsAreActionableAndNeverRetried(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{{401, CodeProviderAuthFailed}, {403, CodeProviderAuthFailed}, {402, CodeProviderCreditExhausted}, {429, CodeProviderRateLimited}, {500, CodeExternalOutcomeUnknown}} {
		posts := 0
		client := openRouterTestClient(t, "key", func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet {
				return openRouterHTTPResponse(200, textDecisionCatalog, nil), nil
			}
			posts++
			return openRouterHTTPResponse(tc.status, `{"error":"untrusted private provider details"}`, nil), nil
		})
		_, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", Request{State: "test", Questions: []Question{{ID: "q", Type: "noul"}}})
		if ErrorCode(err) != tc.code || !IsNonRetryable(err) || posts != 1 || strings.Contains(err.Error(), "private") {
			t.Fatalf("status %d error=%v posts=%d", tc.status, err, posts)
		}
	}
}

func TestOpenRouterCatalogRespectsServingProviderContextLimit(t *testing.T) {
	client := openRouterTestClient(t, "", func(*http.Request) (*http.Response, error) {
		return openRouterHTTPResponse(http.StatusOK, `{"data":[{"id":"cloudflare/clef","context_length":65536,"top_provider":{"context_length":16384},"architecture":{"input_modalities":["text","image"],"output_modalities":["decisions"]}}]}`, nil), nil
	})
	models, err := client.Models(context.Background())
	if err != nil || len(models) != 1 || models[0].ContextLength != 16384 {
		t.Fatalf("serving limits: %+v %v", models, err)
	}
}

func TestOpenRouterModelsCacheDetachedCatalogAndAllowsNoKey(t *testing.T) {
	calls := 0
	client := openRouterTestClient(t, "", func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || request.URL.Host != "openrouter.ai" || request.URL.Path != openRouterModelListPath || request.URL.RawQuery != "output_modalities=decisions" {
			t.Fatalf("unexpected catalog request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatal("catalog request unexpectedly included authorization")
		}
		return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
	})

	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || !models[0].SupportsDecisions || models[0].SupportsImages || models[0].ID != "typesafe/jev-1.13" {
		t.Fatalf("Models() = %#v", models)
	}
	if models[0].Pricing == nil || models[0].Pricing.Prompt != "0.000001" {
		t.Fatalf("model pricing = %#v", models[0].Pricing)
	}
	models[0].InputModalities[0] = "changed"
	models[0].Pricing.Prompt = "changed"

	cached, err := client.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || cached[0].InputModalities[0] != "text" || cached[0].Pricing.Prompt != "0.000001" {
		t.Fatalf("catalog cache was fetched %d times or leaked mutable data: %#v", calls, cached)
	}
}

func TestOpenRouterEvaluateMapsQuestionTypesAndUsage(t *testing.T) {
	noValue, yesValue := 0.0, 1.0
	lowValue, mediumValue, highValue := 10.0, 30.0, 100.0
	request := Request{
		State: map[string]any{"ticket": "sample"},
		Questions: []Question{
			{ID: "binary", Type: "noul", Instructions: "Is it urgent?", Options: []Option{{ID: "no", Label: "No", Value: &noValue}, {ID: "yes", Label: "Yes", Value: &yesValue}}},
			{ID: "route", Type: "choice", Instructions: "Choose a route.", Options: []Option{{ID: "north", Label: "North"}, {ID: "south", Label: "South"}}},
			{ID: "priority", Type: "score", Instructions: "Rate priority.", Options: []Option{{ID: "low", Label: "Low", Value: &lowValue}, {ID: "medium", Label: "Medium", Value: &mediumValue}, {ID: "high", Label: "High", Value: &highValue}}},
		},
	}
	responseBody := `{"answers":{
		"binary":{"noul":0.75,"type":"noul"},
		"route":{"choice":"south","confidence":0.6,"probabilities":{"north":0.4,"south":0.6},"type":"choice"},
		"priority":{"confidence":0.8,"legend":{"0":"Low","1":"Medium","2":"High"},"probabilities":{"0":0.1,"1":0.1,"2":0.8},"score":1.7,"type":"score"}
	},"model":"typesafe/jev-1.13-20260917","provider":"TypeSafe","usage":{"cost":0,"input_tokens":41,"output_tokens":9}}`
	catalogCalls, decisionCalls := 0, 0
	client := openRouterTestClient(t, "test-key", func(httpRequest *http.Request) (*http.Response, error) {
		switch httpRequest.Method {
		case http.MethodGet:
			catalogCalls++
			if httpRequest.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatal("catalog request did not use the configured key")
			}
			return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
		case http.MethodPost:
			decisionCalls++
			if httpRequest.URL.Host != "openrouter.ai" || httpRequest.URL.Path != openRouterDecisionsPath {
				t.Fatalf("unexpected Decisions endpoint %s", httpRequest.URL.String())
			}
			if httpRequest.Header.Get("Authorization") != "Bearer test-key" || httpRequest.Header.Get("Content-Type") != "application/json" {
				t.Fatal("Decisions request headers are incorrect")
			}
			var wire openRouterDecisionRequest
			if err := json.NewDecoder(httpRequest.Body).Decode(&wire); err != nil {
				t.Fatal(err)
			}
			if wire.Model != "typesafe/jev-1.13" || wire.State != `{"ticket":"sample"}` {
				t.Fatalf("wire request = %#v", wire)
			}
			choiceCriteria, ok := wire.Questions["route"].Criteria.(map[string]any)
			if !ok || choiceCriteria["north"] != "North" || choiceCriteria["south"] != "South" {
				t.Fatalf("choice criteria = %#v", wire.Questions["route"].Criteria)
			}
			noulCriteria, ok := wire.Questions["binary"].Criteria.(map[string]any)
			if !ok || noulCriteria["false"] != "No" || noulCriteria["true"] != "Yes" {
				t.Fatalf("noul criteria = %#v", wire.Questions["binary"].Criteria)
			}
			scoreCriteria, ok := wire.Questions["priority"].Criteria.([]any)
			if !ok || len(scoreCriteria) != 3 || scoreCriteria[2] != "High" {
				t.Fatalf("score criteria = %#v", wire.Questions["priority"].Criteria)
			}
			return openRouterHTTPResponse(http.StatusOK, responseBody, nil), nil
		default:
			t.Fatalf("unexpected HTTP method %s", httpRequest.Method)
			return nil, nil
		}
	})

	got, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request)
	if err != nil {
		t.Fatal(err)
	}
	if catalogCalls != 1 || decisionCalls != 1 {
		t.Fatalf("catalog calls=%d Decisions calls=%d", catalogCalls, decisionCalls)
	}
	if got.Model != "typesafe/jev-1.13" || got.ModelRevision != "typesafe/jev-1.13-20260917" || got.Calibration != "uncalibrated" {
		t.Fatalf("response metadata = %#v", got)
	}
	if got.Runtime.DeviceAttested || got.Runtime.GPUOffloadVerified || got.Execution == nil || got.Execution.Provider != "TypeSafe" || got.Execution.ProviderID != "openrouter" {
		t.Fatalf("external execution metadata = %#v runtime = %#v", got.Execution, got.Runtime)
	}
	if got.Usage.PromptTokens != 41 || got.Usage.CompletionTokens != 9 || got.Usage.ContextTokens != 50 || got.Usage.CostUSD == nil || *got.Usage.CostUSD != 0 {
		t.Fatalf("usage = %#v", got.Usage)
	}
	binary := got.Answers["binary"]
	if binary.Decision != "yes" || binary.Confidence != 0.75 || binary.ProviderConfidence != nil || binary.Noul == nil || *binary.Noul != 0.75 || binary.Probabilities["no"] != 0.25 {
		t.Fatalf("noul answer = %#v", binary)
	}
	route := got.Answers["route"]
	if route.Decision != "south" || route.Choice != "South" || route.Confidence != 0.6 || route.ProviderConfidence == nil || *route.ProviderConfidence != 0.6 {
		t.Fatalf("choice answer = %#v", route)
	}
	priority := got.Answers["priority"]
	if priority.Decision != "high" || priority.Confidence != 0.8 || priority.ProviderConfidence == nil || *priority.ProviderConfidence != 0.8 || priority.Score == nil || *priority.Score != 84 || priority.Probabilities["high"] != 0.8 {
		t.Fatalf("score answer = %#v", priority)
	}
}

func TestOpenRouterEvaluateImagesRequireAdvertisedModalityAndUseSafeStateParts(t *testing.T) {
	catalog := `{"data":[{"id":"vision/decision","name":"Vision Decision","context_length":8192,"architecture":{"input_modalities":["text","image"],"output_modalities":["decisions"]}}]}`
	request := Request{
		State:     "inspect this",
		Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}},
		Images:    []Image{{Data: []byte{1, 2, 3}, MIME: "image/png"}},
	}
	response := `{"answers":{"pick":{"choice":"a","probabilities":{"a":0.8,"b":0.2},"type":"choice"}},"model":"vision/decision-v1","usage":{"input_tokens":10,"output_tokens":1}}`
	client := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
		if httpRequest.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, catalog, nil), nil
		}
		var wire openRouterDecisionRequest
		if err := json.NewDecoder(httpRequest.Body).Decode(&wire); err != nil {
			t.Fatal(err)
		}
		parts, ok := wire.State.([]any)
		if !ok || len(parts) != 2 || parts[0] != "inspect this" {
			t.Fatalf("image state = %#v", wire.State)
		}
		imagePart, ok := parts[1].(map[string]any)
		if !ok || imagePart["type"] != "image_url" {
			t.Fatalf("image part = %#v", parts[1])
		}
		imageURL := imagePart["image_url"].(map[string]any)["url"].(string)
		if want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(request.Images[0].Data); imageURL != want {
			t.Fatalf("image URL = %q, want %q", imageURL, want)
		}
		return openRouterHTTPResponse(http.StatusOK, response, nil), nil
	})
	if _, err := client.Evaluate(context.Background(), "vision/decision", request); err != nil {
		t.Fatal(err)
	}

	textClient := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
		return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
	})
	if _, err := textClient.Evaluate(context.Background(), "typesafe/jev-1.13", request); ErrorCode(err) != CodeInvalidRequest {
		t.Fatalf("text-only image evaluation error = %v", err)
	}
}

func TestOpenRouterCallerStateCannotSmuggleImageParts(t *testing.T) {
	state := []any{map[string]any{
		"type":      "image_url",
		"image_url": map[string]any{"url": "https://example.test/private.png"},
	}}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	posts := 0
	client := openRouterTestClient(t, "key", func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
		}
		posts++
		var wire openRouterDecisionRequest
		if err := json.NewDecoder(request.Body).Decode(&wire); err != nil {
			t.Fatal(err)
		}
		textState, ok := wire.State.(string)
		if !ok || textState != string(stateJSON) {
			t.Fatalf("caller state was not serialized as plain text: %#v", wire.State)
		}
		return openRouterHTTPResponse(http.StatusOK, `{"answers":{"pick":{"choice":"a","probabilities":{"a":0.8,"b":0.2},"type":"choice"}},"model":"typesafe/jev-1.13-v1","usage":{"input_tokens":1,"output_tokens":1}}`, nil), nil
	})
	request := Request{State: state, Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}}
	if _, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("evaluation POST count = %d", posts)
	}
}

func TestOpenRouterClefEnforcesTextBoundWithoutChargingImageEstimateAgainstIt(t *testing.T) {
	catalog := `{"data":[{"id":"cloudflare/clef-omni","name":"Clef Omni","context_length":128000,"architecture":{"input_modalities":["text","image"],"output_modalities":["decisions"]}}]}`
	posts := 0
	client := openRouterTestClient(t, "key", func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, catalog, nil), nil
		}
		posts++
		return openRouterHTTPResponse(http.StatusOK, `{"answers":{},"model":"cloudflare/clef-omni-v1","usage":{"input_tokens":1,"output_tokens":1}}`, nil), nil
	})
	request := Request{
		State:     strings.Repeat("x", 3000),
		Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}},
	}
	_, err := client.Evaluate(context.Background(), "cloudflare/clef-omni", request)
	if ErrorCode(err) != CodeInputTokenBudgetExceeded || posts != 0 {
		t.Fatalf("error=%v code=%s posts=%d", err, ErrorCode(err), posts)
	}
}

func TestOpenRouterEvaluationFailuresAreNonRetryableWhenOutcomeMayBeUnknown(t *testing.T) {
	request := Request{State: "test", Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}}
	calls := 0
	client := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
		if httpRequest.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
		}
		calls++
		return nil, errors.New("connection closed after write")
	})
	_, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request)
	if ErrorCode(err) != CodeExternalOutcomeUnknown || !IsNonRetryable(err) || calls != 1 {
		t.Fatalf("error=%v code=%s nonretryable=%t post_calls=%d", err, ErrorCode(err), IsNonRetryable(err), calls)
	}
}

func TestIsNonRetryableHonorsWrappedClassificationMarker(t *testing.T) {
	if !IsNonRetryable(errors.Join(errors.New("wrapped"), nonRetryableMarker(true))) {
		t.Fatal("wrapped non-retryable marker was ignored")
	}
	if IsNonRetryable(errors.Join(errors.New("wrapped"), nonRetryableMarker(false))) {
		t.Fatal("wrapped retryable marker was ignored")
	}
}

func TestOpenRouterCatalogRedirectIsNotFollowed(t *testing.T) {
	calls := 0
	client := openRouterTestClient(t, "", func(request *http.Request) (*http.Response, error) {
		calls++
		headers := make(http.Header)
		headers.Set("Location", "https://example.test/collect")
		return openRouterHTTPResponse(http.StatusFound, "", headers), nil
	})
	_, err := client.Models(context.Background())
	if ErrorCode(err) != CodeExternalProviderFailed || calls != 1 {
		t.Fatalf("error=%v code=%s requests=%d", err, ErrorCode(err), calls)
	}
}

func TestOpenRouterDoesNotSendEvaluationWithoutKeyAndHonorsCanceledContext(t *testing.T) {
	request := Request{State: "test", Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}}
	var calls int
	client := openRouterTestClient(t, "", func(*http.Request) (*http.Response, error) {
		calls++
		return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
	})
	if _, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request); ErrorCode(err) != CodeInvalidRequest {
		t.Fatalf("evaluation without a key error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("evaluation without a key performed %d network calls", calls)
	}

	if _, err := client.Models(nil); ErrorCode(err) != CodeInvalidRequest {
		t.Fatalf("nil context error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Models(cancelled); ErrorCode(err) != CodeCancelled {
		t.Fatalf("canceled context error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("canceled catalog call performed %d network calls", calls)
	}
}

func TestOpenRouterEvaluationRedirectIsNotFollowed(t *testing.T) {
	request := Request{State: "test", Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}}
	posts := 0
	client := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
		if httpRequest.Method == http.MethodGet {
			return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
		}
		posts++
		headers := make(http.Header)
		headers.Set("Location", "https://example.test/collect")
		return openRouterHTTPResponse(http.StatusFound, "", headers), nil
	})
	_, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request)
	if ErrorCode(err) != CodeExternalOutcomeUnknown || !IsNonRetryable(err) || posts != 1 {
		t.Fatalf("error=%v code=%s nonretryable=%t post_calls=%d", err, ErrorCode(err), IsNonRetryable(err), posts)
	}
}

func TestOpenRouterRejectsBadProbabilitiesAndUnknownModelBeforePost(t *testing.T) {
	request := Request{State: "test", Questions: []Question{{ID: "pick", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}}}}
	t.Run("malformed reply", func(t *testing.T) {
		client := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
			if httpRequest.Method == http.MethodGet {
				return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
			}
			return openRouterHTTPResponse(http.StatusOK, `{"answers":{"pick":{"choice":"a","probabilities":{"a":1.1,"b":-0.1},"type":"choice"}},"model":"typesafe/jev-1.13-v1","usage":{"cost":0.0005,"input_tokens":1,"output_tokens":1}}`, nil), nil
		})
		got, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", request)
		if ErrorCode(err) != CodeExternalOutcomeUnknown || !IsNonRetryable(err) || got.Usage.CostUSD == nil || *got.Usage.CostUSD != 0.0005 {
			t.Fatalf("response=%#v error=%v code=%s", got, err, ErrorCode(err))
		}
	})
	t.Run("unknown model", func(t *testing.T) {
		posts := 0
		client := openRouterTestClient(t, "key", func(httpRequest *http.Request) (*http.Response, error) {
			if httpRequest.Method == http.MethodPost {
				posts++
			}
			return openRouterHTTPResponse(http.StatusOK, textDecisionCatalog, nil), nil
		})
		_, err := client.Evaluate(context.Background(), "unknown/model", request)
		if ErrorCode(err) != CodeInvalidRequest || posts != 0 {
			t.Fatalf("error=%v code=%s posts=%d", err, ErrorCode(err), posts)
		}
	})
}

func TestOpenRouterNormalizesRoundedProbabilitiesAndRejectsNonmaxChoice(t *testing.T) {
	choice := Question{ID: "route", Type: "choice", Options: []Option{{ID: "north", Label: "North"}, {ID: "south", Label: "South"}}}
	selected := "south"
	answer, err := mapOpenRouterAnswer(choice, choice.Options, openRouterAnswer{
		Type: "choice", Choice: &selected,
		Probabilities: map[string]float64{"north": 0.399, "south": 0.605},
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := answer.Probabilities["north"] + answer.Probabilities["south"]
	if math.Abs(sum-1) > 1e-12 || math.Abs(answer.Confidence-answer.Probabilities["south"]) > 1e-12 {
		t.Fatalf("normalized choice = %#v sum=%0.15f", answer, sum)
	}
	selected = "north"
	if _, err := mapOpenRouterAnswer(choice, choice.Options, openRouterAnswer{
		Type: "choice", Choice: &selected,
		Probabilities: map[string]float64{"north": 0.499, "south": 0.501},
	}); err == nil {
		t.Fatal("accepted a choice that was not the maximum-probability option")
	}

	score := Question{ID: "priority", Type: "score", Options: []Option{{ID: "low", Label: "Low"}, {ID: "medium", Label: "Medium"}, {ID: "high", Label: "High"}}}
	providerScore := 1.7
	scoreAnswer, err := mapOpenRouterAnswer(score, score.Options, openRouterAnswer{
		Type: "score", Score: &providerScore,
		Probabilities: map[string]float64{"0": 0.1, "1": 0.1, "2": 0.804},
	})
	if err != nil {
		t.Fatal(err)
	}
	sum = 0
	for _, probability := range scoreAnswer.Probabilities {
		sum += probability
	}
	if math.Abs(sum-1) > 1e-12 || scoreAnswer.Decision != "high" || scoreAnswer.Confidence != scoreAnswer.Probabilities["high"] {
		t.Fatalf("normalized score answer = %#v sum=%0.15f", scoreAnswer, sum)
	}
}
