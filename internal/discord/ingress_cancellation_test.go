package discord

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordStopSkipsAttachmentsWithSaturatedWorkers(t *testing.T) {
	for _, tc := range []struct {
		text   string
		author string
	}{
		{"/stop", "owner"},
		{"<@bot> /stop", "owner"},
		{"<@!bot> /stop", "owner"},
		{"/stop", "stranger"},
	} {
		t.Run(tc.author+tc.text, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			store, err := memory.NewSQLiteMemory(":memory:", logger)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			cfg := &config.Config{}
			cfg.Discord.AllowedUserID, cfg.Discord.DefaultChannelID = "owner", "channel"
			recorder := newDiscordRESTRecorder(t)
			recorder.session.State.User = &discordgo.User{ID: "bot"}
			message := &discordgo.MessageCreate{Message: &discordgo.Message{
				Author: &discordgo.User{ID: tc.author}, GuildID: "guild", ChannelID: "channel", Content: tc.text,
				Mentions:    []*discordgo.User{{ID: "bot"}},
				Attachments: []*discordgo.MessageAttachment{{Filename: "voice.ogg", ContentType: "audio/ogg"}},
			}}
			sessionID, err := store.ResolveUserSession(context.Background(), discordConversationID(message))
			if err != nil {
				t.Fatal(err)
			}
			active, release, err := store.RegisterSessionRun(context.Background(), sessionID)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			previous := discordAttachmentProcessor
			t.Cleanup(func() { discordAttachmentProcessor = previous })
			processed := 0
			discordAttachmentProcessor = func(_ context.Context, _ *discordgo.MessageAttachment, text string, _ *config.Config, _ *slog.Logger) string {
				processed++
				return text
			}
			slots := newDiscordWorkerSlots(1)
			slots <- struct{}{}
			stop := commands.IsStop(discordCommandText(message.Content, "bot", true))
			if !runDiscordMessage(context.Background(), slots, 1, recorder.session, "channel", logger, stop, func() {
				handleMessage(recorder.session, message, cfg, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			}) {
				t.Fatal("stop was rejected by the occupied worker pool")
			}
			if processed != 0 {
				t.Errorf("stop performed %d attachment preprocessing calls", processed)
			}
			if got, want := active.Err() == context.Canceled, tc.author == "owner"; got != want {
				t.Errorf("session cancelled=%v, want %v", got, want)
			}
			if len(slots) != 1 {
				t.Fatal("stop changed worker occupancy")
			}
		})
	}
}

func TestDiscordSessionStopCancelsMediaBeforeHistoryOrExecution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	started := make(chan struct{})
	requestCancelled := make(chan struct{})
	releaseFixture := make(chan struct{})
	var secondDownloads atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/second" {
			secondDownloads.Add(1)
			return
		}
		close(started)
		select {
		case <-r.Context().Done():
			close(requestCancelled)
		case <-releaseFixture:
		}
	}))
	defer provider.Close()
	defer close(releaseFixture)
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID, cfg.Discord.DefaultChannelID = "user", "channel"
	message := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author: &discordgo.User{ID: "user"}, GuildID: "guild", ChannelID: "channel", Content: "describe the images",
		Attachments: []*discordgo.MessageAttachment{
			{Filename: "first.png", ContentType: "image/png", URL: provider.URL + "/first"},
			{Filename: "second.png", ContentType: "image/png", URL: provider.URL + "/second"},
		},
	}}
	sessionID, err := store.ResolveUserSession(context.Background(), discordConversationID(message))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		handleMessage(nil, message, cfg, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Discord attachment download did not start")
	}
	store.CancelSessionRuns(sessionID)
	select {
	case <-requestCancelled:
	case <-time.After(time.Second):
		t.Fatal("Session Stop did not reach the active media request")
	}
	select {
	case panicValue := <-done:
		if panicValue != nil {
			t.Fatalf("cancelled preprocessing continued into Discord/agent work: %v", panicValue)
		}
	case <-time.After(time.Second):
		t.Fatal("Discord continued waiting after Session Stop")
	}
	if secondDownloads.Load() != 0 {
		t.Fatal("cancelled ingress downloaded the next attachment")
	}
	messages, err := store.GetRecentMessages(sessionID, 10)
	if err != nil || len(messages) != 0 {
		t.Fatalf("cancelled media became a new history turn: %+v %v", messages, err)
	}
}

func TestDiscordQuestionReplyBypassesWaitingTurnGate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID, cfg.Discord.DefaultChannelID = "user", "channel"
	message := &discordgo.MessageCreate{Message: &discordgo.Message{Author: &discordgo.User{ID: "user"}, GuildID: "guild", ChannelID: "channel", Content: "continue"}}
	sessionID, err := store.ResolveUserSession(context.Background(), discordConversationID(message))
	if err != nil {
		t.Fatal(err)
	}
	releaseTurn, err := store.LockSessionTurn(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseTurn()
	answer := tools.RegisterQuestion(sessionID, &tools.PendingQuestion{Question: "Continue?", AllowFreeText: true})
	defer tools.CancelQuestion(sessionID)
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		handleMessage(nil, message, cfg, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	}()
	select {
	case response := <-answer:
		if response.Source != tools.QuestionSourceDiscord || response.FreeText != "continue" {
			t.Fatalf("Discord reply changed: %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("question answer waited behind the turn that needed it")
	}
	select {
	case panicValue := <-done:
		if panicValue != nil {
			t.Fatalf("question answer entered new-turn processing: %v", panicValue)
		}
	case <-time.After(time.Second):
		t.Fatal("question answer did not finish")
	}
	if messages, err := store.GetRecentMessages(sessionID, 10); err != nil || len(messages) != 0 {
		t.Fatalf("question answer created a new history turn: %+v %v", messages, err)
	}
}
