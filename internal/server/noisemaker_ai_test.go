package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type noisemakerRecordingChatClient struct {
	content string
	finish  openai.FinishReason
	calls   *int
	last    *openai.ChatCompletionRequest
}

func (c noisemakerRecordingChatClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if c.calls != nil {
		*c.calls++
	}
	if c.last != nil {
		*c.last = req
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: c.content}, FinishReason: c.finish}}}, nil
}

func (c noisemakerRecordingChatClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, fmt.Errorf("not implemented")
}

func noisemakerMiniMaxConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.ProviderType = "minimax"
	cfg.MusicGeneration.APIKey = "test-key"
	return cfg
}

func noisemakerLocalConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	cfg.ResolveProviders()
	return cfg
}

func noisemakerPost(t *testing.T, h http.HandlerFunc, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec, payload
}

func noisemakerPromptText(req openai.ChatCompletionRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

const noisemakerEnhancePath = "/api/desktop/noisemaker/enhance"

func TestNoisemakerLocalHelpersRunOnlyOnClick(t *testing.T) {
	calls := 0
	s := &Server{Cfg: noisemakerLocalConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse 1]\nla", calls: &calls}}
	rec := httptest.NewRecorder()
	handleNoisemakerState(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/state", nil))
	var state map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &state)
	guide, _ := state["lyrics_guide"].(map[string]any)
	if state["llm_available"] != true || guide["provider"] != "acestep" || guide["modifiers"] != true {
		t.Fatalf("state = %v", state)
	}
	rec, payload := noisemakerPost(t, handleNoisemakerEnhance(s), noisemakerEnhancePath, `{"kind":"lyrics","text":"Piano"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse 1]\nla" || calls != 1 {
		t.Fatalf("explicit helper in local mode: %d %v calls=%d", rec.Code, payload, calls)
	}
	rec, _ = noisemakerPost(t, handleNoisemakerGenerate(s), "/api/desktop/noisemaker/generate", `{"prompt":"a song","style":"pop"}`)
	if calls != 1 {
		t.Fatalf("local generation called the LLM (status %d)", rec.Code)
	}
}

func TestNoisemakerEnhancePromptsCarryProviderTags(t *testing.T) {
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse]\nla", last: &last}}
	h := handleNoisemakerEnhance(s)
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"lyrics","context":"rain"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	for _, want := range []string{"[Pre Chorus]", "[Build Up]", "3500", "No numbers"} {
		if !strings.Contains(noisemakerPromptText(last), want) {
			t.Fatalf("minimax lyrics prompt lacks %q", want)
		}
	}
	s.Cfg = noisemakerLocalConfig(t)
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"style","text":"synthwave"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(noisemakerPromptText(last), "Do not put BPM") {
		t.Fatal("acestep style prompt lacks the BPM rule")
	}
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"lyrics"}`); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(noisemakerPromptText(last), "[Chorus - anthemic]") {
		t.Fatal("acestep lyrics prompt lacks the modifier rule")
	}
}

func TestNoisemakerEnhanceRevise(t *testing.T) {
	calls := 0
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse]\nnew", calls: &calls, last: &last}}
	h := handleNoisemakerEnhance(s)
	for _, body := range []string{
		`{"kind":"revise","action":"shorten"}`,
		`{"kind":"revise","action":"bogus","text":"[Verse]\nla"}`,
		`{"kind":"revise","action":"custom","text":"[Verse]\nla"}`,
	} {
		if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, rec.Code)
		}
	}
	if calls != 0 {
		t.Fatal("invalid revise called the LLM")
	}
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"custom","instruction":"translate to English","text":"[Verse]\nla"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse]\nnew" || !strings.Contains(noisemakerPromptText(last), "translate to English") {
		t.Fatalf("custom revise: %d %v", rec.Code, payload)
	}
	rec, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"fix_tags","text":"[Verse]\nla la"}`)
	if rec.Code != http.StatusOK || payload["text"] != "[Verse]\nla la" || calls != 1 {
		t.Fatalf("clean fix_tags: %d %v calls=%d", rec.Code, payload, calls)
	}
	rec, _ = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"fix_tags","text":"[Verse 1]\nla la"}`)
	prompt := noisemakerPromptText(last)
	if rec.Code != http.StatusOK || calls != 2 || !strings.Contains(prompt, "Line 1: write [Verse] instead of [Verse 1].") {
		t.Fatalf("fix_tags prompt: %d calls=%d\n%s", rec.Code, calls, prompt)
	}
}

func TestNoisemakerEnhanceSongParsing(t *testing.T) {
	content := "Here you go:\n```json\n{\"idea\":\"A rainy night\",\"style\":\"lo-fi, mellow\",\"title\":\"Rain\",\"lyrics\":\"[Verse]\\nla\",\"bpm\":\"92\",\"vocal_language\":\"de\"}\n```"
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: content}}
	h := handleNoisemakerEnhance(s)
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song","text":"rain"}`)
	song, _ := payload["song"].(map[string]any)
	if rec.Code != http.StatusOK || song["idea"] != "A rainy night" || song["lyrics"] != "[Verse]\nla" || song["bpm"] != nil || song["vocal_language"] != nil {
		t.Fatalf("cloud song: %d %v", rec.Code, payload)
	}
	_, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song","instrumental":true}`)
	if song, _ := payload["song"].(map[string]any); song["lyrics"] != "" {
		t.Fatalf("instrumental kept lyrics: %v", payload)
	}
	s.Cfg = noisemakerLocalConfig(t)
	_, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song"}`)
	if song, _ := payload["song"].(map[string]any); song["bpm"] != float64(92) || song["vocal_language"] != "de" {
		t.Fatalf("local song: %v", payload)
	}
	s.LLMClient = noisemakerRecordingChatClient{content: "no json here"}
	rec, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"song"}`)
	if rec.Code != http.StatusBadGateway || payload["code"] != "ai_unreadable" {
		t.Fatalf("unreadable song: %d %v", rec.Code, payload)
	}
}

func TestNoisemakerEnhanceVariation(t *testing.T) {
	var last openai.ChatCompletionRequest
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: `{"idea":"B","style":"rock","title":"Part Two","lyrics":""}`, last: &last}}
	h := handleNoisemakerEnhance(s)
	for _, body := range []string{`{"kind":"variation","mode":"style"}`, `{"kind":"variation","mode":"remix","base":{"idea":"A"}}`} {
		if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, rec.Code)
		}
	}
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"variation","mode":"sequel","base":{"title":"Rain","idea":"A rainy night","style":"lo-fi","lyrics":"[Verse]\nla"}}`)
	prompt := noisemakerPromptText(last)
	if rec.Code != http.StatusOK || !strings.Contains(prompt, "Title: Rain") || !strings.Contains(prompt, "sequel") {
		t.Fatalf("variation: %d %v\n%s", rec.Code, payload, prompt)
	}
	if song, _ := payload["song"].(map[string]any); song["title"] != "Part Two" {
		t.Fatalf("variation song: %v", payload)
	}
}

func TestNoisemakerReviseNeverTruncatesSilently(t *testing.T) {
	calls := 0
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default(), LLMClient: noisemakerRecordingChatClient{content: "[Verse]\nnew", calls: &calls}}
	h := handleNoisemakerEnhance(s)
	body, _ := json.Marshal(map[string]any{"kind": "revise", "action": "shorten", "text": strings.Repeat("\u00e4", 8001)})
	if rec, _ := noisemakerPost(t, h, noisemakerEnhancePath, string(body)); rec.Code != http.StatusBadRequest || calls != 0 {
		t.Fatalf("too long: %d calls=%d", rec.Code, calls)
	}
	s.LLMClient = noisemakerRecordingChatClient{content: "[Verse]\ncut", finish: openai.FinishReasonLength}
	rec, payload := noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"shorten","text":"[Verse]\nla"}`)
	if rec.Code != http.StatusBadGateway || payload["code"] != "ai_unreadable" {
		t.Fatalf("cut off: %d %v", rec.Code, payload)
	}
	rec, payload = noisemakerPost(t, h, noisemakerEnhancePath, `{"kind":"revise","action":"fix_tags","text":"[Verse]\nla"}`)
	if rec.Code != http.StatusOK || payload["unchanged"] != true {
		t.Fatalf("unchanged: %d %v", rec.Code, payload)
	}
}

func TestNoisemakerTextHelpers(t *testing.T) {
	if got := limitNoisemakerText(strings.Repeat("\u00e4", 10), 3); got != "\u00e4\u00e4\u00e4" {
		t.Fatalf("limit = %q", got)
	}
	if got := stripNoisemakerFence("Here:\n```text\n[Verse]\nla\n```\nEnjoy"); got != "[Verse]\nla" {
		t.Fatalf("fence = %q", got)
	}
}

func TestNoisemakerLyricsCheck(t *testing.T) {
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default()}
	h := handleNoisemakerLyricsCheck(s)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/noisemaker/lyrics/check", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status %d", rec.Code)
	}
	rec, payload := noisemakerPost(t, h, "/api/desktop/noisemaker/lyrics/check", `{"lyrics":"[Verse 1]\nla"}`)
	issues, _ := payload["issues"].([]any)
	if rec.Code != http.StatusOK || payload["provider"] != "minimax" || payload["max_chars"] != float64(3500) || len(issues) != 1 {
		t.Fatalf("check: %d %v", rec.Code, payload)
	}
	if first, _ := issues[0].(map[string]any); first["code"] != "tag_numbered" || first["line"] != float64(1) {
		t.Fatalf("issue = %v", issues[0])
	}
	roCfg := noisemakerMiniMaxConfig(t)
	roCfg.VirtualDesktop.ReadOnly = true
	rec, _ = noisemakerPost(t, handleNoisemakerLyricsCheck(&Server{Cfg: roCfg, Logger: slog.Default()}), "/api/desktop/noisemaker/lyrics/check", `{"lyrics":"[Verse]\nla"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("read-only desktop: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNoisemakerGenerateRejectsLongMiniMaxLyrics(t *testing.T) {
	s := &Server{Cfg: noisemakerMiniMaxConfig(t), Logger: slog.Default()}
	body, _ := json.Marshal(map[string]any{"prompt": "pop", "lyrics": strings.Repeat("a", 3501)})
	rec, payload := noisemakerPost(t, handleNoisemakerGenerate(s), "/api/desktop/noisemaker/generate", string(body))
	if rec.Code != http.StatusBadRequest || payload["code"] != "invalid_lyrics_length" || !strings.Contains(fmt.Sprint(payload["message"]), "3500") {
		t.Fatalf("generate: %d %v", rec.Code, payload)
	}
}
