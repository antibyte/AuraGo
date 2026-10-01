package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

const strictSystemTemplateError = `{"error":{"message":"messages[6]: this model's chat template rejected the conversation: System message must be at the beginning.","type":"invalid_request_error","code":422}}`

type strictTemplateServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []strictTemplateRequest
}

type strictTemplateRequest struct {
	Model    string
	Roles    []string
	Contents []string
	Raw      []byte
}

// newStrictTemplateServer mimics a llama.cpp/vLLM style endpoint whose chat
// template raises when a system message is not first. Only strictModel is
// strict; any other model accepts arbitrary role ordering.
func newStrictTemplateServer(t *testing.T, strictModel string) *strictTemplateServer {
	t.Helper()
	s := &strictTemplateServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var wire struct {
			Model    string `json:"model"`
			Stream   bool   `json:"stream"`
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Error(err)
			return
		}
		rec := strictTemplateRequest{Model: wire.Model, Raw: body}
		rejected := false
		for i, m := range wire.Messages {
			rec.Roles = append(rec.Roles, m.Role)
			var text string
			_ = json.Unmarshal(m.Content, &text)
			rec.Contents = append(rec.Contents, text)
			if m.Role == "system" && i > 0 && wire.Model == strictModel {
				rejected = true
			}
		}
		s.mu.Lock()
		s.requests = append(s.requests, rec)
		s.mu.Unlock()
		if rejected {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, strictSystemTemplateError)
			return
		}
		if wire.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *strictTemplateServer) seen() []strictTemplateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]strictTemplateRequest(nil), s.requests...)
}

func (s *strictTemplateServer) client() *openai.Client {
	return NewClientFromProviderWithConfig(nil, "openai", s.URL+"/v1", "test-only", "")
}

const recoveryHintText = "RECOVERY HINT: Do not repeat the exact same tool call."

// recoveryConversation is the request shape seen in production: the agent's
// duplicate-tool-call guard appends a system message after a tool round.
func recoveryConversation() []openai.ChatCompletionMessage {
	index := 0
	call := func(id string) []openai.ToolCall {
		return []openai.ToolCall{{Index: &index, ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "homepage_registry", Arguments: `{}`}}}
	}
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "# CORE IDENTITY\nYou are AuraGo."},
		{Role: openai.ChatMessageRoleUser, Content: "List the history."},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: call("call-1")},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-1", Content: "error: id required"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: call("call-2")},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-2", Content: "error: id required"},
		{Role: openai.ChatMessageRoleSystem, Content: recoveryHintText},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: call("call-3")},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-3", Content: "error: id required"},
	}
}

func sendChat(t *testing.T, client *openai.Client, model string, stream bool, messages []openai.ChatCompletionMessage) error {
	t.Helper()
	request := openai.ChatCompletionRequest{Model: model, Messages: messages}
	if stream {
		stm, err := client.CreateChatCompletionStream(context.Background(), request)
		if err != nil {
			return err
		}
		defer stm.Close()
		_, err = stm.Recv()
		return err
	}
	_, err := client.CreateChatCompletion(context.Background(), request)
	return err
}

func TestStrictTemplateProviderGetsMidConversationSystemMessagesDemoted(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			server := newStrictTemplateServer(t, "strict-model")
			client := server.client()
			original := recoveryConversation()

			if err := sendChat(t, client, "strict-model", stream, original); err != nil {
				t.Fatalf("request failed instead of being adapted: %v", err)
			}
			seen := server.seen()
			if len(seen) != 2 {
				t.Fatalf("expected the rejected request plus one adapted retry, got %d requests", len(seen))
			}
			retry := seen[1]
			wantRoles := []string{"system", "user", "assistant", "tool", "assistant", "tool", "user", "assistant", "tool"}
			if fmt.Sprint(retry.Roles) != fmt.Sprint(wantRoles) {
				t.Fatalf("retry roles = %v, want %v", retry.Roles, wantRoles)
			}
			if !strings.Contains(retry.Contents[6], recoveryHintText) {
				t.Fatalf("the recovery hint must survive at its original position, got %q", retry.Contents[6])
			}
			if retry.Contents[0] != original[0].Content {
				t.Fatalf("leading system prompt changed: %q", retry.Contents[0])
			}
			if original[6].Role != openai.ChatMessageRoleSystem {
				t.Fatal("the caller's private message history was mutated")
			}

			// The provider has now been learned: the next request is adapted up front.
			if err := sendChat(t, client, "strict-model", stream, original); err != nil {
				t.Fatalf("second request failed: %v", err)
			}
			seen = server.seen()
			if len(seen) != 3 {
				t.Fatalf("second request should need no retry, got %d total requests", len(seen))
			}
			if fmt.Sprint(seen[2].Roles) != fmt.Sprint(wantRoles) {
				t.Fatalf("learned request roles = %v, want %v", seen[2].Roles, wantRoles)
			}
		})
	}
}

func TestStrictTemplateProviderMergesLeadingSystemRun(t *testing.T) {
	server := newStrictTemplateServer(t, "strict-model")
	client := server.client()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "PRIMARY"},
		{Role: openai.ChatMessageRoleSystem, Content: "[RELEVANT_CONVERSATION_CONTEXT] recall"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
	}
	if err := sendChat(t, client, "strict-model", false, messages); err != nil {
		t.Fatal(err)
	}
	seen := server.seen()
	last := seen[len(seen)-1]
	if fmt.Sprint(last.Roles) != "[system user]" {
		t.Fatalf("roles = %v, want a single leading system message", last.Roles)
	}
	if !strings.HasPrefix(last.Contents[0], "PRIMARY") || !strings.Contains(last.Contents[0], "[RELEVANT_CONVERSATION_CONTEXT] recall") {
		t.Fatalf("merged system content lost text: %q", last.Contents[0])
	}
}

func TestTolerantProviderKeepsSystemMessagesUntouched(t *testing.T) {
	server := newStrictTemplateServer(t, "strict-model")
	client := server.client()
	if err := sendChat(t, client, "tolerant-model", false, recoveryConversation()); err != nil {
		t.Fatal(err)
	}
	seen := server.seen()
	if len(seen) != 1 {
		t.Fatalf("a provider that accepts the request must see exactly one, got %d", len(seen))
	}
	if seen[0].Roles[6] != "system" || seen[0].Contents[6] != recoveryHintText {
		t.Fatalf("system message was rewritten for a tolerant provider: roles=%v", seen[0].Roles)
	}
}

func TestLearnedStrictTemplateIsScopedToModel(t *testing.T) {
	server := newStrictTemplateServer(t, "strict-model")
	client := server.client()
	if err := sendChat(t, client, "strict-model", false, recoveryConversation()); err != nil {
		t.Fatal(err)
	}
	before := len(server.seen())
	if err := sendChat(t, client, "tolerant-model", false, recoveryConversation()); err != nil {
		t.Fatal(err)
	}
	seen := server.seen()
	if len(seen) != before+1 {
		t.Fatalf("unexpected request count %d, want %d", len(seen), before+1)
	}
	if seen[before].Roles[6] != "system" {
		t.Fatalf("learning for one model rewrote another model on the same host: %v", seen[before].Roles)
	}
}

func TestUnrelatedProviderRejectionIsNotRetried(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"error":{"message":"messages[2]: unsupported tool role content","type":"invalid_request_error"}}`)
	}))
	defer provider.Close()
	client := NewClientFromProviderWithConfig(nil, "openai", provider.URL+"/v1", "test-only", "")

	err := sendChat(t, client, "any-model", false, recoveryConversation())
	if err == nil {
		t.Fatal("expected the provider rejection to be returned")
	}
	if calls != 1 {
		t.Fatalf("unrelated rejections must not be retried, provider saw %d requests", calls)
	}
	if !strings.Contains(err.Error(), "unsupported tool role content") {
		t.Fatalf("provider error body was lost: %v", err)
	}
}

func TestNormalizeLeadingSystemMessagesShapes(t *testing.T) {
	roleAndContent := func(t *testing.T, body []byte) ([]string, []string) {
		t.Helper()
		var wire struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatal(err)
		}
		var roles, contents []string
		for _, m := range wire.Messages {
			roles = append(roles, m.Role)
			contents = append(contents, string(m.Content))
		}
		return roles, contents
	}

	t.Run("nothing to change", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[{"role":"system","content":"s"},{"role":"user","content":"u"}]}`)
		out, changed := normalizeLeadingSystemMessages(body)
		if changed || string(out) != string(body) {
			t.Fatalf("a conforming conversation must be returned untouched, changed=%v", changed)
		}
	})

	t.Run("no leading system demotes every system message", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[{"role":"user","content":"u"},{"role":"system","content":"late"}]}`)
		out, changed := normalizeLeadingSystemMessages(body)
		if !changed {
			t.Fatal("expected a change")
		}
		roles, contents := roleAndContent(t, out)
		if fmt.Sprint(roles) != "[user user]" || !strings.Contains(contents[1], "late") {
			t.Fatalf("roles=%v contents=%v", roles, contents)
		}
	})

	t.Run("text part arrays are merged and labelled", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[` +
			`{"role":"system","content":[{"type":"text","text":"A"},{"type":"text","text":"B"}]},` +
			`{"role":"system","content":"C"},` +
			`{"role":"user","content":"u"},` +
			`{"role":"system","content":[{"type":"text","text":"late"}]}]}`)
		out, changed := normalizeLeadingSystemMessages(body)
		if !changed {
			t.Fatal("expected a change")
		}
		roles, contents := roleAndContent(t, out)
		if fmt.Sprint(roles) != "[system user user]" {
			t.Fatalf("roles = %v", roles)
		}
		if contents[0] != `"AB\n\nC"` {
			t.Fatalf("merged content = %s", contents[0])
		}
		if !strings.Contains(contents[2], "[System notice]") || !strings.Contains(contents[2], "late") {
			t.Fatalf("demoted part array lost its label or text: %s", contents[2])
		}
	})

	t.Run("non text system content is left alone", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[{"role":"system","content":[{"type":"image_url","image_url":{"url":"x"}}]},{"role":"user","content":"u"},{"role":"system","content":"late"}]}`)
		out, changed := normalizeLeadingSystemMessages(body)
		if changed || string(out) != string(body) {
			t.Fatal("content that cannot be merged losslessly must not be rewritten")
		}
	})

	t.Run("invalid payloads are left alone", func(t *testing.T) {
		for _, body := range []string{``, `not json`, `{"messages":"x"}`, `{"messages":[{"content":"no role"}]}`} {
			if out, changed := normalizeLeadingSystemMessages([]byte(body)); changed || string(out) != body {
				t.Fatalf("payload %q must be returned untouched", body)
			}
		}
	})
}

func TestStrictTemplateRejectionWithoutSystemMessagesIsReturned(t *testing.T) {
	// A server may emit the template error for reasons we cannot repair (no
	// system message to move). The error must surface instead of looping.
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, strictSystemTemplateError)
	}))
	defer provider.Close()
	client := NewClientFromProviderWithConfig(nil, "openai", provider.URL+"/v1", "test-only", "")

	err := sendChat(t, client, "any-model", false, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "only system"},
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
	})
	if err == nil || !strings.Contains(err.Error(), "System message must be at the beginning") {
		t.Fatalf("expected the template error to surface, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("nothing to repair, so exactly one request is expected, got %d", calls)
	}
}
