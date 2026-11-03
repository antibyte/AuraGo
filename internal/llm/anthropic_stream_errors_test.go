package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const anthropicStreamStartFixture = "event: message_start\ndata: {\"message\":{\"id\":\"msg_fixture\",\"model\":\"claude-sonnet-4-6\",\"usage\":{\"input_tokens\":4,\"output_tokens\":0}}}\n\n"
const anthropicStreamFinishFixture = "event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"

func anthropicSDKStreamForTest(t *testing.T, source io.ReadCloser) *openai.ChatCompletionStream {
	t.Helper()
	cfg := openai.DefaultConfig("fixture")
	cfg.BaseURL = "https://fixture.invalid/v1"
	cfg.HTTPClient = &http.Client{Transport: &anthropicTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: source, Request: req}, nil
	})}}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel)
	stream, err := openai.NewClientWithConfig(cfg).CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{Model: "claude-sonnet-4-6", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stream.Close() })
	return stream
}

func TestAnthropicStreamSDKPreservesNativeFailures(t *testing.T) {
	toolPrefix := "event: content_block_start\ndata: {\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"first\",\"name\":\"read\",\"input\":{}}}\n\nevent: content_block_delta\ndata: {\"index\":2,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"value\\\":\"}}\n\nevent: content_block_start\ndata: {\"index\":5,\"content_block\":{\"type\":\"tool_use\",\"id\":\"second\",\"name\":\"read\",\"input\":{}}}\n\nevent: content_block_delta\ndata: {\"index\":5,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"second\\\":true}\"}}\n\n"
	for _, failure := range []struct {
		kind     string
		status   int
		category ErrorCategory
	}{
		{"overloaded_error", 529, ErrCategoryTemporaryTransport},
		{"rate_limit_error", http.StatusTooManyRequests, ErrCategoryRateLimit},
		{"authentication_error", http.StatusUnauthorized, ErrCategoryAuthError},
	} {
		for _, phase := range []struct{ name, prefix string }{
			{"before_start", ""}, {"after_start", anthropicStreamStartFixture},
			{"text", anthropicStreamStartFixture + "event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n"},
			{"thinking", anthropicStreamStartFixture + "event: content_block_start\ndata: {\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\nevent: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"private fixture\"}}\n\n"},
			{"tools", anthropicStreamStartFixture + toolPrefix},
			{"after_finish_delta", anthropicStreamStartFixture + anthropicStreamFinishFixture},
		} {
			t.Run(failure.kind+"/"+phase.name, func(t *testing.T) {
				sse := phase.prefix + fmt.Sprintf("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":%q,\"message\":\"synthetic provider failure\"}}\n\n", failure.kind)
				stream := anthropicSDKStreamForTest(t, io.NopCloser(strings.NewReader(sse)))
				successes, failures := 0, 0
				observed := observeCompletionStream(t.Context(), stream, openai.ChatCompletionRequest{}, func() { successes++ }, func(error) { failures++ })
				var text strings.Builder
				arguments := map[int]string{}
				for {
					chunk, err := observed.Recv()
					if err != nil {
						var apiErr *openai.APIError
						if !errors.As(err, &apiErr) || apiErr.HTTPStatusCode != failure.status || apiErr.Type != failure.kind || ClassifyError(err) != failure.category {
							t.Fatalf("lost typed failure: %T %v", err, err)
						}
						break
					}
					for _, choice := range chunk.Choices {
						if choice.FinishReason != "" {
							t.Fatal("failed stream published a successful terminal marker")
						}
						text.WriteString(choice.Delta.Content)
						for _, call := range choice.Delta.ToolCalls {
							arguments[*call.Index] += call.Function.Arguments
						}
					}
				}
				if successes != 0 || failures != 1 || strings.Contains(text.String(), "provider failure") || strings.Contains(text.String(), "Anthropic Error") {
					t.Fatalf("failed stream became success or prose: successes=%d failures=%d text=%q", successes, failures, text.String())
				}
				if phase.name == "tools" && !reflect.DeepEqual(arguments, map[int]string{0: `{"value":`, 1: `{"second":true}`}) {
					t.Fatalf("error mutated tool indices or arguments: %+v", arguments)
				}
			})
		}
	}
}

type anthropicFailingReader struct{ err error }

func (r anthropicFailingReader) Read([]byte) (int, error) { return 0, r.err }

func TestAnthropicStreamReadFailuresReachSDKWithoutDone(t *testing.T) {
	sentinel := errors.New("fixture read failure")
	for _, terminalDelta := range []bool{false, true} {
		prefix := anthropicStreamStartFixture
		if terminalDelta {
			prefix += anthropicStreamFinishFixture
		}
		for _, nativeErr := range []error{sentinel, io.EOF} {
			stream := anthropicSDKStreamForTest(t, io.NopCloser(io.MultiReader(strings.NewReader(prefix), anthropicFailingReader{nativeErr})))
			for {
				chunk, err := stream.Recv()
				if err != nil {
					want := nativeErr
					if nativeErr == io.EOF {
						want = io.ErrUnexpectedEOF
					}
					if !errors.Is(err, want) {
						t.Fatalf("lost read error: got %v want %v", err, want)
					}
					break
				}
				for _, choice := range chunk.Choices {
					if choice.FinishReason != "" {
						t.Fatal("truncated native stream published finish")
					}
				}
			}
		}
	}
}

func TestAnthropicStreamMalformedKnownEventsFail(t *testing.T) {
	for _, event := range []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop", "error"} {
		for _, payload := range []string{"{", "null", "[]"} {
			t.Run(event+payload, func(t *testing.T) {
				var out bytes.Buffer
				err := translateStreamEvents(strings.NewReader(anthropicStreamStartFixture+anthropicStreamFinishFixture+"event: "+event+"\ndata: "+payload+"\n\n"), &out, "fixture", nil)
				if err == nil || strings.Contains(out.String(), "[DONE]") || strings.Contains(out.String(), `"finish_reason":"stop"`) {
					t.Fatalf("malformed event became success: err=%v output=%s", err, out.String())
				}
			})
		}
	}
}

func TestAnthropicStreamRejectsUnknownOrDuplicateToolBlocks(t *testing.T) {
	block := "event: content_block_start\ndata: {\"index\":2,\"content_block\":{\"type\":\"tool_use\",\"id\":\"fixture\",\"name\":\"read\",\"input\":{}}}\n\n"
	for _, content := range []string{
		block + block,
		block + "event: content_block_delta\ndata: {\"index\":1000000000,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{}\"}}\n\n",
	} {
		var out bytes.Buffer
		err := translateStreamEvents(strings.NewReader(anthropicStreamStartFixture+content+anthropicStreamFinishFixture+"event: message_stop\ndata: {}\n\n"), &out, "fixture", nil)
		if err == nil || strings.Contains(out.String(), "[DONE]") {
			t.Fatalf("invalid tool index became successful completion: %v", err)
		}
	}
}

type anthropicCloseProbe struct {
	io.ReadCloser
	closed chan struct{}
	closes atomic.Int32
}

func (r *anthropicCloseProbe) Close() error {
	if r.closes.Add(1) == 1 {
		close(r.closed)
	}
	return r.ReadCloser.Close()
}

func TestAnthropicStreamCloseUnblocksNativeReadOnce(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	source := &anthropicCloseProbe{ReadCloser: r, closed: make(chan struct{})}
	resp, err := translateAnthropicStream(&http.Response{Body: source, Header: make(http.Header)}, "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.closed:
	case <-time.After(time.Second):
		t.Fatal("downstream close did not close blocked native reader")
	}
	_ = resp.Body.Close()
	if source.closes.Load() != 1 {
		t.Fatalf("source closed %d times", source.closes.Load())
	}
}

func TestAnthropicStreamReframesLengthAndLineEndings(t *testing.T) {
	sse := anthropicStreamStartFixture + "event: future_event\ndata: ignored\n\n" + anthropicStreamFinishFixture + "event: message_stop\ndata: {}\n\n"
	var baseline []byte
	for _, data := range []string{sse, strings.ReplaceAll(sse, "\n", "\r\n"), strings.ReplaceAll(sse, ": ", ":")} {
		resp, err := translateAnthropicStream(&http.Response{Body: io.NopCloser(strings.NewReader(data)), ContentLength: int64(len(data)), Header: http.Header{"Content-Length": {strconv.Itoa(len(data))}, "Content-Type": {"text/event-stream"}}}, "fixture", nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.ContentLength != -1 || resp.Header.Get("Content-Length") != "" {
			t.Fatalf("reframing: length=%d header=%q err=%v", resp.ContentLength, resp.Header.Get("Content-Length"), err)
		}
		if baseline == nil {
			baseline = got
		} else if !bytes.Equal(baseline, got) {
			t.Fatal("valid line endings or field spacing changed translation")
		}
	}
}

func TestAnthropicStreamStopsAfterDownstreamWriteFailure(t *testing.T) {
	sentinel := errors.New("fixture write failure")
	w := &anthropicFailingWriter{err: sentinel}
	err := translateStreamEvents(strings.NewReader(anthropicStreamStartFixture+anthropicStreamFinishFixture), w, "fixture", nil)
	if !errors.Is(err, sentinel) || w.writes != 1 {
		t.Fatalf("ignored downstream failure: writes=%d err=%v", w.writes, err)
	}
}

func TestAnthropicStreamCallerCancellationClosesNativeRequest(t *testing.T) {
	closed := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicStreamStartFixture)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer provider.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	client := NewClientFromProvider("anthropic", provider.URL+"/v1", "fixture")
	stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{Model: "claude-sonnet-4-6", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := stream.Recv(); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost caller cancellation: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("native HTTP request remained open after cancellation")
	}
}

type anthropicFailingWriter struct {
	err    error
	writes int
}

func (w *anthropicFailingWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, w.err
}
