package chatrt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var testRequest = ChatRequest{
	Messages: []Message{{Role: "user", Content: "Hi"}},
	Sampling: Sampling{Temperature: 0.7, TopP: 0.8, TopK: 20, RepeatPenalty: 1.1, PresencePenalty: 1.5, MaxTokens: 64},
	Stop:     []string{"<|end|>"},
}

func TestBuildChatBody(t *testing.T) {
	raw, err := json.Marshal(buildChatBody(testRequest))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	kwargs, _ := body["chat_template_kwargs"].(map[string]any)
	opts, _ := body["stream_options"].(map[string]any)
	if body["model"] != "chibi" || body["stream"] != true || kwargs["enable_thinking"] != false || opts["include_usage"] != true ||
		body["max_tokens"] != float64(64) || body["top_k"] != float64(20) || body["temperature"] != 0.7 ||
		body["top_p"] != 0.8 || body["repeat_penalty"] != 1.1 || body["presence_penalty"] != 1.5 || body["cache_prompt"] != true {
		t.Fatalf("body %s", raw)
	}
	if stop, _ := body["stop"].([]any); len(stop) != 1 || stop[0] != "<|end|>" {
		t.Fatalf("stop %v", body["stop"])
	}
	if _, has := body["seed"]; has {
		t.Fatal("seed sent for 0")
	}
	seeded := testRequest
	seeded.Sampling.Seed = 42
	raw, _ = json.Marshal(buildChatBody(seeded))
	if !strings.Contains(string(raw), `"seed":42`) {
		t.Fatalf("seed missing: %s", raw)
	}
	noStop := testRequest
	noStop.Stop = nil
	raw, _ = json.Marshal(buildChatBody(noStop))
	if strings.Contains(string(raw), `"stop"`) {
		t.Fatalf("empty stop sent: %s", raw)
	}
}

func TestStreamChatRequest(t *testing.T) {
	var gotAuth, gotPath, gotAccept string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotAccept = r.Header.Get("Authorization"), r.URL.Path, r.Header.Get("Accept")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, readTestdata(t, "stream_basic.txt"))
	}))
	defer srv.Close()

	var text strings.Builder
	stats, err := streamChat(context.Background(), newLoopbackClient(), srv.URL, "secret-key", testRequest, func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-key" || gotPath != "/v1/chat/completions" || gotAccept != "text/event-stream" {
		t.Fatalf("auth %q path %q accept %q", gotAuth, gotPath, gotAccept)
	}
	if !strings.Contains(string(gotBody), `"enable_thinking":false`) {
		t.Fatalf("body %s", gotBody)
	}
	if text.String() != "Hello there, how are you?" || stats.FinishReason != "stop" || stats.PromptTokens != 18 || stats.CompletionTokens != 8 ||
		stats.TokensPerSecond <= 0 || stats.MS < 0 {
		t.Fatalf("text %q stats %+v", text.String(), stats)
	}
}

func TestStreamChatStatsFallbacks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	stats, err := streamChat(context.Background(), newLoopbackClient(), srv.URL, "k", testRequest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.FinishReason != "stop" || stats.CompletionTokens != 2 {
		t.Fatalf("stats %+v", stats)
	}
}

func TestStreamChatErrorAnswers(t *testing.T) {
	var redirected atomic.Bool
	var status atomic.Int32
	var body atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/elsewhere":
			redirected.Store(true)
		default:
			if status.Load() == http.StatusFound {
				http.Redirect(w, r, "/elsewhere", http.StatusFound)
				return
			}
			w.WriteHeader(int(status.Load()))
			_, _ = io.WriteString(w, body.Load().(string))
		}
	}))
	defer srv.Close()
	call := func(code int, answer string) error {
		status.Store(int32(code))
		body.Store(answer)
		_, err := streamChat(context.Background(), newLoopbackClient(), srv.URL, "k", testRequest, nil)
		return err
	}

	if err := call(http.StatusFound, ""); err == nil {
		t.Fatal("redirect treated as success")
	}
	if redirected.Load() {
		t.Fatal("the redirect was followed")
	}
	if err := call(400, readTestdata(t, "context_error.json")); !errors.Is(err, ErrContext) {
		t.Fatalf("context answer: %v", err)
	}
	if err := call(400, readTestdata(t, "context_error_stream.txt")); !errors.Is(err, ErrContext) {
		t.Fatalf("context answer of the streamed request: %v", err)
	}
	if err := call(400, `{"error":{"code":400,"message":"bad sampling value","type":"invalid_request_error"}}`); err == nil || errors.Is(err, ErrContext) {
		t.Fatalf("other 400: %v", err)
	}
	if err := call(500, "<html>boom</html>"); err == nil || errors.Is(err, ErrContext) || !errors.Is(err, errUpstream) {
		t.Fatalf("500: %v", err)
	}
	// The error body is read up to 16 KiB: a context message after that point is not seen.
	padded := `{"error":{"code":400,"message":"` + strings.Repeat("x", maxErrorBody) + ` exceeds the context size","type":"x"}}`
	if err := call(400, padded); errors.Is(err, ErrContext) {
		t.Fatal("error body above 16 KiB was read")
	}
}

func TestStreamChatCancelClosesUpstream(t *testing.T) {
	started := make(chan struct{})
	upstreamEnded := make(chan struct{})
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // the server notices a closed connection only once the body is read
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n")
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-r.Context().Done():
			close(upstreamEnded)
		case <-stop:
		}
	}))
	defer srv.Close()
	defer close(stop)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := streamChat(ctx, newLoopbackClient(), srv.URL, "k", testRequest, nil)
		errc <- err
	}()
	waitClosed(t, started, "first delta")
	cancel()
	select {
	case <-upstreamEnded:
	case <-time.After(2 * time.Second):
		t.Fatal("the upstream request context did not end after the client cancel")
	}
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

// llama-server closes idle connections on its own schedule; a POST on a reused connection it just closed fails
// with EOF (seen in the b9994 discovery run). Every request therefore uses a fresh connection.
func TestStreamChatUsesFreshConnections(t *testing.T) {
	var mu sync.Mutex
	conns := 0
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			mu.Lock()
			conns++
			mu.Unlock()
		}
	}
	srv.Start()
	defer srv.Close()
	client := newLoopbackClient()
	for i := 0; i < 3; i++ {
		if _, err := streamChat(context.Background(), client, srv.URL, "k", testRequest, nil); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if conns != 3 {
		t.Fatalf("%d connections for 3 requests", conns)
	}
}
