package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnthropicNativeThinkingSurvivesSDKContinuation(t *testing.T) {
	raw := `{"id":"msg_fixture","model":"claude-sonnet-4-5","content":[{"type":"thinking","thinking":"real API string","signature":"opaque-signature","future":{"preserve":true}},{"type":"redacted_thinking","data":"encrypted"},{"type":"tool_use","id":"call_1","name":"test","input":{}}],"stop_reason":"tool_use"}`
	var ant anthropicResponse
	if err := json.Unmarshal([]byte(raw), &ant); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(mapAnthropicToOpenAI(ant))
	var sdk openai.ChatCompletionResponse
	if err := json.Unmarshal(encoded, &sdk); err != nil {
		t.Fatal(err)
	}
	msg := sdk.Choices[0].Message
	if got := ReasoningText(msg.ReasoningContent); got != "real API string" {
		t.Fatalf("display = %q", got)
	}
	wire, _ := json.Marshal(msg)
	_, messages, err := translateMessages([]json.RawMessage{wire})
	if err != nil {
		t.Fatal(err)
	}
	blocks := messages[1].Content.([]anthropicContentBlock)
	for i := range 2 {
		got, _ := json.Marshal(blocks[i])
		var a, b any
		_ = json.Unmarshal(ant.Content[i].native, &a)
		_ = json.Unmarshal(got, &b)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("native block changed: %s", got)
		}
	}
}
func TestAnthropicStreamRetainsThinkingSignatureAndRedactedBlocks(t *testing.T) {
	events := []string{
		`event: message_start` + "\n" + `data: {"message":{"id":"msg_fixture","model":"claude-sonnet-4-5"}}`,
		`event: content_block_start` + "\n" + `data: {"index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`event: content_block_delta` + "\n" + `data: {"index":0,"delta":{"type":"thinking_delta","thinking":"live thought"}}`,
		`event: content_block_delta` + "\n" + `data: {"index":0,"delta":{"type":"signature_delta","signature":"sig-a"}}`,
		`event: content_block_delta` + "\n" + `data: {"index":0,"delta":{"type":"signature_delta","signature":"sig-b"}}`,
		`event: content_block_stop` + "\n" + `data: {"index":0}`,
		`event: content_block_start` + "\n" + `data: {"index":1,"content_block":{"type":"redacted_thinking","data":"opaque"}}`,
		`event: message_delta` + "\n" + `data: {"delta":{"stop_reason":"tool_use"}}`,
		`event: message_stop` + "\n" + `data: {}`,
	}
	var out bytes.Buffer
	var live string
	if err := translateStreamEvents(strings.NewReader(strings.Join(events, "\n\n")+"\n\n"), &out, "fixture", func(content, state string) {
		if state == "delta" {
			live += content
		}
	}); err != nil {
		t.Fatal(err)
	}
	var reasoning string
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "data: {") {
			continue
		}
		var chunk openai.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			t.Fatal(err)
		}
		for _, choice := range chunk.Choices {
			reasoning += choice.Delta.ReasoningContent
		}
	}
	blocks := decodeAnthropicReasoning(reasoning)
	if len(blocks) != 2 || live != "live thought" || ReasoningText(reasoning) != live {
		t.Fatalf("lost reasoning: %s", out.String())
	}
	if !strings.Contains(string(blocks[0]), `"signature":"sig-asig-b"`) || !strings.Contains(string(blocks[1]), `"data":"opaque"`) {
		t.Fatalf("lost signatures: %s", reasoning)
	}
}
func TestAnthropicStableCachePrefixAndThinkingPolicy(t *testing.T) {
	a := cacheableAnthropicSystem("stable\n# TURN CONTEXT\ntime A")
	b := cacheableAnthropicSystem("stable\n# TURN CONTEXT\ntime B")
	if len(a) != 2 || a[0].Text != b[0].Text || a[0].CacheControl == nil || a[1].CacheControl != nil || a[1].Text == b[1].Text {
		t.Fatal("unstable cache boundary")
	}
	temp, top := float32(.2), float32(.3)
	for _, test := range []struct {
		model  string
		cap    int
		choice any
		kind   string
	}{
		{"claude-sonnet-4-5", 1024, "auto", ""}, {"claude-sonnet-4-5", 1025, "auto", "enabled"}, {"claude-sonnet-4-5", 8192, "required", ""}, {"claude-sonnet-4-6", 8192, "required", "adaptive"}, {"claude-opus-4-7", 8192, "auto", "adaptive"},
	} {
		ant, err := translateOpenAIToAnthropic(openaiRequest{Model: test.model, MaxTokens: test.cap, Temperature: &temp, TopP: &top, ToolChoice: test.choice}, anthropicThinkingConfig{Enabled: true, BudgetTokens: 1})
		if err != nil {
			t.Fatal(err)
		}
		if test.kind == "" {
			if ant.Thinking != nil {
				t.Fatalf("unexpected thinking: %+v", ant)
			}
		} else {
			if ant.Thinking == nil || ant.Thinking.Type != test.kind || ant.Temperature != nil || ant.TopP != nil {
				t.Fatalf("bad policy: %+v", ant)
			}
			if test.kind == "enabled" && ant.Thinking.BudgetTokens != 1024 {
				t.Fatal("budget below API minimum")
			}
		}
	}
}
func TestAttemptDeadlineCountsTowardsFailoverButParentCancellationDoesNot(t *testing.T) {
	parent := context.Background()
	attempt, cancel := context.WithDeadline(parent, time.Now().Add(-time.Second))
	defer cancel()
	fm := &FailoverManager{primary: failoverTestClient(failoverRoundTripper(func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })), fallback: openai.NewClient("fixture"), primaryModel: "fixture", errorThreshold: 1, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	_, _ = fm.CreateChatCompletion(withAttemptParent(attempt, parent), openai.ChatCompletionRequest{})
	if !fm.isOnFallback {
		t.Fatal("attempt timeout did not trip failover")
	}
	fm.isOnFallback = false
	fm.errorCount = 0
	cancelled, cancelParent := context.WithCancel(parent)
	cancelParent()
	_, _ = fm.CreateChatCompletion(withAttemptParent(cancelled, cancelled), openai.ChatCompletionRequest{})
	if fm.isOnFallback || fm.errorCount != 0 {
		t.Fatal("parent cancellation changed health")
	}
}
func TestExactRegistryPricingPrecedesFamilyFallback(t *testing.T) {
	key := "openai/gpt-4o-fixture-audit"
	old, existed := KnownModelRegistry[key]
	KnownModelRegistry[key] = ModelRegistryEntry{ID: "gpt-4o-fixture-audit", Provider: "openai", InputPricePer1M: 123, OutputPricePer1M: 456}
	defer func() {
		if existed {
			KnownModelRegistry[key] = old
		} else {
			delete(KnownModelRegistry, key)
		}
	}()
	price, ok := StaticPricingForModel("openai", "gpt-4o-fixture-audit")
	if !ok || price.InputPerMillion != 123 {
		t.Fatalf("wrong price: %+v", price)
	}
}

func TestRecoveryProbeRequiresGenerationAndBooksOnce(t *testing.T) {
	for _, test := range []struct {
		name                 string
		metadata, generation int
		wantCalls            int
		wantSuccess          bool
	}{{"healthy", 200, 200, 1, true}, {"broken_generation", 200, 503, 1, false}, {"unauthorized", 401, 200, 0, false}, {"rate_limited", 429, 200, 0, false}} {
		t.Run(test.name, func(t *testing.T) {
			calls, booked := 0, 0
			client := failoverTestClient(failoverRoundTripper(func(r *http.Request) (*http.Response, error) {
				status := test.metadata
				body := `{"data":[{"id":"fixture"}]}`
				if r.Method == http.MethodPost {
					calls++
					status = test.generation
					data, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(data), `"max_tokens":1`) {
						t.Fatal("probe output unbounded")
					}
					body = `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`
				}
				if status >= 400 {
					body = `{"error":{"message":"fixture failure"}}`
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			err := probePrimaryGeneration(context.Background(), failoverProbeSnapshot{primaryClient: client, primaryType: "openai", primaryModel: "fixture", recorder: func(ModelRoute, *UsageCapture, openai.ChatCompletionResponse) { booked++ }})
			if (err == nil) != test.wantSuccess || calls != test.wantCalls || booked != test.wantCalls {
				t.Fatalf("err=%v generation=%d booked=%d", err, calls, booked)
			}
		})
	}
}

func TestNativeRecoveryMetadataAuthenticationAndRetryAfter(t *testing.T) {
	for _, fixture := range []struct {
		name, provider string
		status         int
	}{
		{"anthropic_healthy", "anthropic", http.StatusOK},
		{"anthropic_unauthorized", "anthropic", http.StatusUnauthorized},
		{"anthropic_rate_limited", "anthropic", http.StatusTooManyRequests},
		{"ollama_rate_limited", "ollama", http.StatusTooManyRequests},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var metadataCalls, generationCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					generationCalls.Add(1)
					_, _ = io.WriteString(w, `{"id":"fixture","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
					return
				}
				metadataCalls.Add(1)
				if fixture.provider == "anthropic" && (r.Header.Get("x-api-key") != "fixture-probe-key" || r.Header.Get("anthropic-version") != anthropicAPIVersion || r.Header.Get("Authorization") != "") {
					t.Error("Anthropic metadata request did not use native authentication/version headers")
				}
				w.Header().Set("Retry-After", "125")
				w.WriteHeader(fixture.status)
				_, _ = io.WriteString(w, `{"data":[],"error":{"message":"fixture metadata result"}}`)
			}))
			defer server.Close()
			snapshot := failoverProbeSnapshot{
				primaryClient: NewClientFromProvider(fixture.provider, server.URL+"/v1", "fixture-probe-key"),
				primaryType:   fixture.provider, primaryBaseURL: server.URL + "/v1", primaryAPIKey: "fixture-probe-key", primaryModel: "claude-sonnet-4-5",
			}
			err := probePrimaryGeneration(context.Background(), snapshot)
			wantGeneration := int32(0)
			if fixture.status == http.StatusOK {
				wantGeneration = 1
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("failed metadata admitted a generation probe")
			}
			if metadataCalls.Load() != 1 || generationCalls.Load() != wantGeneration {
				t.Fatalf("metadata=%d generation=%d, want 1/%d", metadataCalls.Load(), generationCalls.Load(), wantGeneration)
			}
			if fixture.status == http.StatusTooManyRequests && GetRetryAfter(err) != 125*time.Second {
				t.Fatalf("metadata lost Retry-After: %v", GetRetryAfter(err))
			}
		})
	}
}
