package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

type fakeDesktopChessChatClient struct {
	responses []string
	requests  []openai.ChatCompletionRequest
	err       error
}

func (f *fakeDesktopChessChatClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return openai.ChatCompletionResponse{}, f.err
	}
	if len(f.responses) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("no fake chess response")
	}
	content := f.responses[0]
	f.responses = f.responses[1:]
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{Content: content},
		}},
	}, nil
}

func (f *fakeDesktopChessChatClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming is not used by chess agent move")
}

func TestDesktopChessAgentMoveRejectsInvalidLegalMovesBeforeLLM(t *testing.T) {
	fake := &fakeDesktopChessChatClient{responses: []string{`{"move":"e2e4"}`}}
	s := &Server{
		Cfg:       &config.Config{},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		LLMClient: fake,
	}

	for _, body := range []string{
		`{"fen":"start","legal_moves":[],"side_to_move":"w","move_number":1,"player_color":"white"}`,
		`{"fen":"start","legal_moves":["e9e4"],"side_to_move":"w","move_number":1,"player_color":"white"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/desktop/chess/agent-move", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handleDesktopChessAgentMove(s).ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid request to return 400, got %d with body %s", rec.Code, rec.Body.String())
		}
	}
	if len(fake.requests) != 0 {
		t.Fatalf("invalid chess agent requests must not reach LLM, got %d calls", len(fake.requests))
	}
}

func TestDesktopChessAgentMoveRetriesAndAcceptsOnlyListedMove(t *testing.T) {
	fake := &fakeDesktopChessChatClient{
		responses: []string{
			`{"move":"h7h5","comment":"not legal here"}`,
			`{"move":"e2e4","comment":"Take the center."}`,
		},
	}
	s := &Server{
		Cfg:       &config.Config{},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		LLMClient: fake,
	}
	body := `{"fen":"rn1qkbnr/ppp1pppp/8/3p4/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 2","pgn":"1... d5","legal_moves":["e2e4","g1f3"],"side_to_move":"w","move_number":2,"player_color":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/chess/agent-move", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleDesktopChessAgentMove(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Move    string `json:"move"`
		Comment string `json:"comment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode chess response: %v", err)
	}
	if payload.Move != "e2e4" || payload.Comment == "" {
		t.Fatalf("response = %+v, want e2e4 with comment", payload)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("expected one retry after invalid LLM move, got %d calls", len(fake.requests))
	}
	if len(fake.requests[0].Tools) != 0 {
		t.Fatal("chess agent move request must not expose tool definitions")
	}
	userPrompt := fake.requests[0].Messages[len(fake.requests[0].Messages)-1].Content
	for _, want := range []string{"<external_data", "legal_moves", "e2e4", "g1f3"} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("chess agent prompt missing safe context marker %q in %q", want, userPrompt)
		}
	}
}

func TestDesktopChessAgentMoveReturnsBadGatewayWhenLLMNeverChoosesLegalMove(t *testing.T) {
	fake := &fakeDesktopChessChatClient{
		responses: []string{
			`{"move":"a7a5"}`,
			`{"move":"h7h5"}`,
		},
	}
	s := &Server{
		Cfg:       &config.Config{},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		LLMClient: fake,
	}
	body := `{"fen":"start","legal_moves":["e2e4"],"side_to_move":"w","move_number":1,"player_color":"white"}`
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/chess/agent-move", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleDesktopChessAgentMove(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 after invalid retry, got %d with body %s", rec.Code, rec.Body.String())
	}
	if len(fake.requests) != 2 {
		t.Fatalf("expected exactly one retry, got %d calls", len(fake.requests))
	}
}
