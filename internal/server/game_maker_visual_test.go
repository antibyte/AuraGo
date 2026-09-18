package server

import (
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"context"
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGameVisualRoutes(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Provider = "game"
	cfg.LLM.Model = "chosen-model"
	manual := false
	cfg.Providers = []config.ProviderEntry{{ID: "game", Type: "openai", Model: "provider-default"}, {ID: "vision", Type: "openai", Model: "vision-model"}}
	// Manual capabilities must win even for models absent from the catalog.
	for i := range cfg.Providers {
		cfg.Providers[i].Capabilities.Auto = &manual
		cfg.Providers[i].Capabilities.Multimodal = true
	}
	if p, reason := gameVisualRoute(cfg); p.Model != "chosen-model" || reason != "" {
		t.Fatalf("selected model ignored: %+v %s", p, reason)
	}
	cfg.Providers[0].Capabilities.Multimodal = false
	cfg.Vision.Provider = "vision"
	if p, reason := gameVisualRoute(cfg); p.ID != "vision" || reason != "" {
		t.Fatalf("vision fallback missing: %+v %s", p, reason)
	}
	cfg.Providers[1].Type = "agnes"
	if _, reason := gameVisualRoute(cfg); reason != "public_url_required" {
		t.Fatalf("private image public URL restriction ignored: %s", reason)
	}
	cfg.Vision.Provider = ""
	if _, reason := gameVisualRoute(cfg); reason != "no_vision_route" {
		t.Fatal(reason)
	}
}

func TestGameVisualProviderReceivesImagesWithoutTools(t *testing.T) {
	var received openai.ChatCompletionRequest
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": `{"findings":[{"image":0,"observation":"Two player sprites overlap","region":"left","severity":"defect","confidence":0.95,"suggestion":"Remove only the extra sprite"}]}`}}}})
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.Provider = "game"
	cfg.LLM.Model = "text-only"
	cfg.Vision.Provider = "vision"
	cfg.Agent.ContextWindow = 32768
	manual := false
	cfg.Providers = []config.ProviderEntry{{ID: "game", Type: "openai", Model: "text-only"}, {ID: "vision", Type: "openai", Model: "vision-test", BaseURL: provider.URL, APIKey: "local-test"}}
	for i := range cfg.Providers {
		cfg.Providers[i].Capabilities.Auto = &manual
	}
	cfg.Providers[1].Capabilities.Multimodal = true
	runner := &gameMakerAgentRunner{server: &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	result := gamemaker.BuildResult{Visual: gamemaker.VisualReview{BuildID: "current-build"}}
	err := runner.reviewGameImages(context.Background(), cfg, nil, gamemaker.JobRun{Result: &result, Captures: []gamemaker.VisualCapture{{Image: "data:image/png;base64,test", Scenario: "start", Width: 320, Height: 200}}})
	if err != nil || result.Visual.Status != "reviewed" || result.Visual.BuildID != "current-build" || len(result.Visual.RepairDiagnostics()) != 1 {
		t.Fatalf("review not propagated: %+v %v", result.Visual, err)
	}
	if received.Model != "vision-test" || len(received.Tools) != 0 {
		t.Fatalf("wrong provider/tools: %+v", received)
	}
	found := false
	for _, m := range received.Messages {
		for _, p := range m.MultiContent {
			found = found || p.Type == openai.ChatMessagePartTypeImageURL
		}
	}
	if !found {
		t.Fatal("image missing from provider request")
	}
}

func TestGameVisualResponseBounds(t *testing.T) {
	for _, bad := range []string{"", `{}`, `{"findings":[{"image":5,"observation":"x","severity":"defect","confidence":0.9}]}`, `{"findings":[{"image":0,"observation":"x","severity":"passed","confidence":0.9}]}`} {
		if _, err := decodeGameVisualReview(bad, 1); err == nil {
			t.Errorf("invalid response accepted: %s", bad)
		}
	}
	if f, err := decodeGameVisualReview("\x60\x60\x60json\n{\"findings\":[]}\n\x60\x60\x60", 1); err != nil || len(f) != 0 {
		t.Fatalf("valid fenced JSON rejected: %v", err)
	}
	if f, err := decodeGameVisualReview("<think>Inspect the frame.</think>\n```json\n{\"findings\":[]}\n```", 1); err != nil || len(f) != 0 {
		t.Fatalf("wrapped JSON rejected: %v", err)
	}
}

func TestGameVisualFormatRecovery(t *testing.T) {
	for _, test := range []struct {
		name, first, second, finish, status, reason string
		structured                                  bool
		calls                                       int
	}{
		{"supported", `{"findings":[]}`, "", "stop", "reviewed", "", true, 1},
		{"unsupported", `{"findings":[]}`, "", "stop", "reviewed", "", false, 1},
		{"corrected", "I see the truck.", `{"findings":[]}`, "stop", "reviewed", "", true, 2},
		{"bounded", "bad format", `{"findings":null}`, "stop", "failed", "invalid_response", true, 2},
		{"truncated", `{"findings":[]}`, "", "length", "failed", "analysis_failed", true, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				calls++
				if req.Model != "selected-multimodal-model" || len(req.Tools) != 0 || (req.ResponseFormat != nil) != test.structured {
					t.Errorf("wrong route, tools or unsupported JSON mode: model=%s tools=%d format=%+v", req.Model, len(req.Tools), req.ResponseFormat)
				}
				images := 0
				for _, m := range req.Messages {
					for _, part := range m.MultiContent {
						if part.Type == openai.ChatMessagePartTypeImageURL {
							images++
						}
						if strings.Contains(part.Text, "defect|suggestion|uncertain") {
							t.Error("schema example still contains an invalid severity literal")
						}
					}
				}
				if images != 1 {
					t.Errorf("image lost or duplicated on correction: %d", images)
				}
				content := test.first
				if calls > 1 {
					content = test.second
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "finish_reason": test.finish, "message": map[string]any{"role": "assistant", "content": content}}}})
			}))
			defer provider.Close()
			cfg := &config.Config{}
			cfg.LLM.Provider, cfg.LLM.Model = "game", "selected-multimodal-model"
			cfg.Agent.ContextWindow = 32768
			manual := false
			cfg.Providers = []config.ProviderEntry{{ID: "game", Type: "openai", Model: "different-provider-default", BaseURL: provider.URL, APIKey: "local-test"}}
			cfg.Providers[0].Capabilities.Auto = &manual
			cfg.Providers[0].Capabilities.Multimodal = true
			cfg.Providers[0].Capabilities.StructuredOutputs = test.structured
			runner := &gameMakerAgentRunner{server: &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
			result := gamemaker.BuildResult{}
			err := runner.reviewGameImages(context.Background(), cfg, nil, gamemaker.JobRun{Result: &result, Captures: []gamemaker.VisualCapture{{Image: "data:image/png;base64,test"}}})
			if err != nil || calls != test.calls || result.Visual.Status != test.status || result.Visual.Reason != test.reason {
				t.Fatalf("calls=%d visual=%+v err=%v", calls, result.Visual, err)
			}
		})
	}
}
