package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestProcessUpdateReturnsBeforeStorageWhenAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	processUpdate(ctx, nil, tgbotapi.Update{}, nil, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestConvertOggToMp3ContextReturnsCanceledBeforeStarting(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "existing.mp3")
	if err := os.WriteFile(outputPath, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ConvertOggToMp3Context(ctx, "missing.ogg", outputPath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ConvertOggToMp3Context error = %v, want context.Canceled", err)
	}
	if got, err := os.ReadFile(outputPath); err != nil || string(got) != "preserve" {
		t.Fatalf("pre-existing output after canceled conversion = %q, err=%v", got, err)
	}
}

func TestConvertToOGGContextReturnsCanceledBeforeStarting(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "existing.mp3")
	outputPath := inputPath + ".ogg"
	if err := os.WriteFile(outputPath, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := convertToOGGContext(ctx, inputPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("convertToOGGContext error = %v, want context.Canceled", err)
	}
	if got, err := os.ReadFile(outputPath); err != nil || string(got) != "preserve" {
		t.Fatalf("pre-existing output after canceled conversion = %q, err=%v", got, err)
	}
}

func TestDownloadFileHonorsCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := downloadFile(ctx, server.URL, nil)
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("download request did not start")
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("downloadFile error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("downloadFile did not return after cancellation")
	}
}

func TestTelegramSDKRequestBodyHonorsParentCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	serverCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest-token/getMe":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"test","username":"testbot"}}`)
		case "/bottest-token/getFile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"ok":true,"result":`)
			w.(http.Flusher).Flush()
			close(requestStarted)
			<-r.Context().Done()
			close(serverCanceled)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	endpoint := fmt.Sprintf("%s/bot%%s/%%s", server.URL)
	bot, err := tgbotapi.NewBotAPIWithClient("test-token", endpoint, contextBoundHTTPClient{parent: parent, client: &http.Client{}})
	if err != nil {
		t.Fatalf("construct Telegram bot API: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := bot.GetFile(tgbotapi.FileConfig{FileID: "file-id"})
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("Telegram SDK request did not start")
	}
	cancel()
	select {
	case <-serverCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("parent cancellation did not cancel the in-flight SDK response body")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("GetFile succeeded after its response body was canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetFile did not return after parent cancellation")
	}
}
