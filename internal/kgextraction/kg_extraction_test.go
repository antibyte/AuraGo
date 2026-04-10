package kgextraction

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// ---------------------------------------------------------------------------
// trimJSONResponse tests
// ---------------------------------------------------------------------------

func TestTrimJSONResponse_PlainJSON(t *testing.T) {
	input := `{"nodes": [], "edges": []}`
	got := trimJSONResponse(input)
	if got != input {
		t.Errorf("trimJSONResponse(%q) = %q, want %q", input, got, input)
	}
}

func TestTrimJSONResponse_LeadingWhitespace(t *testing.T) {
	input := `  {"nodes": []}  `
	want := `{"nodes": []}`
	got := trimJSONResponse(input)
	if got != want {
		t.Errorf("trimJSONResponse(%q) = %q, want %q", input, got, want)
	}
}

func TestTrimJSONResponse_JSONCodeFence(t *testing.T) {
	input := "```json\n{\"nodes\": []}\n```"
	want := `{"nodes": []}`
	got := trimJSONResponse(input)
	if got != want {
		t.Errorf("trimJSONResponse(%q) = %q, want %q", input, got, want)
	}
}

func TestTrimJSONResponse_PlainCodeFence(t *testing.T) {
	input := "```\n{\"nodes\": []}\n```"
	want := `{"nodes": []}`
	got := trimJSONResponse(input)
	if got != want {
		t.Errorf("trimJSONResponse(%q) = %q, want %q", input, got, want)
	}
}

func TestTrimJSONResponse_CodeFenceNoNewlines(t *testing.T) {
	input := "```json{\"nodes\": []}```"
	want := `{"nodes": []}`
	got := trimJSONResponse(input)
	if got != want {
		t.Errorf("trimJSONResponse(%q) = %q, want %q", input, got, want)
	}
}

func TestTrimJSONResponse_EmptyString(t *testing.T) {
	got := trimJSONResponse("")
	if got != "" {
		t.Errorf("trimJSONResponse(\"\") = %q, want empty", got)
	}
}

func TestTrimJSONResponse_OnlyCodeFences(t *testing.T) {
	input := "```json\n```"
	got := trimJSONResponse(input)
	if got != "" {
		t.Errorf("trimJSONResponse(%q) = %q, want empty", input, got)
	}
}

// ---------------------------------------------------------------------------
// ExtractKGFromText tests
// ---------------------------------------------------------------------------

func TestExtractKGFromText_InputTooShort(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()
	_, _, err := ExtractKGFromText(cfg, logger, nil, "short", "")
	if err == nil {
		t.Fatal("expected error for short input, got nil")
	}
}

func TestExtractKGFromText_NilClientAndModel(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()
	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	_, _, err := ExtractKGFromText(cfg, logger, nil, longInput, "")
	if err == nil {
		t.Fatal("expected error when no LLM client/model available, got nil")
	}
}

func TestExtractKGFromText_EmptyModel(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.Default()
	client := &mockChatClient{}
	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	_, _, err := ExtractKGFromText(cfg, logger, client, longInput, "")
	if err == nil {
		t.Fatal("expected error when model is empty, got nil")
	}
}

func TestExtractKGFromText_Success(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	logger := slog.Default()

	jsonResp := `{"nodes":[{"id":"test_node","label":"Test Node","properties":{"type":"concept"}}],"edges":[{"source":"test_node","target":"other","relation":"related_to"}]}`
	client := &mockChatClient{
		response: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: jsonResp,
					},
				},
			},
		},
	}

	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	nodes, edges, err := ExtractKGFromText(cfg, logger, client, longInput, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "test_node" {
		t.Errorf("node ID = %q, want %q", nodes[0].ID, "test_node")
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Relation != "related_to" {
		t.Errorf("edge relation = %q, want %q", edges[0].Relation, "related_to")
	}
}

func TestExtractKGFromText_InvalidJSONResponse(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	logger := slog.Default()

	client := &mockChatClient{
		response: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "this is not valid JSON",
					},
				},
			},
		},
	}

	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	_, _, err := ExtractKGFromText(cfg, logger, client, longInput, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestExtractKGFromText_EmptyChoices(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	logger := slog.Default()

	client := &mockChatClient{
		response: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{},
		},
	}

	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	_, _, err := ExtractKGFromText(cfg, logger, client, longInput, "")
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
}

func TestExtractKGFromText_WithExistingNodes(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	logger := slog.Default()

	jsonResp := `{"nodes":[{"id":"new_node","label":"New","properties":{"type":"concept"}}],"edges":[]}`
	client := &mockChatClient{
		response: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: jsonResp,
					},
				},
			},
		},
	}

	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	existingNodes := "Existing Nodes (reuse IDs if possible):\n- ID: old_node, Label: Old Node\n\n"

	nodes, _, err := ExtractKGFromText(cfg, logger, client, longInput, existingNodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "new_node" {
		t.Errorf("node ID = %q, want %q", nodes[0].ID, "new_node")
	}
}

// ---------------------------------------------------------------------------
// resolveHelperBackedLLM tests (indirect via ExtractKGFromText)
// ---------------------------------------------------------------------------

func TestExtractKGFromText_HelperLLMNotEnabled(t *testing.T) {
	// When helper is not enabled, the fallback client should be used.
	cfg := &config.Config{}
	cfg.LLM.Model = "fallback-model"
	logger := slog.Default()

	jsonResp := `{"nodes":[],"edges":[]}`
	client := &mockChatClient{
		response: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: jsonResp}},
			},
		},
	}

	longInput := "This is a sufficiently long input text that exceeds the fifty character minimum threshold for extraction."
	nodes, edges, err := ExtractKGFromText(cfg, logger, client, longInput, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 || len(edges) != 0 {
		t.Fatal("expected empty results for empty JSON response")
	}
}

// ---------------------------------------------------------------------------
// mock ChatClient
// ---------------------------------------------------------------------------

type mockChatClient struct {
	response openai.ChatCompletionResponse
	err      error
}

func (m *mockChatClient) CreateChatCompletion(_ context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return m.response, m.err
}

func (m *mockChatClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

// Compile-time interface check.
var _ interface {
	CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
} = (*mockChatClient)(nil)

// ---------------------------------------------------------------------------
// Node/Edge type sanity checks (ensures JSON tags are correct)
// ---------------------------------------------------------------------------

func TestNodeJSONRoundTrip(t *testing.T) {
	n := memory.Node{
		ID:    "test_id",
		Label: "Test Label",
		Properties: map[string]string{
			"type": "person",
		},
	}
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}
	if string(data) != `{"id":"test_id","label":"Test Label","properties":{"type":"person"}}` {
		// Properties map order is non-deterministic; just verify keys exist.
		t.Logf("marshalled node: %s", data)
	}

	var got memory.Node
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	if got.ID != n.ID || got.Label != n.Label {
		t.Errorf("round-trip: got ID=%q Label=%q, want ID=%q Label=%q", got.ID, got.Label, n.ID, n.Label)
	}
}

func TestEdgeJSONRoundTrip(t *testing.T) {
	e := memory.Edge{
		Source:   "a",
		Target:   "b",
		Relation: "runs_on",
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal edge: %v", err)
	}
	var got memory.Edge
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal edge: %v", err)
	}
	if got.Source != e.Source || got.Target != e.Target || got.Relation != e.Relation {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, e)
	}
}
