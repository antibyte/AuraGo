package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAnthropicThinkingModelPolicyMatrix(t *testing.T) {
	for _, model := range []struct {
		id                            string
		adaptive, defaultOn, alwaysOn bool
		fixed, forbidForced           bool
	}{
		{"claude-3-7-sonnet-20250219", false, false, false, false, false},
		{"claude-sonnet-4-20250514", false, false, false, false, false},
		{"claude-opus-4-1", false, false, false, false, false},
		{"claude-sonnet-4-5", false, false, false, false, false},
		{"claude-haiku-4-5", false, false, false, false, false},
		{"claude-opus-4-6", true, false, false, false, false},
		{"anthropic/claude-sonnet-4.6", true, false, false, false, false},
		{"claude-opus-4-7", true, false, false, true, false},
		{"claude-opus-4-8", true, false, false, true, false},
		{"claude-opus-5", true, true, false, true, false},
		{"claude-sonnet-5", true, true, false, true, false},
		{"claude-haiku-5-5", true, true, false, true, false},
		{"claude-opus-5-5", true, true, true, true, true},
		{"Anthropic/CLAUDE-SONNET-5.5-20261001", true, true, true, true, true},
		{"claude-fable-5", true, true, true, true, false},
		{"claude-fable-5-1", true, true, true, true, true},
		{"claude-mythos-5", true, true, true, true, false},
		{"claude-mythos-5-1", true, true, true, true, true},
		{"claude-mythos-preview", true, true, true, true, false},
	} {
		for _, enabled := range []bool{false, true} {
			for _, cap := range []int{1, 1024, 1025} {
				for _, choice := range []any{"auto", "none", "required", map[string]any{"type": "function", "function": map[string]any{"name": "fixture"}}} {
					t.Run(fmt.Sprintf("%s/enabled=%t/cap=%d/choice=%v", model.id, enabled, cap, choice), func(t *testing.T) {
						temp, top := float32(.2), float32(.3)
						forced := choice != "auto" && choice != "none"
						ant, err := translateOpenAIToAnthropic(openaiRequest{Model: model.id, MaxTokens: cap, ToolChoice: choice, Temperature: &temp, TopP: &top}, anthropicThinkingConfig{Enabled: enabled, BudgetTokens: 1})
						if forced && model.forbidForced || !enabled && model.alwaysOn {
							if err == nil {
								t.Fatal("incompatible thinking/tool policy was accepted")
							}
							return
						}
						if err != nil || ant.MaxTokens != cap {
							t.Fatalf("output cap changed or compatible policy rejected: cap=%d err=%v", ant.MaxTokens, err)
						}
						want := ""
						switch {
						case !enabled && model.defaultOn:
							want = "disabled"
						case enabled && model.adaptive:
							want = "adaptive"
						case enabled && cap > 1024 && !forced:
							want = "enabled"
						}
						got := ""
						if ant.Thinking != nil {
							got = ant.Thinking.Type
						}
						if got != want {
							t.Fatalf("thinking=%q, want %q", got, want)
						}
						if want == "enabled" && (ant.Thinking.BudgetTokens != 1024 || ant.Thinking.BudgetTokens >= cap) {
							t.Fatalf("invalid manual budget: %+v", ant.Thinking)
						}
						if want == "adaptive" || want == "disabled" {
							wire, _ := json.Marshal(ant.Thinking)
							if strings.Contains(string(wire), "budget_tokens") {
								t.Fatal("non-manual policy sent a manual budget")
							}
						}
						if model.fixed || want == "enabled" || want == "adaptive" {
							if ant.Temperature != nil || ant.TopP != nil {
								t.Fatal("incompatible sampling fields reached native request")
							}
						} else if ant.Temperature == nil || ant.TopP != nil {
							t.Fatal("valid single sampling control was lost")
						}
					})
				}
			}
		}
	}
}

func TestAnthropicThinkingAllowlistAndUnknownVersions(t *testing.T) {
	for _, test := range []struct {
		model string
		allow []string
		want  string
		err   bool
	}{
		{"claude-opus-5", []string{"claude-sonnet-4-6"}, "disabled", false},
		{"claude-opus-5-5", []string{"claude-sonnet-4-6"}, "", true},
		{"claude-opus-5-5", []string{"claude-opus-5*"}, "adaptive", false},
		{"fixture-compatible", []string{"fixture-*"}, "enabled", false},
		{"fixture-compatible", nil, "", false},
		{"claude-opus-5-6", nil, "", false},
		{"claude-opus-5-50", nil, "", false},
		{"claude-sonnet-4-60", nil, "", false},
		{"claude-3-5-sonnet", nil, "", false},
	} {
		ant, err := translateOpenAIToAnthropic(openaiRequest{Model: test.model, MaxTokens: 1025}, anthropicThinkingConfig{Enabled: true, ModelAllowlist: test.allow})
		got := ""
		if ant.Thinking != nil {
			got = ant.Thinking.Type
		}
		if (err != nil) != test.err || !test.err && got != test.want {
			t.Fatalf("allowlist/model %s/%v became %q: err=%v", test.model, test.allow, got, err)
		}
	}
}

func TestAnthropicThinkingRestrictionsFailBeforeNetwork(t *testing.T) {
	for _, test := range []struct {
		model  string
		cfg    anthropicThinkingConfig
		choice string
	}{
		{"claude-opus-5-5", anthropicThinkingConfig{}, "auto"},
		{"claude-fable-5-1", anthropicThinkingConfig{Enabled: true, ModelAllowlist: []string{"claude-sonnet-4-6"}}, "auto"},
		{"claude-sonnet-5-5", anthropicThinkingConfig{Enabled: true}, "required"},
	} {
		calls := 0
		transport := &anthropicTransport{thinkingCfg: test.cfg, base: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, nil })}
		req, _ := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":%q,"max_tokens":1,"tool_choice":%q}`, test.model, test.choice)))
		_, err := transport.RoundTrip(req)
		if err == nil || calls != 0 || ClassifyError(err) != ErrCategoryNonRetryableConfig {
			t.Fatalf("thinking policy escaped local rejection: calls=%d err=%v", calls, err)
		}
	}
}

func TestAnthropicDefaultOnThinkingIsExplicitlyDisabledOnNativeWire(t *testing.T) {
	called := false
	transport := &anthropicTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		var body anthropicRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Thinking == nil || body.Thinking.Type != "disabled" || body.MaxTokens != 1 || req.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("native off/cap/version changed: %+v version=%q", body, req.Header.Get("anthropic-version"))
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[],"stop_reason":"end_turn"}`))}, nil
	})}
	req, _ := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1/chat/completions", strings.NewReader(`{"model":"claude-opus-5","max_tokens":1}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !called {
		t.Fatal("compatible default-on model was blocked")
	}
}
