package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestStepFunReasoningResponseCompatibility(t *testing.T) {
	for _, route := range []struct{ provider, model string }{
		{"stepfun", "step-5-preview"},
		{"openrouter", "stepfun/step-5-preview"},
		{"openai", "unrelated-model"},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", route.provider, stream), func(t *testing.T) {
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						// Real StepFun reasoning chunks need not contain usage.
						fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":\"private reasoning\"}}]}\n\n")
						fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"public answer\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":10,\"cached_tokens\":80}}\n\ndata: [DONE]\n\n")
					} else {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","reasoning":"private reasoning","content":"public answer"},"finish_reason":"stop"}]}`)
					}
				}))
				defer provider.Close()
				client := NewClientFromProvider(route.provider, provider.URL+"/v1", "fixture")
				req := openai.ChatCompletionRequest{Model: route.model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}}
				var content, reasoning string
				if stream {
					response, err := client.CreateChatCompletionStream(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					defer response.Close()
					for {
						chunk, err := response.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
						for _, choice := range chunk.Choices {
							content += choice.Delta.Content
							reasoning += choice.Delta.ReasoningContent
						}
					}
				} else {
					response, err := client.CreateChatCompletion(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					content, reasoning = response.Choices[0].Message.Content, response.Choices[0].Message.ReasoningContent
				}
				want := "private reasoning"
				if route.provider == "openai" {
					want = ""
				}
				if content != "public answer" || reasoning != want {
					t.Fatalf("reasoning lost or exposed in answer: content=%q reasoning=%q", content, reasoning)
				}
			})
		}
	}
}

func TestStepFunReasoningAliasesAndUsage(t *testing.T) {
	for _, tc := range []struct{ fields, want string }{
		{`"reasoning":"private"`, "private"},
		{`"reasoning_content":"private"`, "private"},
		{`"reasoning":"alias","reasoning_content":"private"`, "private"},
		{`"reasoning":"private","reasoning_content":null`, "private"},
		{`"reasoning":null`, ""},
		{`"reasoning":{"unexpected":true}`, ""},
	} {
		data := []byte(`{"choices":[{"message":{"content":"answer",` + tc.fields + `}}],"usage":{"prompt_tokens":100,"completion_tokens":10,"cached_tokens":80}}`)
		var measured RequestUsage
		encoded := normalizeObservedUsage(data, true, &measured)
		var response openai.ChatCompletionResponse
		if err := json.Unmarshal(encoded, &response); err != nil {
			t.Fatal(err)
		}
		message := response.Choices[0].Message
		if message.ReasoningContent != tc.want || message.Content != "answer" || response.Usage.PromptTokensDetails == nil || response.Usage.PromptTokensDetails.CachedTokens != 80 || measured.CacheReadTokens == nil || *measured.CacheReadTokens != 80 {
			t.Fatalf("reasoning or simultaneous cache normalization failed for %s", tc.fields)
		}
	}
}
