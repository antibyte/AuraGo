package rocketchat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"aurago/ui"

	"github.com/sashabaranov/go-openai"
)

func TestRocketChatSessionIDIsScopedToRoomAndSender(t *testing.T) {
	got := rocketChatSessionID("room-1", "user-9")
	if got != "rocketchat:room-1:user-9" {
		t.Fatalf("session id = %q", got)
	}
	if rocketChatSessionID("room-1", "") == rocketChatSessionID("room-1", "user-9") {
		t.Fatal("different senders must not share a session")
	}
	if rocketChatSessionID("room-2", "user-9") == got {
		t.Fatal("different rooms must not share a session")
	}
}

// Username-only senders never collapse into a shared "rocketchat:<room>:" key.
func TestRocketChatSenderKeyFallsBackToUsername(t *testing.T) {
	for _, tc := range []struct {
		id, username, want string
	}{
		{"user-9", "alice", "user-9"},
		{"", "alice", "username:alice"},
		{"", "bob", "username:bob"},
		{"", "", ""},
	} {
		msg := message{}
		msg.User.ID, msg.User.Username = tc.id, tc.username
		if got := rocketChatSenderKey(msg); got != tc.want {
			t.Fatalf("rocketChatSenderKey(id=%q, username=%q) = %q, want %q", tc.id, tc.username, got, tc.want)
		}
	}
}

// sessionTestClient answers every completion with a fixed reply and records
// every conversation it was shown.
type sessionTestClient struct {
	mu       sync.Mutex
	requests [][]openai.ChatCompletionMessage
}

func (c *sessionTestClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.mu.Lock()
	c.requests = append(c.requests, append([]openai.ChatCompletionMessage(nil), req.Messages...))
	c.mu.Unlock()
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "rc-reply"},
		FinishReason: openai.FinishReasonStop,
	}}}, nil
}

func (c *sessionTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming is not used by the Rocket.Chat bot")
}

func (c *sessionTestClient) sawTogether(a, b string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, messages := range c.requests {
		var all strings.Builder
		for _, m := range messages {
			all.WriteString(m.Content)
			all.WriteByte('\n')
		}
		if strings.Contains(all.String(), a) && strings.Contains(all.String(), b) {
			return true
		}
	}
	return false
}

func (c *sessionTestClient) saw(text string) bool {
	return c.sawTogether(text, text)
}

func sessionContents(t *testing.T, stm *memory.SQLiteMemory, sessionID string) []openai.ChatCompletionMessage {
	t.Helper()
	messages, err := stm.GetRecentMessages(sessionID, 100)
	if err != nil {
		t.Fatalf("load %s: %v", sessionID, err)
	}
	return messages
}

func hasMessage(messages []openai.ChatCompletionMessage, role, text string) bool {
	for _, m := range messages {
		if m.Role == role && strings.Contains(m.Content, text) {
			return true
		}
	}
	return false
}

// Two allow-listed senders in one room each get their own conversation; the
// owner's shared "default" conversation is neither read nor written, and
// /reset from one sender clears only that sender's conversation.
func TestRocketChatProcessMessageKeepsEachSenderInItsOwnSession(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	i18n.Load(ui.Content, logger)

	var mu sync.Mutex
	var posts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message struct {
				Msg string `json:"msg"`
			} `json:"message"`
		}
		if r.URL.Path == "/api/v1/chat.sendMessage" && json.NewDecoder(r.Body).Decode(&body) == nil {
			mu.Lock()
			posts = append(posts, body.Message.Msg)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	historyManager := memory.NewEphemeralHistoryManager()
	defer historyManager.Close()

	// The owner's web/Telegram conversation lives in "default".
	const ownerText = "owner-private-web-turn"
	ownerID, err := stm.InsertMessage("default", openai.ChatMessageRoleUser, ownerText, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := historyManager.Add(openai.ChatMessageRoleUser, ownerText, ownerID, false, false); err != nil {
		t.Fatal(err)
	}

	promptsDir, err := filepath.Abs("../../prompts")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Agent.SystemLanguage = "English"
	cfg.Agent.ContextWindow = 32768
	cfg.Server.UILanguage = "en"
	cfg.Directories.PromptsDir = promptsDir
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.RocketChat.Enabled = true
	cfg.RocketChat.URL = srv.URL
	cfg.RocketChat.AllowedUsers = []string{"id:alice", "id:bob"}

	client := &sessionTestClient{}
	registry := tools.NewProcessRegistry(logger)
	send := func(userID, text string) {
		t.Helper()
		msg := message{ID: userID + "-" + text, Msg: text}
		msg.User.ID = userID
		msg.User.Username = userID
		processMessage(context.Background(), cfg, logger, client, stm, nil, nil, registry, nil, historyManager, nil, nil, "room-1", msg, nil, nil, nil)
	}

	const aliceText, bobText = "alice-question-one", "bob-question-two"
	send("alice", aliceText)
	send("bob", bobText)

	aliceSession := rocketChatSessionID("room-1", "alice")
	bobSession := rocketChatSessionID("room-1", "bob")
	alice := sessionContents(t, stm, aliceSession)
	bob := sessionContents(t, stm, bobSession)
	if !hasMessage(alice, openai.ChatMessageRoleUser, aliceText) || !hasMessage(alice, openai.ChatMessageRoleAssistant, "rc-reply") {
		t.Fatalf("alice session lacks her turn and the reply: %#v", alice)
	}
	if !hasMessage(bob, openai.ChatMessageRoleUser, bobText) || !hasMessage(bob, openai.ChatMessageRoleAssistant, "rc-reply") {
		t.Fatalf("bob session lacks his turn and the reply: %#v", bob)
	}
	if hasMessage(alice, openai.ChatMessageRoleUser, bobText) || hasMessage(bob, openai.ChatMessageRoleUser, aliceText) {
		t.Fatalf("senders share a session: alice=%#v bob=%#v", alice, bob)
	}

	owner := sessionContents(t, stm, "default")
	if len(owner) != 1 || owner[0].Content != ownerText {
		t.Fatalf("Rocket.Chat wrote into the default session: %#v", owner)
	}
	if web := historyManager.Get(); len(web) != 1 || web[0].Content != ownerText {
		t.Fatalf("Rocket.Chat wrote into the owner's history: %#v", web)
	}
	if client.saw(ownerText) {
		t.Fatal("the model saw the owner's default conversation while answering Rocket.Chat")
	}
	if client.sawTogether(aliceText, bobText) {
		t.Fatal("the model saw alice's turn while answering bob")
	}
	mu.Lock()
	sent := append([]string(nil), posts...)
	mu.Unlock()
	replies := 0
	for _, text := range sent {
		if text == "rc-reply" {
			replies++
		}
	}
	if replies != 2 || len(sent) != 2 {
		t.Fatalf("the bot posted %q, want the reply exactly once per message", sent)
	}

	send("alice", "/reset")
	if left := sessionContents(t, stm, aliceSession); len(left) != 0 {
		t.Fatalf("/reset left alice's session intact: %#v", left)
	}
	if !hasMessage(sessionContents(t, stm, bobSession), openai.ChatMessageRoleUser, bobText) {
		t.Fatal("/reset from alice cleared bob's session")
	}
	if owner := sessionContents(t, stm, "default"); len(owner) != 1 || owner[0].Content != ownerText {
		t.Fatalf("/reset from Rocket.Chat cleared the default session: %#v", owner)
	}
	if web := historyManager.Get(); len(web) != 1 || web[0].Content != ownerText {
		t.Fatalf("/reset from Rocket.Chat cleared the owner's history: %#v", web)
	}
}
