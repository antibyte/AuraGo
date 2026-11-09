package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

const (
	relayTestToken = "relay-token-0123456789abcdef"
	relayTestChat  = storeLLMRelayPrefix + "/chat/completions"
	relayTestEmbed = storeLLMRelayPrefix + "/embeddings"
	relayTestModel = storeLLMRelayPrefix + "/models"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type relayFakeVerifier struct {
	token   string
	revoked atomic.Bool
	calls   atomic.Int32
	appID   atomic.Value
}

func (v *relayFakeVerifier) VerifyRelayToken(_ context.Context, appID, token string) bool {
	v.calls.Add(1)
	v.appID.Store(appID)
	return !v.revoked.Load() && appID == "deeptutor" && token == v.token
}

type relayFakeClient struct {
	mu       sync.Mutex
	requests []openai.ChatCompletionRequest
	calls    atomic.Int32
	response openai.ChatCompletionResponse
	err      error
	stream   func(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error)
	gate     chan struct{}
	entered  chan struct{}
}

func (c *relayFakeClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls.Add(1)
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	if c.entered != nil {
		c.entered <- struct{}{}
	}
	if c.gate != nil {
		<-c.gate
	}
	return c.response, c.err
}

func (c *relayFakeClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	c.calls.Add(1)
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	if c.stream == nil {
		return nil, errors.New("no fake stream")
	}
	return c.stream(ctx, req)
}

func (c *relayFakeClient) lastRequest(t *testing.T) openai.ChatCompletionRequest {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) == 0 {
		t.Fatal("the fake LLM client was never called")
	}
	return c.requests[len(c.requests)-1]
}

type relayFakeStream struct {
	ctx        context.Context
	chunks     []openai.ChatCompletionStreamResponse
	next       int
	endErr     error
	blockAfter bool
	closed     chan struct{}
	closeOnce  sync.Once
}

func newRelayFakeStream(ctx context.Context, chunks ...openai.ChatCompletionStreamResponse) *relayFakeStream {
	return &relayFakeStream{ctx: ctx, chunks: chunks, closed: make(chan struct{})}
}

func (s *relayFakeStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if s.next < len(s.chunks) {
		chunk := s.chunks[s.next]
		s.next++
		return chunk, nil
	}
	if s.blockAfter {
		<-s.ctx.Done()
		return openai.ChatCompletionStreamResponse{}, s.ctx.Err()
	}
	if s.endErr != nil {
		return openai.ChatCompletionStreamResponse{}, s.endErr
	}
	return openai.ChatCompletionStreamResponse{}, io.EOF
}

func (s *relayFakeStream) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

func relayDeltaChunk(content string) openai.ChatCompletionStreamResponse {
	return openai.ChatCompletionStreamResponse{
		ID: "chatcmpl-1", Object: "chat.completion.chunk", Model: "real-model-v1",
		Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: content}}},
	}
}

func newStoreRelayServer(client llm.ChatClient, mutate func(*config.Config)) *Server {
	cfg := &config.Config{}
	cfg.LLM.Model = "real-model-v1"
	cfg.LLM.ProviderType = "openai"
	if mutate != nil {
		mutate(cfg)
	}
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), LLMClient: client}
}

func newStoreRelayTestHandler(s *Server, v *relayFakeVerifier) *storeLLMRelay {
	h := newStoreLLMRelay(s, func(context.Context) (storeRelayTokenVerifier, error) { return v, nil })
	h.slots = make(chan struct{}, storeLLMRelayMaxConcurrent) // never share the process-wide semaphore in tests
	return h
}

func relayRequest(method, path, token, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func serveRelay(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type relayErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func decodeRelayError(t *testing.T, rec *httptest.ResponseRecorder) relayErrorBody {
	t.Helper()
	var body relayErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not an OpenAI-style error: %v\n%s", err, rec.Body.String())
	}
	return body
}

func parseRelaySSE(t *testing.T, body string) []string {
	t.Helper()
	var frames []string
	for _, block := range strings.Split(strings.TrimSpace(body), "\n\n") {
		if !strings.HasPrefix(block, "data: ") {
			t.Fatalf("not an SSE data frame: %q", block)
		}
		frames = append(frames, strings.TrimPrefix(block, "data: "))
	}
	return frames
}

func TestStoreLLMRelayConstantsMatchTheStoreContract(t *testing.T) {
	t.Parallel()
	if storeLLMRelayAppID != desktopstore.DeepTutorAppID || storeLLMRelayPrefix != desktopstore.DeepTutorRelayPath {
		t.Fatalf("relay constants (%q, %q) drifted from desktopstore (%q, %q)",
			storeLLMRelayAppID, storeLLMRelayPrefix, desktopstore.DeepTutorAppID, desktopstore.DeepTutorRelayPath)
	}
}

// ── authentication ───────────────────────────────────────────────────────────

func TestStoreLLMRelayRejectsMissingWrongAndRevokedToken(t *testing.T) {
	client := &relayFakeClient{response: openai.ChatCompletionResponse{}}
	s := newStoreRelayServer(client, nil)
	verifier := &relayFakeVerifier{token: relayTestToken}
	h := newStoreRelayTestHandler(s, verifier)

	chatBody := `{"messages":[{"role":"user","content":"hi"}]}`
	cases := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"no header", func(*http.Request) {}},
		{"wrong token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong-token") }},
		{"basic scheme", func(r *http.Request) { r.Header.Set("Authorization", "Basic "+relayTestToken) }},
		{"empty bearer", func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") }},
		{"two credentials", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+relayTestToken+" extra") }},
		{"oversized token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 4096)) }},
		{"query string token only", func(r *http.Request) {
			q := r.URL.Query()
			q.Set("api_key", relayTestToken)
			q.Set("token", relayTestToken)
			r.URL.RawQuery = q.Encode()
		}},
		{"cookie only", func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "aurago_session", Value: relayTestToken}) }},
	}
	var wantBody string
	for _, path := range []string{relayTestModel, relayTestChat, relayTestEmbed} {
		method := http.MethodPost
		if path == relayTestModel {
			method = http.MethodGet
		}
		for _, tc := range cases {
			req := relayRequest(method, path, "", chatBody)
			tc.mutate(req)
			rec := serveRelay(h, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s: status = %d, want 401", tc.name, path, rec.Code)
			}
			if got := decodeRelayError(t, rec).Error.Code; got != "invalid_api_key" {
				t.Fatalf("%s %s: code = %q, want invalid_api_key", tc.name, path, got)
			}
			if wantBody == "" {
				wantBody = rec.Body.String()
			} else if rec.Body.String() != wantBody {
				t.Fatalf("%s %s: 401 body differs between failure causes:\n%s\n%s", tc.name, path, rec.Body.String(), wantBody)
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatalf("%s %s: missing WWW-Authenticate", tc.name, path)
			}
		}
	}

	// A token that was valid becomes unusable once the verifier revokes it
	// (the app was uninstalled).
	if rec := serveRelay(h, relayRequest(http.MethodGet, relayTestModel, relayTestToken, "")); rec.Code != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200", rec.Code)
	}
	verifier.revoked.Store(true)
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, chatBody))
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != wantBody {
		t.Fatalf("revoked token: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got, _ := verifier.appID.Load().(string); got != "deeptutor" {
		t.Fatalf("verifier asked for app %q, want deeptutor", got)
	}

	// A Store service that cannot be resolved fails closed with the same 401.
	broken := newStoreLLMRelay(s, func(context.Context) (storeRelayTokenVerifier, error) { return nil, errors.New("desktop disabled") })
	rec = serveRelay(broken, relayRequest(http.MethodPost, relayTestChat, relayTestToken, chatBody))
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != wantBody {
		t.Fatalf("unresolved verifier: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if n := client.calls.Load(); n != 0 {
		t.Fatalf("the LLM client was called %d times for unauthenticated requests", n)
	}
}

func TestStoreLLMRelayRejectsWrongMethodsAndUnknownRoutes(t *testing.T) {
	client := &relayFakeClient{}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	for _, tc := range []struct{ method, path, allow string }{
		{http.MethodPost, relayTestModel, "GET"},
		{http.MethodDelete, relayTestModel, "GET"},
		{http.MethodGet, relayTestChat, "POST"},
		{http.MethodPut, relayTestChat, "POST"},
		{http.MethodGet, relayTestEmbed, "POST"},
		{http.MethodHead, relayTestEmbed, "POST"},
	} {
		rec := serveRelay(h, relayRequest(tc.method, tc.path, relayTestToken, `{}`))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != tc.allow {
			t.Fatalf("%s %s: status = %d allow = %q, want 405 %q", tc.method, tc.path, rec.Code, rec.Header().Get("Allow"), tc.allow)
		}
	}
	rec := serveRelay(h, relayRequest(http.MethodGet, storeLLMRelayPrefix+"/files", relayTestToken, ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", rec.Code)
	}
	if rec = serveRelay(h, relayRequest(http.MethodGet, storeLLMRelayPrefix+"/files", "", "")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown route without token status = %d, want 401", rec.Code)
	}
	if client.calls.Load() != 0 {
		t.Fatal("the LLM client must not run for rejected requests")
	}
}

func TestStoreLLMRelayListsOnlyTheAlias(t *testing.T) {
	h := newStoreRelayTestHandler(newStoreRelayServer(&relayFakeClient{}, nil), &relayFakeVerifier{token: relayTestToken})
	rec := serveRelay(h, relayRequest(http.MethodGet, relayTestModel, relayTestToken, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var list struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Object != "list" || len(list.Data) != 1 || list.Data[0].ID != "aurago" || list.Data[0].Object != "model" || list.Data[0].OwnedBy != "aurago" {
		t.Fatalf("unexpected model list: %s", rec.Body.String())
	}
}

// ── chat ─────────────────────────────────────────────────────────────────────

func TestStoreLLMRelayChatUsesConfiguredModelAndPassesRequestThrough(t *testing.T) {
	client := &relayFakeClient{response: openai.ChatCompletionResponse{
		ID: "chatcmpl-1", Model: "real-model-v1",
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: "hello"}, FinishReason: openai.FinishReasonStop}},
		Usage:   openai.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
	}}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	body := `{
		"model": "gpt-evil-override",
		"messages": [{"role":"system","content":"be brief"},{"role":"user","content":"hi"}],
		"stop": "END",
		"temperature": 0.3,
		"max_tokens": 77,
		"store": true,
		"metadata": {"k":"v"},
		"some_future_field": {"nested": true},
		"tools": [{"type":"function","function":{"name":"lookup","description":"d","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}}],
		"tool_choice": {"type":"function","function":{"name":"lookup"}},
		"response_format": {"type":"json_schema","json_schema":{"name":"out","schema":{"type":"object"}}}
	}`
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := client.lastRequest(t)
	if got.Model != "real-model-v1" {
		t.Fatalf("upstream model = %q, want the configured AuraGo model", got.Model)
	}
	if len(got.Messages) != 2 || got.Messages[1].Content != "hi" {
		t.Fatalf("messages = %+v", got.Messages)
	}
	if len(got.Stop) != 1 || got.Stop[0] != "END" {
		t.Fatalf("scalar stop = %v, want [END]", got.Stop)
	}
	if got.Temperature != 0.3 || got.MaxTokens != 77 {
		t.Fatalf("temperature/max_tokens = %v/%d", got.Temperature, got.MaxTokens)
	}
	if got.Store || got.Metadata != nil {
		t.Fatalf("provider-side persistence fields must be dropped: store=%v metadata=%v", got.Store, got.Metadata)
	}
	if len(got.Tools) != 1 || got.Tools[0].Function == nil || got.Tools[0].Function.Name != "lookup" {
		t.Fatalf("tools = %+v", got.Tools)
	}
	choice, ok := got.ToolChoice.(map[string]any)
	if !ok || choice["type"] != "function" {
		t.Fatalf("tool_choice = %#v", got.ToolChoice)
	}
	if got.ResponseFormat == nil || got.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONSchema || got.ResponseFormat.JSONSchema == nil || got.ResponseFormat.JSONSchema.Name != "out" {
		t.Fatalf("response_format = %+v", got.ResponseFormat)
	}

	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Model != "aurago" || resp.Object != "chat.completion" || len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hello" || resp.Usage.TotalTokens != 4 {
		t.Fatalf("response = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "real-model-v1") {
		t.Fatal("the real model name must not reach the client")
	}
}

func TestStoreLLMRelayChatNeedsAConfiguredModel(t *testing.T) {
	for name, build := range map[string]func() *Server{
		"no model": func() *Server {
			return newStoreRelayServer(&relayFakeClient{}, func(c *config.Config) { c.LLM.Model = "  " })
		},
		"no client": func() *Server { return newStoreRelayServer(nil, nil) },
	} {
		rec := serveRelay(newStoreRelayTestHandler(build(), &relayFakeVerifier{token: relayTestToken}),
			relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"messages":[{"role":"user","content":"hi"}]}`))
		if rec.Code != http.StatusServiceUnavailable || decodeRelayError(t, rec).Error.Code != "llm_not_configured" {
			t.Fatalf("%s: status = %d body = %s", name, rec.Code, rec.Body.String())
		}
	}
	client := &relayFakeClient{}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	for _, body := range []string{`not json`, `{"messages":[]}`, `{}`} {
		if rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, body)); rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
	if client.calls.Load() != 0 {
		t.Fatal("invalid requests must not reach the LLM")
	}
}

func TestStoreLLMRelayChatErrorsHideUpstreamDetails(t *testing.T) {
	const secret = "sk-relay-test-secret-9f8e7d6c5b4a"
	security.RegisterSensitive(secret)
	cases := []struct {
		name     string
		err      error
		status   int
		code     string
		keepText string
	}{
		{"validation message is relayed but scrubbed", &openai.APIError{HTTPStatusCode: 400, Message: "tool_choice is not supported, see https://docs.provider.example/tools?key=" + secret + " " + secret},
			http.StatusBadRequest, "invalid_request", "tool_choice is not supported"},
		{"transport error", errors.New(`Post "https://api.provider.example/v1/chat/completions": dial tcp 10.0.0.9:443: connect: refused ` + secret),
			http.StatusBadGateway, "upstream_error", ""},
		{"auth error is not a client 401", &openai.APIError{HTTPStatusCode: 401, Message: "Incorrect API key provided: " + secret},
			http.StatusBadGateway, "upstream_auth_error", ""},
		{"rate limit", &openai.APIError{HTTPStatusCode: 429, Message: "slow down"}, http.StatusTooManyRequests, "rate_limit_exceeded", ""},
		{"timeout", context.DeadlineExceeded, http.StatusGatewayTimeout, "upstream_timeout", ""},
	}
	for _, tc := range cases {
		client := &relayFakeClient{err: tc.err}
		h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
		rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"messages":[{"role":"user","content":"hi"}]}`))
		if rec.Code != tc.status || decodeRelayError(t, rec).Error.Code != tc.code {
			t.Fatalf("%s: status = %d body = %s, want %d %s", tc.name, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
		for _, leak := range []string{secret, "provider.example", "10.0.0.9", "https://", "dial tcp"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Fatalf("%s: error body leaks %q: %s", tc.name, leak, rec.Body.String())
			}
		}
		if tc.keepText != "" && !strings.Contains(rec.Body.String(), tc.keepText) {
			t.Fatalf("%s: provider validation message was dropped: %s", tc.name, rec.Body.String())
		}
	}
}

// ── streaming ────────────────────────────────────────────────────────────────

func TestStoreLLMRelayStreamsOrderedFramesAndDone(t *testing.T) {
	usage := openai.ChatCompletionStreamResponse{ID: "chatcmpl-1", Model: "real-model-v1", Usage: &openai.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}}
	client := &relayFakeClient{}
	client.stream = func(ctx context.Context, _ openai.ChatCompletionRequest) (llm.CompletionStream, error) {
		return newRelayFakeStream(ctx, relayDeltaChunk("Hel"), relayDeltaChunk("lo"), relayDeltaChunk("!"), usage), nil
	}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken,
		`{"model":"x","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	for header, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
	req := client.lastRequest(t)
	if !req.Stream || req.Model != "real-model-v1" || req.StreamOptions == nil || !req.StreamOptions.IncludeUsage {
		t.Fatalf("upstream stream request = %+v", req)
	}
	frames := parseRelaySSE(t, rec.Body.String())
	if len(frames) != 5 || frames[4] != "[DONE]" {
		t.Fatalf("frames = %q", frames)
	}
	var text string
	for i, frame := range frames[:3] {
		var chunk openai.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(frame), &chunk); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if chunk.Model != "aurago" || len(chunk.Choices) != 1 {
			t.Fatalf("frame %d = %s", i, frame)
		}
		text += chunk.Choices[0].Delta.Content
	}
	if text != "Hello!" {
		t.Fatalf("assembled text = %q, frames out of order", text)
	}
	var final map[string]any
	if err := json.Unmarshal([]byte(frames[3]), &final); err != nil {
		t.Fatal(err)
	}
	choices, ok := final["choices"].([]any)
	if !ok || len(choices) != 0 {
		t.Fatalf("usage chunk must carry an empty choices array, got %#v", final["choices"])
	}
	if u, _ := final["usage"].(map[string]any); u["total_tokens"] != float64(7) || final["model"] != "aurago" {
		t.Fatalf("usage chunk = %s", frames[3])
	}
	if strings.Contains(rec.Body.String(), "real-model-v1") {
		t.Fatal("the real model name must not reach the client")
	}
}

func TestStoreLLMRelayStreamCancelsUpstreamWhenClientDisconnects(t *testing.T) {
	var upstream *relayFakeStream
	started := make(chan struct{})
	client := &relayFakeClient{}
	client.stream = func(ctx context.Context, _ openai.ChatCompletionRequest) (llm.CompletionStream, error) {
		upstream = newRelayFakeStream(ctx, relayDeltaChunk("first"))
		upstream.blockAfter = true
		close(started)
		return upstream, nil
	}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+relayTestChat, strings.NewReader(`{"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+relayTestToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "data: ") || !strings.Contains(line, `"first"`) {
		t.Fatalf("first frame = %q (%v)", line, err)
	}
	_ = resp.Body.Close() // the client walks away mid-stream

	select {
	case <-upstream.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the upstream stream was not closed after the client disconnected")
	}
	if upstream.ctx.Err() == nil {
		t.Fatal("the upstream context was not canceled after the client disconnected")
	}
}

func TestStoreLLMRelayStreamReportsMidStreamErrorsInBand(t *testing.T) {
	client := &relayFakeClient{}
	client.stream = func(ctx context.Context, _ openai.ChatCompletionRequest) (llm.CompletionStream, error) {
		s := newRelayFakeStream(ctx, relayDeltaChunk("partial"))
		s.endErr = &openai.APIError{HTTPStatusCode: 429, Message: "quota hit at https://billing.provider.example/acct"}
		return s, nil
	}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	frames := parseRelaySSE(t, rec.Body.String())
	if len(frames) != 3 || frames[2] != "[DONE]" || !strings.Contains(frames[0], "partial") {
		t.Fatalf("frames = %q", frames)
	}
	var errFrame relayErrorBody
	if err := json.Unmarshal([]byte(frames[1]), &errFrame); err != nil || errFrame.Error.Code != "rate_limit_exceeded" || errFrame.Error.Type != "rate_limit_error" {
		t.Fatalf("error frame = %s (%v)", frames[1], err)
	}
	if strings.Contains(rec.Body.String(), "provider.example") {
		t.Fatalf("error frame leaks the upstream URL: %s", rec.Body.String())
	}
}

func TestStoreLLMRelayStreamFailureBeforeFirstChunkKeepsHTTPStatus(t *testing.T) {
	client := &relayFakeClient{}
	client.stream = func(ctx context.Context, _ openai.ChatCompletionRequest) (llm.CompletionStream, error) {
		s := newRelayFakeStream(ctx)
		s.endErr = &openai.APIError{HTTPStatusCode: 429, Message: "rate limited"}
		return s, nil
	}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if rec.Code != http.StatusTooManyRequests || decodeRelayError(t, rec).Error.Code != "rate_limit_exceeded" {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal("a failure before the first chunk must be a plain JSON error")
	}

	client.stream = func(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
		return nil, errors.New(`Post "https://api.provider.example/v1/chat/completions": EOF`)
	}
	rec = serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "provider.example") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

// ── limits ───────────────────────────────────────────────────────────────────

func TestStoreLLMRelayRejectsOversizedBodiesWith413(t *testing.T) {
	client := &relayFakeClient{}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, func(c *config.Config) {
		c.Embeddings.Provider, c.Embeddings.BaseURL = "embed", "http://127.0.0.1:1/v1"
	}), &relayFakeVerifier{token: relayTestToken})
	big := strings.Repeat("a", storeLLMRelayMaxBodyBytes+1)
	for _, path := range []string{relayTestChat, relayTestEmbed} {
		// Declared length over the cap.
		rec := serveRelay(h, relayRequest(http.MethodPost, path, relayTestToken, big))
		if rec.Code != http.StatusRequestEntityTooLarge || decodeRelayError(t, rec).Error.Code != "request_too_large" {
			t.Fatalf("%s declared: status = %d body = %s", path, rec.Code, rec.Body.String())
		}
		// Unknown length (chunked): the reader enforces the cap.
		req := relayRequest(http.MethodPost, path, relayTestToken, "")
		req.Body = io.NopCloser(bytes.NewReader([]byte(big)))
		req.ContentLength = -1
		rec = serveRelay(h, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s chunked: status = %d body = %.200s", path, rec.Code, rec.Body.String())
		}
	}
	if client.calls.Load() != 0 {
		t.Fatal("oversized requests must not reach the LLM")
	}
}

func TestStoreLLMRelayCapsConcurrentCalls(t *testing.T) {
	client := &relayFakeClient{
		gate:     make(chan struct{}),
		entered:  make(chan struct{}, 8),
		response: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "ok"}}}},
	}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	h.slots = make(chan struct{}, 2)
	const chat = `{"messages":[{"role":"user","content":"hi"}]}`

	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, chat)).Code
		}()
	}
	for range codes {
		select {
		case <-client.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("the first calls never reached the client")
		}
	}
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, chat))
	if rec.Code != http.StatusTooManyRequests || decodeRelayError(t, rec).Error.Code != "rate_limit_exceeded" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over the cap: status = %d body = %s", rec.Code, rec.Body.String())
	}
	close(client.gate)
	wg.Wait()
	for _, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("admitted call status = %d, want 200", code)
		}
	}
	if rec = serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, chat)); rec.Code != http.StatusOK {
		t.Fatalf("after release: status = %d, the slot leaked", rec.Code)
	}
	if got := cap(storeLLMRelaySlots); got != 8 {
		t.Fatalf("process-wide cap = %d, want 8", got)
	}
}

// ── embeddings ───────────────────────────────────────────────────────────────

func TestStoreLLMRelayEmbeddingsAreUnavailableWithoutAnExternalProvider(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"empty":         func(c *config.Config) { c.Embeddings.Provider = "" },
		"disabled":      func(c *config.Config) { c.Embeddings.Provider = "disabled" },
		"local granite": func(c *config.Config) { c.Embeddings.Provider = "local-granite" },
		"no endpoint":   func(c *config.Config) { c.Embeddings.Provider = "embed" },
		"not http":      func(c *config.Config) { c.Embeddings.Provider, c.Embeddings.BaseURL = "embed", "ftp://example.test/v1" },
		"userinfo": func(c *config.Config) {
			c.Embeddings.Provider, c.Embeddings.BaseURL = "embed", "http://u:p@example.test/v1"
		},
	} {
		h := newStoreRelayTestHandler(newStoreRelayServer(&relayFakeClient{}, mutate), &relayFakeVerifier{token: relayTestToken})
		rec := serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, `{"input":"x"}`))
		if rec.Code != http.StatusNotFound || decodeRelayError(t, rec).Error.Code != "embeddings_unavailable" {
			t.Fatalf("%s: status = %d body = %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestStoreLLMRelayEmbeddingTargetFollowsTheMemoryResolution(t *testing.T) {
	cfg := func(mutate func(*config.Config)) *config.Config {
		c := &config.Config{}
		mutate(c)
		return c
	}
	cases := []struct {
		name         string
		cfg          *config.Config
		wantOK       bool
		wantEndpoint string
		wantKey      string
		wantModel    string
	}{
		{"nil", nil, false, "", "", ""},
		{"provider entry", cfg(func(c *config.Config) {
			c.Embeddings.Provider, c.Embeddings.BaseURL, c.Embeddings.APIKey, c.Embeddings.Model = "emb", "https://api.example.test/v1/", "k1", "m1"
		}), true, "https://api.example.test/v1/embeddings", "k1", "m1"},
		{"default model", cfg(func(c *config.Config) {
			c.Embeddings.Provider, c.Embeddings.BaseURL = "emb", "http://127.0.0.1:11435/v1"
		}),
			true, "http://127.0.0.1:11435/v1/embeddings", "", "text-embedding-3-small"},
		{"internal uses the main LLM", cfg(func(c *config.Config) {
			c.Embeddings.Provider, c.Embeddings.InternalModel = "internal", "im"
			c.LLM.BaseURL, c.LLM.APIKey = "https://llm.example.test/v1", "main-key"
		}), true, "https://llm.example.test/v1/embeddings", "main-key", "im"},
		{"legacy external", cfg(func(c *config.Config) {
			c.Embeddings.Provider, c.Embeddings.ExternalURL, c.Embeddings.APIKey, c.Embeddings.ExternalModel = "external", "https://ext.example.test", "ek", "em"
		}), true, "https://ext.example.test/embeddings", "ek", "em"},
		{"disabled", cfg(func(c *config.Config) { c.Embeddings.Provider, c.Embeddings.BaseURL = "disabled", "https://x.test/v1" }), false, "", "", ""},
	}
	for _, tc := range cases {
		got, ok := storeRelayEmbeddingTargetFor(tc.cfg)
		if ok != tc.wantOK || got.Endpoint != tc.wantEndpoint || got.APIKey != tc.wantKey || got.Model != tc.wantModel {
			t.Fatalf("%s: got %+v ok=%v, want %s key=%q model=%q ok=%v", tc.name, got, ok, tc.wantEndpoint, tc.wantKey, tc.wantModel, tc.wantOK)
		}
		if storeRelayEmbeddingsAvailable(tc.cfg) != tc.wantOK {
			t.Fatalf("%s: storeRelayEmbeddingsAvailable disagrees", tc.name)
		}
	}
}

func TestStoreLLMRelayEmbeddingsForwardWithConfiguredModelKeyAndFixedURL(t *testing.T) {
	var (
		mu       sync.Mutex
		gotPath  string
		gotAuth  string
		gotBody  map[string]json.RawMessage
		attacker atomic.Int32
	)
	attackerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attacker.Add(1)
		_, _ = io.WriteString(w, `{"stolen":true}`)
	}))
	defer attackerSrv.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.Unmarshal(raw, &gotBody)
		mu.Unlock()
		switch {
		case strings.Contains(string(raw), "trigger-redirect"):
			http.Redirect(w, r, attackerSrv.URL+"/steal", http.StatusTemporaryRedirect)
		case strings.Contains(string(raw), "trigger-401"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":{"message":"bad key provider-secret-key-123 at http://internal.example/x"}}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"embed-model","usage":{"prompt_tokens":2,"total_tokens":2}}`)
		}
	}))
	defer upstream.Close()

	s := newStoreRelayServer(&relayFakeClient{}, func(c *config.Config) {
		c.Embeddings.Provider, c.Embeddings.ProviderType = "embed", "openai"
		c.Embeddings.BaseURL, c.Embeddings.APIKey, c.Embeddings.Model = upstream.URL+"/v1", "provider-secret-key-123", "embed-model"
	})
	h := newStoreRelayTestHandler(s, &relayFakeVerifier{token: relayTestToken})

	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken,
		`{"model":"evil-model","input":["a","b"],"encoding_format":"float","url":"`+attackerSrv.URL+`","base_url":"`+attackerSrv.URL+`"}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"embedding":[0.1,0.2]`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	path, auth, body := gotPath, gotAuth, gotBody
	mu.Unlock()
	if path != "/v1/embeddings" || auth != "Bearer provider-secret-key-123" {
		t.Fatalf("upstream saw path %q auth %q", path, auth)
	}
	if string(body["model"]) != `"embed-model"` || string(body["input"]) != `["a","b"]` || string(body["encoding_format"]) != `"float"` {
		t.Fatalf("upstream body = %v", body)
	}
	if attacker.Load() != 0 {
		t.Fatal("a body field steered the request to another host")
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("provider-secret-key-123")) {
		t.Fatal("the provider key reached the client")
	}

	// A provider redirect to another origin is not followed.
	rec = serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, `{"input":"trigger-redirect"}`))
	if rec.Code != http.StatusBadGateway || attacker.Load() != 0 || strings.Contains(rec.Body.String(), "stolen") {
		t.Fatalf("redirect: status = %d attacker hits = %d body = %s", rec.Code, attacker.Load(), rec.Body.String())
	}
	// A failing provider is mapped without echoing its body.
	rec = serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, `{"input":"trigger-401"}`))
	if rec.Code != http.StatusBadGateway || decodeRelayError(t, rec).Error.Code != "upstream_auth_error" ||
		strings.Contains(rec.Body.String(), "provider-secret-key-123") || strings.Contains(rec.Body.String(), "internal.example") {
		t.Fatalf("401 mapping: status = %d body = %s", rec.Code, rec.Body.String())
	}
	// input is required.
	if rec = serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, `{"model":"x"}`)); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing input: status = %d", rec.Code)
	}
}

// ── session bypass and Host guard ────────────────────────────────────────────

func newRelayRoutingServer(authEnabled bool) (*Server, http.Handler, *int) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Cfg.Auth.Enabled = authEnabled
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	reached := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusAccepted)
	})
	return s, authMiddleware(s, mux), &reached
}

func TestAuthBypassStoreLLMRelayOnlyForTheExactPrefix(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		storeLLMRelayPrefix + "/models",
		storeLLMRelayPrefix + "/chat/completions",
		storeLLMRelayPrefix + "/embeddings",
	} {
		if !isAuthBypassed(path) {
			t.Fatalf("%s should skip the session layer", path)
		}
	}
	for _, path := range []string{
		"/api/store-llm/other/v1/models",
		"/api/store-llm/deeptutor/../x",
		"/api/store-llm/deeptutor/v1/../../config",
		"/api/store-llm/deeptutor/v1/%2e%2e/x",
		"/api/store-llm/deeptutor/v1//models",
		"/api/store-llm/deeptutor/v1/models%5Cx",
		"/api/store-llm/deeptutor/v1/models\x00",
		"/api/store-llm/deeptutorX",
		"/api/store-llm/deeptutorX/v1/models",
		"/api/store-llm/deeptutor/v1",
		"/api/store-llm/deeptutor/",
		"/api/store-llm/deeptutor/v2/models",
		"/api/store-llm/",
		"/api/store-llm",
		"/API/store-llm/deeptutor/v1/models",
		"/x/api/store-llm/deeptutor/v1/models",
	} {
		if isAuthBypassed(path) {
			t.Fatalf("%s must require a session", path)
		}
	}
	for _, prefix := range authBypassSubtrees {
		if strings.Contains(prefix, "store-llm") && prefix != storeLLMRelayAuthPrefix {
			t.Fatalf("unexpected store-llm bypass %q", prefix)
		}
	}
}

func TestAuthBypassStoreLLMRelayThroughTheSessionMiddleware(t *testing.T) {
	_, handler, reached := newRelayRoutingServer(true)
	do := func(path string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.RemoteAddr = "198.51.100.9:443"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := do(storeLLMRelayPrefix + "/models"); code != http.StatusAccepted || *reached != 1 {
		t.Fatalf("relay path without session: status = %d reached = %d, want the handler", code, *reached)
	}
	for _, path := range []string{
		"/api/store-llm/other/v1/models", "/api/store-llm/deeptutor/../x", "/api/store-llm/deeptutorX", "/api/store-llm/deeptutor/v1/../config", "/api/config",
	} {
		if code := do(path); code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", path, code)
		}
	}
	if *reached != 1 {
		t.Fatalf("the bypass reached %d handlers, want only the relay path", *reached)
	}
}

func TestStoreLLMRelayThroughTheFullStackAuthenticatesItself(t *testing.T) {
	client := &relayFakeClient{}
	s, _, _ := newRelayRoutingServer(true)
	s.LLMClient = client
	s.Cfg.LLM.Model = "real-model-v1"
	mux := http.NewServeMux()
	mux.Handle(storeLLMRelayAuthPrefix, newStoreRelayTestHandler(s, &relayFakeVerifier{token: relayTestToken}))
	handler := authMiddleware(s, mux)

	req := httptest.NewRequest(http.MethodGet, storeLLMRelayPrefix+"/models", nil)
	req.RemoteAddr = "198.51.100.9:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || decodeRelayError(t, rec).Error.Code != "invalid_api_key" {
		t.Fatalf("no token: status = %d body = %s (the relay itself must answer, not the session layer)", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, storeLLMRelayPrefix+"/models", nil)
	req.RemoteAddr = "198.51.100.9:443"
	req.Header.Set("Authorization", "Bearer "+relayTestToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with token: status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestRebindingStoreLLMRelayAcceptsTheDockerHostAliasOnlyOnRelayRoutes(t *testing.T) {
	_, handler, reached := newRelayRoutingServer(false)
	do := func(host, path string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Host = host
		req.RemoteAddr = "172.17.0.2:51234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, host := range []string{"host.docker.internal:8088", "host.docker.internal", "HOST.DOCKER.INTERNAL:8088"} {
		if code := do(host, storeLLMRelayPrefix+"/chat/completions"); code != http.StatusAccepted {
			t.Fatalf("Host %q on the relay: status = %d, want it accepted", host, code)
		}
	}
	if *reached != 3 {
		t.Fatalf("reached = %d, want 3", *reached)
	}
	for _, tc := range []struct{ host, path string }{
		{"host.docker.internal:8088", "/api/config"},
		{"host.docker.internal:8088", "/api/backup/create"},
		{"host.docker.internal:8088", "/v1/chat/completions"},
		{"host.docker.internal:8088", "/api/store-llm/deeptutor/v1/../../config"},
		{"host.docker.internal:8088", "/api/store-llm/deeptutor/v1//models"},
		{"host.docker.internal:8088", "/api/store-llm/deeptutor/v1"},
		{"host.docker.internal:8088", "/api/store-llm/other/v1/models"},
		{"host.docker.internal.evil.example", storeLLMRelayPrefix + "/models"},
		{"evil.example", storeLLMRelayPrefix + "/models"},
		{"aurago.evil.example:8088", storeLLMRelayPrefix + "/chat/completions"},
	} {
		code := do(tc.host, tc.path)
		if code != http.StatusForbidden {
			t.Fatalf("Host %q path %s: status = %d, want 403 from the rebinding guard", tc.host, tc.path, code)
		}
	}
	if *reached != 3 {
		t.Fatalf("a rejected request reached a handler (%d)", *reached)
	}
	// Ordinary hosts keep working everywhere.
	if code := do("192.168.1.10:8088", "/api/config"); code != http.StatusAccepted {
		t.Fatalf("IP literal host: status = %d, want 202", code)
	}
}

// ── n, budget ────────────────────────────────────────────────────────────────

func TestStoreLLMRelayRejectsMoreThanOneChoice(t *testing.T) {
	client := &relayFakeClient{response: openai.ChatCompletionResponse{}}
	h := newStoreRelayTestHandler(newStoreRelayServer(client, nil), &relayFakeVerifier{token: relayTestToken})
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"n":3,"messages":[{"role":"user","content":"hi"}]}`))
	got := decodeRelayError(t, rec)
	if rec.Code != http.StatusBadRequest || got.Error.Type != "invalid_request_error" || got.Error.Message != "n must be 1" {
		t.Fatalf("n=3: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if client.calls.Load() != 0 {
		t.Fatal("n > 1 must not reach the LLM")
	}
	for _, body := range []string{`{"n":1,"messages":[{"role":"user","content":"hi"}]}`, `{"messages":[{"role":"user","content":"hi"}]}`} {
		if rec = serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, body)); rec.Code != http.StatusOK {
			t.Fatalf("body %s: status = %d", body, rec.Code)
		}
	}
}

func newRelayBudgetServer(t *testing.T, upstreamURL string) (*Server, *budget.Tracker) {
	t.Helper()
	s := newStoreRelayServer(&relayFakeClient{response: openai.ChatCompletionResponse{}}, func(c *config.Config) {
		c.Budget.Enabled, c.Budget.DailyLimitUSD, c.Budget.Enforcement, c.Budget.WarningThreshold = true, 1, "partial", 0.9
		c.Budget.Models = []config.ModelCost{{Name: "embed-model", InputPerMillion: 2}}
		c.Embeddings.Provider, c.Embeddings.ProviderType, c.Embeddings.Model = "embed", "openai", "embed-model"
		c.Embeddings.BaseURL = upstreamURL + "/v1"
	})
	tracker := budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	if tracker == nil {
		t.Fatal("budget tracker was not created")
	}
	s.BudgetTracker = tracker
	return s, tracker
}

func TestStoreLLMRelayEmbeddingsHonourTheBudget(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.5]}],"model":"embed-model","usage":{"prompt_tokens":500000,"total_tokens":500000}}`)
	}))
	defer upstream.Close()
	s, tracker := newRelayBudgetServer(t, upstream.URL)
	h := newStoreRelayTestHandler(s, &relayFakeVerifier{token: relayTestToken})
	const body = `{"input":"hello"}`

	// Usage reported by the provider is booked under the relay category.
	rec := serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if spent := tracker.CategorySpendUSD("store_relay"); spent < 0.99 || spent > 1.01 {
		t.Fatalf("store_relay spend = %v, want about 1.0 (500000 tokens at 2 USD per million)", spent)
	}

	// The budget is now exceeded: the relay category is blocked before any upstream call.
	if !tracker.IsBlocked("store_relay") {
		t.Fatal("test setup: the budget should be exceeded and block the relay category")
	}
	before := upstreamCalls.Load()
	rec = serveRelay(h, relayRequest(http.MethodPost, relayTestEmbed, relayTestToken, body))
	if rec.Code != http.StatusTooManyRequests || decodeRelayError(t, rec).Error.Code != "budget_exceeded" {
		t.Fatalf("blocked embeddings: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if upstreamCalls.Load() != before {
		t.Fatal("a blocked embeddings request reached the provider")
	}
	// Chat shares the gate.
	rec = serveRelay(h, relayRequest(http.MethodPost, relayTestChat, relayTestToken, `{"messages":[{"role":"user","content":"hi"}]}`))
	if rec.Code != http.StatusTooManyRequests || decodeRelayError(t, rec).Error.Code != "budget_exceeded" {
		t.Fatalf("blocked chat: status = %d body = %s", rec.Code, rec.Body.String())
	}
}

// ── login requirement ────────────────────────────────────────────────────────

func TestAuthBypassStoreLLMRelayLeavesEveryOtherRouteClosedForContainerHosts(t *testing.T) {
	s, _, _ := newRelayRoutingServer(true) // login enabled with a password hash
	reached := 0
	mux := http.NewServeMux()
	mux.Handle(storeLLMRelayAuthPrefix, newStoreRelayTestHandler(s, &relayFakeVerifier{token: relayTestToken}))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusAccepted)
	})
	handler := authMiddleware(s, mux)
	do := func(host, method, path, token string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Host = host
		req.RemoteAddr = "172.17.0.2:51234"
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, host := range []string{"127.0.0.1:8088", "localhost:8088", "host.docker.internal:8088"} {
		for _, path := range []string{"/api/config", "/api/desktop/store/apps"} {
			if code := do(host, http.MethodGet, path, ""); code != http.StatusUnauthorized {
				t.Fatalf("Host %q %s without a session: status = %d, want 401", host, path, code)
			}
			// The Store token opens the relay only, never another route.
			if code := do(host, http.MethodGet, path, relayTestToken); code != http.StatusUnauthorized && code != http.StatusForbidden {
				t.Fatalf("Host %q %s with the Store token: status = %d, want it refused", host, path, code)
			}
		}
		if code := do(host, http.MethodGet, relayTestModel, relayTestToken); code != http.StatusOK {
			t.Fatalf("Host %q relay with a valid token: status = %d, want 200", host, code)
		}
		if code := do(host, http.MethodGet, relayTestModel, ""); code != http.StatusUnauthorized {
			t.Fatalf("Host %q relay without a token: status = %d, want 401", host, code)
		}
	}
	if reached != 0 {
		t.Fatalf("%d requests reached a non-relay handler without a session", reached)
	}
}
