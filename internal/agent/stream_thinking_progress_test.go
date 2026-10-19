package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestAnthropicLiveThinkingExtendsBothStreamReaders(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		t.Run(fmt.Sprintf("minimal=%t", minimal), func(t *testing.T) {
			const thought = "ongoing signed thinking "
			const count = 30
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"fixture\",\"model\":\"claude-sonnet-4\",\"usage\":{\"input_tokens\":10}}}\n\n")
				fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\",\"signature\":\"\",\"fixture_extension\":\"preserved\"}}\n\n")
				for i := 0; i < count; i++ {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
					fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":%q}}\n\n", thought)
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"signature_delta\",\"signature\":\"signed-fixture\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
				fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Finished\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
				fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":50}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			}))
			defer provider.Close()
			client := llm.WrapOpenAIClient(llm.NewClientFromProvider("anthropic", provider.URL+"/v1", "fixture"))
			parent, cancelParent := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelParent()
			ctx, cancel := llm.NewThroughputDeadline(parent, llm.ThroughputPolicy{Base: time.Second, ReserveTokens: 4096, MinTokensPerSecond: 1, MaxFactor: 4, MinSample: 50 * time.Millisecond})
			defer cancel()
			req := openai.ChatCompletionRequest{Model: "claude-sonnet-4", Stream: true, MaxTokens: 4096, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Finish the fixture."}}}
			var response openai.ChatCompletionResponse
			var err error
			if minimal {
				response, err = minimalLoopStreamText(ctx, client, req, 300*time.Millisecond)
			} else {
				retries := 0
				result := handleStreamingResponse(ctx, req, client, false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), NoopBroker{}, AgentTelemetryScope{}, cancel, 300*time.Millisecond, &retries, true)
				response, err = result.resp, result.err
			}
			if err != nil || len(response.Choices) != 1 || response.Choices[0].Message.Content != "Finished" {
				t.Fatalf("active thinking stream was interrupted: %+v %v", response, err)
			}
			reasoning := response.Choices[0].Message.ReasoningContent
			if llm.ReasoningText(reasoning) != strings.Repeat(thought, count) {
				t.Fatal("thinking text changed")
			}
			const prefix = "aurago:anthropic-reasoning:v1:"
			raw, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(reasoning, prefix))
			var blocks []map[string]any
			if !strings.HasPrefix(reasoning, prefix) || decodeErr != nil || json.Unmarshal(raw, &blocks) != nil || len(blocks) != 1 || blocks[0]["signature"] != "signed-fixture" || blocks[0]["fixture_extension"] != "preserved" {
				t.Fatal("signed native envelope changed")
			}
		})
	}
}

func TestFinalThinkingEnvelopeDoesNotDoubleCountLiveOutput(t *testing.T) {
	const text = "already measured thinking"
	raw, _ := json.Marshal([]map[string]string{{"type": "thinking", "thinking": text, "signature": "fixture"}})
	envelope := "aurago:anthropic-reasoning:v1:" + base64.StdEncoding.EncodeToString(raw)
	p := &streamThinkingProgress{}
	p.chars.Store(int64(len(text)))
	if got := p.reasoningChars(envelope); got != 0 {
		t.Fatalf("final envelope counted %d already measured bytes", got)
	}
	if got := p.reasoningChars(text); got != len(text) {
		t.Fatalf("plain provider reasoning counted %d bytes", got)
	}
}

func TestMinimalStreamSeparatesFirstTokenIdleAndParentDeadline(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		stream := &boundaryStream{ctx: ctx, heartbeat: heartbeat}
		_, err := minimalLoopStreamText(ctx, boundaryStreamClient{stream}, openai.ChatCompletionRequest{Model: "fixture"}, 25*time.Millisecond)
		cancel()
		if heartbeat && !errors.Is(err, errStreamIdleTimeout) {
			t.Fatalf("empty keepalives bypassed minimal idle limit: %v", err)
		}
		if !heartbeat && err != nil {
			t.Fatalf("first token incorrectly used minimal idle budget: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := minimalLoopStreamText(ctx, boundaryStreamClient{&boundaryStream{ctx: ctx, firstDelay: time.Second}}, openai.ChatCompletionRequest{Model: "fixture"}, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errStreamFirstTokenTimeout) {
		t.Fatalf("minimal stream lost parent deadline or first-token classification: %v", err)
	}
}
