package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Request translation tests
// ---------------------------------------------------------------------------

func TestAnthropicRequestTranslation(t *testing.T) {
	oai := openaiRequest{
		Model:       "claude-sonnet-4-20250514",
		MaxTokens:   1024,
		Temperature: ptrFloat32(0.7),
		Stream:      false,
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		}),
		User: "test-user",
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if ant.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", ant.Model)
	}
	if ant.MaxTokens != 1024 {
		t.Errorf("max_tokens = %d, want 1024", ant.MaxTokens)
	}
	if ant.System != "You are helpful." {
		t.Errorf("system = %q, want 'You are helpful.'", ant.System)
	}
	if len(ant.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(ant.Messages))
	}
	if ant.Messages[0].Role != "user" {
		t.Errorf("messages[0].role = %q, want 'user'", ant.Messages[0].Role)
	}
	if ant.Metadata == nil || ant.Metadata.UserID != "test-user" {
		t.Errorf("metadata.user_id not mapped correctly")
	}
}

func TestAnthropicMaxTokensDefault(t *testing.T) {
	oai := openaiRequest{
		Model:    "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{{Role: "user", Content: "Hi"}}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.MaxTokens != anthropicDefaultMaxTokens {
		t.Errorf("max_tokens = %d, want %d", ant.MaxTokens, anthropicDefaultMaxTokens)
	}
}

func TestAnthropicThinkingConditionalEnable(t *testing.T) {
	oai := openaiRequest{
		Model:     "claude-3-5-sonnet-latest",
		MaxTokens: 8192,
		Messages:  marshalMessages(t, []openaiMessage{{Role: "user", Content: "Hi"}}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{Enabled: true, BudgetTokens: 4242})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.Thinking != nil {
		t.Fatalf("expected thinking disabled for older model, got %+v", ant.Thinking)
	}

	oai.Model = "claude-sonnet-4-20250514"
	ant, err = translateOpenAIToAnthropic(oai, anthropicThinkingConfig{Enabled: true, BudgetTokens: 4242})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.Thinking == nil || ant.Thinking.Type != "enabled" || ant.Thinking.BudgetTokens != 4242 {
		t.Fatalf("expected thinking enabled with budget 4242, got %+v", ant.Thinking)
	}
}

func TestAnthropicThinkingBudgetClamping(t *testing.T) {
	oai := openaiRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 128,
		Messages:  marshalMessages(t, []openaiMessage{{Role: "user", Content: "Hi"}}),
	}

	// Budget exceeding MaxTokens should be clamped to MaxTokens/2
	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{Enabled: true, BudgetTokens: 10000})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.Thinking == nil {
		t.Fatal("expected thinking to be enabled")
	}
	if ant.Thinking.BudgetTokens != 64 {
		t.Errorf("expected budget clamped to 64, got %d", ant.Thinking.BudgetTokens)
	}

	// Budget exceeding maxThinkingBudget should be clamped to maxThinkingBudget
	oai.MaxTokens = 65536
	ant, err = translateOpenAIToAnthropic(oai, anthropicThinkingConfig{Enabled: true, BudgetTokens: 50000})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.Thinking == nil {
		t.Fatal("expected thinking to be enabled")
	}
	if ant.Thinking.BudgetTokens != 32000 {
		t.Errorf("expected budget clamped to 32000, got %d", ant.Thinking.BudgetTokens)
	}

	// Negative budget should use default, then clamped by MaxTokens
	oai.MaxTokens = 8192
	ant, err = translateOpenAIToAnthropic(oai, anthropicThinkingConfig{Enabled: true, BudgetTokens: -1})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if ant.Thinking == nil {
		t.Fatal("expected thinking to be enabled")
	}
	// Default 10000 is clamped to MaxTokens/2 = 4096
	if ant.Thinking.BudgetTokens != 4096 {
		t.Errorf("expected default budget clamped to 4096, got %d", ant.Thinking.BudgetTokens)
	}
}

func TestAnthropicSystemMessageExtraction(t *testing.T) {
	oai := openaiRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "system", Content: "System prompt 1"},
			{Role: "system", Content: "System prompt 2"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "Bye"},
		}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if ant.System != "System prompt 1\n\nSystem prompt 2" {
		t.Errorf("system = %q, want concatenated system prompts", ant.System)
	}
	if len(ant.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(ant.Messages))
	}
}

func TestAnthropicConsecutiveUserMerge(t *testing.T) {
	oai := openaiRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "user", Content: "First message"},
			{Role: "user", Content: "Second message"},
		}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(ant.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1 (merged)", len(ant.Messages))
	}
	if ant.Messages[0].Role != "user" {
		t.Errorf("role = %q, want 'user'", ant.Messages[0].Role)
	}
}

func TestAnthropicToolDefinitionTranslation(t *testing.T) {
	toolJSON := `{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}`

	oai := openaiRequest{
		Model:    "claude-3-5-sonnet-latest",
		Tools:    []json.RawMessage{json.RawMessage(toolJSON)},
		Messages: marshalMessages(t, []openaiMessage{{Role: "user", Content: "Weather?"}}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(ant.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(ant.Tools))
	}
	if ant.Tools[0].Name != "get_weather" {
		t.Errorf("tool name = %q, want 'get_weather'", ant.Tools[0].Name)
	}
	if ant.Tools[0].Description != "Get weather" {
		t.Errorf("tool description = %q, want 'Get weather'", ant.Tools[0].Description)
	}
}

func TestAnthropicToolResultTranslation(t *testing.T) {
	oai := openaiRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "user", Content: "Get weather"},
			{Role: "assistant", Content: "", ToolCalls: []openaiToolCall{
				{ID: "call_123", Type: "function", Function: openaiToolCallFn{Name: "get_weather", Arguments: `{"city":"Berlin"}`}},
			}},
			{Role: "tool", Content: "Sunny, 22°C", ToolCallID: "call_123"},
		}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	// Should have: user, assistant (with tool_use), user (with tool_result)
	if len(ant.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(ant.Messages))
	}

	// Last message should be user with tool_result
	lastMsg := ant.Messages[2]
	if lastMsg.Role != "user" {
		t.Errorf("last message role = %q, want 'user'", lastMsg.Role)
	}

	blocks, ok := lastMsg.Content.([]anthropicContentBlock)
	if !ok {
		t.Fatalf("last message content is not []anthropicContentBlock")
	}
	if len(blocks) != 1 {
		t.Fatalf("tool_result blocks = %d, want 1", len(blocks))
	}
	if blocks[0].Type != "tool_result" {
		t.Errorf("block type = %q, want 'tool_result'", blocks[0].Type)
	}
	if blocks[0].ToolUseID != "call_123" {
		t.Errorf("tool_use_id = %q, want 'call_123'", blocks[0].ToolUseID)
	}
}

func TestAnthropicToolChoiceMapping(t *testing.T) {
	tests := []struct {
		input    any
		wantType string
		wantName string
		wantNil  bool
	}{
		{input: "auto", wantType: "auto"},
		{input: "none", wantNil: true},
		{input: "required", wantType: "any"},
		{input: map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "my_tool"}}, wantType: "tool", wantName: "my_tool"},
	}

	for _, tt := range tests {
		tc, err := translateToolChoice(tt.input)
		if err != nil {
			t.Errorf("toolChoice(%v): err = %v", tt.input, err)
			continue
		}
		if tt.wantNil {
			if tc != nil {
				t.Errorf("toolChoice(%v) = %+v, want nil", tt.input, tc)
			}
			continue
		}
		if tc == nil {
			t.Errorf("toolChoice(%v) = nil, want type=%q", tt.input, tt.wantType)
			continue
		}
		if tc.Type != tt.wantType {
			t.Errorf("toolChoice(%v).type = %q, want %q", tt.input, tc.Type, tt.wantType)
		}
		if tt.wantName != "" && tc.Name != tt.wantName {
			t.Errorf("toolChoice(%v).name = %q, want %q", tt.input, tc.Name, tt.wantName)
		}
	}
}

func TestAnthropicStopSequences(t *testing.T) {
	// string input
	seqs := translateStopSequences("STOP")
	if len(seqs) != 1 || seqs[0] != "STOP" {
		t.Errorf("string stop = %v, want [STOP]", seqs)
	}

	// array input
	seqs = translateStopSequences([]interface{}{"END", "DONE"})
	if len(seqs) != 2 {
		t.Errorf("array stop len = %d, want 2", len(seqs))
	}

	// nil input
	seqs = translateStopSequences(nil)
	if seqs != nil {
		t.Errorf("nil stop = %v, want nil", seqs)
	}
}

func TestAnthropicEmptyContent(t *testing.T) {
	oai := openaiRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "user", Content: ""},
		}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if len(ant.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(ant.Messages))
	}
	// Content should be " " (space) not empty
	content, ok := ant.Messages[0].Content.(string)
	if !ok {
		t.Fatalf("content is not string")
	}
	if content == "" {
		t.Error("empty content should be replaced with space")
	}
}

func TestAnthropicStopReasonMapping(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"end_turn", "stop"},
		{"tool_use", "tool_calls"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"unknown", "stop"},
	}
	for _, tt := range tests {
		got := mapStopReason(tt.input)
		if got != tt.want {
			t.Errorf("mapStopReason(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAnthropicImageTranslation(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgo="
	part := map[string]interface{}{
		"image_url": map[string]interface{}{
			"url": dataURI,
		},
	}

	block, err := translateImageURL(part)
	if err != nil {
		t.Fatalf("translateImageURL: %v", err)
	}
	if block == nil {
		t.Fatal("block is nil")
	}
	if block.Type != "image" {
		t.Errorf("type = %q, want 'image'", block.Type)
	}
	if block.Source.Type != "base64" {
		t.Errorf("source.type = %q, want 'base64'", block.Source.Type)
	}
	if block.Source.MediaType != "image/png" {
		t.Errorf("source.media_type = %q, want 'image/png'", block.Source.MediaType)
	}
}

func TestAnthropicImageTranslationErrors(t *testing.T) {
	tests := []struct {
		name    string
		part    map[string]interface{}
		wantErr string
	}{
		{
			name:    "nil part",
			part:    nil,
			wantErr: "missing image_url in part",
		},
		{
			name:    "missing image_url key",
			part:    map[string]interface{}{},
			wantErr: "missing image_url in part",
		},
		{
			name:    "image_url not a map",
			part:    map[string]interface{}{"image_url": "not-a-map"},
			wantErr: "image_url is not a map",
		},
		{
			name:    "empty url",
			part:    map[string]interface{}{"image_url": map[string]interface{}{"url": ""}},
			wantErr: "image_url.url is empty",
		},
		{
			name: "invalid base64 data",
			part: map[string]interface{}{
				"image_url": map[string]interface{}{
					"url": "data:image/png;base64,!!!invalid!!!",
				},
			},
			wantErr: "invalid base64 data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := translateImageURL(tt.part)
			if block != nil {
				t.Errorf("translateImageURL() block = %v, want nil", block)
			}
			if err == nil {
				t.Fatal("translateImageURL() err = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("translateImageURL() err = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAnthropicPassthrough(t *testing.T) {
	// Non-chat-completion requests should pass through unchanged
	called := false
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})

	transport := &anthropicTransport{base: base}
	req, _ := http.NewRequest(http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	if !called {
		t.Error("base transport was not called")
	}
}

// ---------------------------------------------------------------------------
// Response translation tests
// ---------------------------------------------------------------------------

func TestAnthropicResponseTranslation(t *testing.T) {
	antResp := anthropicResponse{
		ID:    "msg_01XFDUDYJgAACzvnptvVoYEL",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []anthropicResponseBlock{
			{Type: "text", Text: "Hello! How can I help?"},
		},
		StopReason: "end_turn",
		Usage:      anthropicUsage{InputTokens: 25, OutputTokens: 10},
	}

	oai := mapAnthropicToOpenAI(antResp)

	if oai.ID != "chatcmpl-01XFDUDYJgAACzvnptvVoYEL" {
		t.Errorf("id = %q", oai.ID)
	}
	if oai.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("model = %q", oai.Model)
	}
	if len(oai.Choices) != 1 {
		t.Fatalf("choices len = %d", len(oai.Choices))
	}
	if oai.Choices[0].Message.Content != "Hello! How can I help?" {
		t.Errorf("content = %q", oai.Choices[0].Message.Content)
	}
	if *oai.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %q", *oai.Choices[0].FinishReason)
	}
	if oai.Usage.PromptTokens != 25 || oai.Usage.CompletionTokens != 10 {
		t.Errorf("usage = %+v", oai.Usage)
	}
}

func TestAnthropicParallelToolCalls(t *testing.T) {
	antResp := anthropicResponse{
		ID:    "msg_abc",
		Model: "claude-3-5-sonnet-latest",
		Content: []anthropicResponseBlock{
			{Type: "tool_use", ID: "toolu_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Berlin"}`)},
			{Type: "tool_use", ID: "toolu_2", Name: "get_time", Input: json.RawMessage(`{"tz":"CET"}`)},
		},
		StopReason: "tool_use",
		Usage:      anthropicUsage{InputTokens: 50, OutputTokens: 30},
	}

	oai := mapAnthropicToOpenAI(antResp)

	if len(oai.Choices[0].Message.ToolCalls) != 2 {
		t.Fatalf("tool_calls len = %d, want 2", len(oai.Choices[0].Message.ToolCalls))
	}
	tc0 := oai.Choices[0].Message.ToolCalls[0]
	if tc0.ID != "toolu_1" || tc0.Function.Name != "get_weather" {
		t.Errorf("tool_call[0] = %+v", tc0)
	}
	tc1 := oai.Choices[0].Message.ToolCalls[1]
	if tc1.ID != "toolu_2" || tc1.Function.Name != "get_time" {
		t.Errorf("tool_call[1] = %+v", tc1)
	}
	if *oai.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %q, want 'tool_calls'", *oai.Choices[0].FinishReason)
	}
}

// ---------------------------------------------------------------------------
// Streaming tests
// ---------------------------------------------------------------------------

func TestAnthropicStreamTextOnly(t *testing.T) {
	// Simulate Anthropic streaming response using a custom RoundTripper
	// to avoid httptest TCP/pipe interaction issues
	ssePayload := buildAnthropicSSE([]string{
		"event: message_start\ndata: " + `{"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","model":"claude-3-5-sonnet-latest","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`,
		"event: content_block_start\ndata: " + `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		"event: content_block_stop\ndata: " + `{"type":"content_block_stop","index":0}`,
		"event: message_delta\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		"event: message_stop\ndata: " + `{"type":"message_stop"}`,
	})

	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(ssePayload)),
		}, nil
	})

	transport := &anthropicTransport{base: base}
	client := &http.Client{Transport: transport}

	body := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	resp, err := client.Post("https://api.anthropic.com/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	output := string(data)

	if !strings.Contains(output, `"role":"assistant"`) {
		t.Error("missing initial role chunk")
	}
	if !strings.Contains(output, `"Hello"`) {
		t.Error("missing 'Hello' text delta")
	}
	if !strings.Contains(output, `" world"`) {
		t.Error("missing ' world' text delta")
	}
	if !strings.Contains(output, `"finish_reason":"stop"`) {
		t.Error("missing finish_reason")
	}
	if !strings.Contains(output, "data: [DONE]") {
		t.Error("missing [DONE]")
	}
}

func TestAnthropicStreamToolUse(t *testing.T) {
	ssePayload := buildAnthropicSSE([]string{
		"event: message_start\ndata: " + `{"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","model":"claude-3-5-sonnet-latest","content":[],"stop_reason":null,"usage":{"input_tokens":50,"output_tokens":0}}}`,
		"event: content_block_start\ndata: " + `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"get_weather"}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"Berlin\"}"}}`,
		"event: content_block_stop\ndata: " + `{"type":"content_block_stop","index":0}`,
		"event: message_delta\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
		"event: message_stop\ndata: " + `{"type":"message_stop"}`,
	})

	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(ssePayload)),
		}, nil
	})

	transport := &anthropicTransport{base: base}
	client := &http.Client{Transport: transport}

	body := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"Weather?"}],"stream":true,"tools":[{"type":"function","function":{"name":"get_weather","parameters":{}}}]}`
	resp, err := client.Post("https://api.anthropic.com/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	output := string(data)

	if !strings.Contains(output, `"get_weather"`) {
		t.Error("missing tool name in stream")
	}
	if !strings.Contains(output, `toolu_abc`) {
		t.Error("missing tool call ID in stream")
	}
	if !strings.Contains(output, `"finish_reason":"tool_calls"`) {
		t.Error("missing finish_reason 'tool_calls'")
	}
	if !strings.Contains(output, "data: [DONE]") {
		t.Error("missing [DONE]")
	}
}

func TestAnthropicStreamThinking(t *testing.T) {
	// Test that streaming thinking events are processed correctly
	ssePayload := buildAnthropicSSE([]string{
		"event: message_start\ndata: " + `{"type":"message_start","message":{"id":"msg_think","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"stop_reason":null,"usage":{"input_tokens":20,"output_tokens":0}}}`,
		"event: content_block_start\ndata: " + `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me think about this"}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":" step by step"}}`,
		"event: content_block_stop\ndata: " + `{"type":"content_block_stop","index":0}`,
		"event: content_block_start\ndata: " + `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		"event: content_block_delta\ndata: " + `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Here is my answer"}}`,
		"event: content_block_stop\ndata: " + `{"type":"content_block_stop","index":1}`,
		"event: message_delta\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`,
		"event: message_stop\ndata: " + `{"type":"message_stop"}`,
	})

	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(ssePayload)),
		}, nil
	})

	transport := &anthropicTransport{base: base}
	client := &http.Client{Transport: transport}

	body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Think about something"}],"stream":true}`
	resp, err := client.Post("https://api.anthropic.com/v1/chat/completions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	output := string(data)

	// Verify text content is present
	if !strings.Contains(output, `"Here is my answer"`) {
		t.Error("missing text content in stream")
	}
	// Verify finish_reason is present
	if !strings.Contains(output, `"finish_reason":"stop"`) {
		t.Error("missing finish_reason")
	}
	// Verify [DONE] is present
	if !strings.Contains(output, "data: [DONE]") {
		t.Error("missing [DONE]")
	}
}

// ---------------------------------------------------------------------------
// Integration tests: full round-trip through httptest
// ---------------------------------------------------------------------------

func TestAnthropicE2E_Chat(t *testing.T) {
	// Mock Anthropic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("Authorization header should be removed")
		}

		// Parse request to verify translation
		body, _ := io.ReadAll(r.Body)
		var antReq anthropicRequest
		if err := json.Unmarshal(body, &antReq); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		if antReq.System != "You are helpful." {
			t.Errorf("system = %q", antReq.System)
		}
		if antReq.MaxTokens == 0 {
			t.Error("max_tokens should not be 0")
		}

		// Return Anthropic response
		resp := anthropicResponse{
			ID:    "msg_test_e2e",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-5-sonnet-latest",
			Content: []anthropicResponseBlock{
				{Type: "text", Text: "Hello from Claude!"},
			},
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 15, OutputTokens: 8},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transport := &anthropicTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	reqBody := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hi"}],"max_tokens":100}`
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-ant-test-key")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	var oaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if oaiResp.ID != "chatcmpl-test_e2e" {
		t.Errorf("id = %q", oaiResp.ID)
	}
	if len(oaiResp.Choices) != 1 {
		t.Fatalf("choices = %d", len(oaiResp.Choices))
	}
	if oaiResp.Choices[0].Message.Content != "Hello from Claude!" {
		t.Errorf("content = %q", oaiResp.Choices[0].Message.Content)
	}
	if oaiResp.Usage.PromptTokens != 15 {
		t.Errorf("prompt_tokens = %d", oaiResp.Usage.PromptTokens)
	}
}

func TestAnthropicE2E_ToolCycle(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var antReq anthropicRequest
		json.Unmarshal(body, &antReq)

		if callCount == 1 {
			// First call: return tool_use
			resp := anthropicResponse{
				ID: "msg_1", Type: "message", Role: "assistant",
				Model: "claude-3-5-sonnet-latest",
				Content: []anthropicResponseBlock{
					{Type: "tool_use", ID: "toolu_abc", Name: "get_weather", Input: json.RawMessage(`{"city":"Berlin"}`)},
				},
				StopReason: "tool_use",
				Usage:      anthropicUsage{InputTokens: 30, OutputTokens: 15},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		} else {
			// Verify tool result was sent correctly
			foundToolResult := false
			for _, msg := range antReq.Messages {
				if msg.Role == "user" {
					// Check for tool_result content block
					raw, _ := json.Marshal(msg.Content)
					if strings.Contains(string(raw), "tool_result") {
						foundToolResult = true
					}
				}
			}
			if !foundToolResult {
				t.Error("tool_result not found in second request")
			}

			resp := anthropicResponse{
				ID: "msg_2", Type: "message", Role: "assistant",
				Model: "claude-3-5-sonnet-latest",
				Content: []anthropicResponseBlock{
					{Type: "text", Text: "The weather in Berlin is sunny."},
				},
				StopReason: "end_turn",
				Usage:      anthropicUsage{InputTokens: 50, OutputTokens: 12},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	transport := &anthropicTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	// First request: get tool call
	reqBody := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"Weather in Berlin?"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]}`
	resp, err := client.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	defer resp.Body.Close()

	var oaiResp openaiResponse
	json.NewDecoder(resp.Body).Decode(&oaiResp)

	if len(oaiResp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(oaiResp.Choices[0].Message.ToolCalls))
	}
	if oaiResp.Choices[0].Message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool name = %q", oaiResp.Choices[0].Message.ToolCalls[0].Function.Name)
	}

	// Second request: send tool result
	reqBody2 := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"Weather in Berlin?"},{"role":"assistant","content":"","tool_calls":[{"id":"toolu_abc","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Berlin\"}"}}]},{"role":"tool","content":"Sunny, 22°C","tool_call_id":"toolu_abc"}]}`
	resp2, err := client.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(reqBody2))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	defer resp2.Body.Close()

	var oaiResp2 openaiResponse
	json.NewDecoder(resp2.Body).Decode(&oaiResp2)

	if oaiResp2.Choices[0].Message.Content != "The weather in Berlin is sunny." {
		t.Errorf("final content = %q", oaiResp2.Choices[0].Message.Content)
	}
}

func TestAnthropicErrorTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "max_tokens: required field missing",
			},
		})
	}))
	defer server.Close()

	transport := &anthropicTransport{base: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	reqBody := `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"Hi"}]}`
	resp, err := client.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "max_tokens") {
		t.Errorf("error body doesn't mention max_tokens: %s", body)
	}
}

func TestAnthropicConversationStartsWithAssistant(t *testing.T) {
	// If messages start with assistant (e.g. after stripping system),
	// a user message should be prepended
	oai := openaiRequest{
		Model: "claude-3-5-sonnet-latest",
		Messages: marshalMessages(t, []openaiMessage{
			{Role: "system", Content: "Be helpful"},
			{Role: "assistant", Content: "I'm ready"},
			{Role: "user", Content: "Go"},
		}),
	}

	ant, err := translateOpenAIToAnthropic(oai, anthropicThinkingConfig{})
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	if ant.Messages[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", ant.Messages[0].Role)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func marshalMessages(t *testing.T, msgs []openaiMessage) []json.RawMessage {
	t.Helper()
	var result []json.RawMessage
	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal message: %v", err)
		}
		result = append(result, raw)
	}
	return result
}

func ptrFloat32(f float32) *float32 {
	return &f
}

func buildAnthropicSSE(events []string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(e)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// roundTripFunc is defined in client_test.go (same package)
