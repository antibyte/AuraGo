package telegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
)

func photoTestConfig(token string) *config.Config {
	cfg := &config.Config{}
	cfg.Telegram.BotToken = token
	cfg.Telegram.UserID = 7
	return cfg
}

func usePhotoEndpoint(t *testing.T, base string) {
	t.Helper()
	previous := photoAPIEndpoint
	photoAPIEndpoint = base + "/bot%s/%s"
	t.Cleanup(func() { photoAPIEndpoint = previous })
}

type photoUpload struct {
	chatID, caption, filename string
	data                      []byte
}

// photoServer answers getMe and sendPhoto like the Bot API and hands every upload to the test.
func photoServer(t *testing.T, token, sendReply string) (*httptest.Server, <-chan photoUpload) {
	t.Helper()
	uploads := make(chan photoUpload, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bot" + token + "/getMe":
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"test","username":"testbot"}}`)
		case "/bot" + token + "/sendPhoto":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("multipart: %v", err)
				return
			}
			file, header, err := r.FormFile("photo")
			if err != nil {
				t.Errorf("photo part: %v", err)
				return
			}
			data, _ := io.ReadAll(file)
			uploads <- photoUpload{chatID: r.FormValue("chat_id"), caption: r.FormValue("caption"), filename: header.Filename, data: data}
			_, _ = io.WriteString(w, sendReply)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, uploads
}

const photoOKReply = `{"ok":true,"result":{"message_id":5,"date":1,"chat":{"id":7,"type":"private"}}}`

func TestSendPhotoUploadsBytesWithCaption(t *testing.T) {
	const token = "123456:photo-test-token"
	server, uploads := photoServer(t, token, photoOKReply)
	usePhotoEndpoint(t, server.URL)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0xff, 0xd9}
	caption := "Person erkannt – Garten\nGarten (Einfahrt), 21:14 Uhr, 91 %"
	if err := SendPhoto(context.Background(), photoTestConfig(token), "person-detection.jpg", jpeg, caption); err != nil {
		t.Fatal(err)
	}
	got := <-uploads
	if got.chatID != "7" || got.caption != caption || got.filename != "person-detection.jpg" || string(got.data) != string(jpeg) {
		t.Fatalf("upload = %+v", got)
	}
}

func TestSendPhotoBoundsTheCaption(t *testing.T) {
	const token = "123456:photo-test-token"
	server, uploads := photoServer(t, token, photoOKReply)
	usePhotoEndpoint(t, server.URL)
	if err := SendPhoto(context.Background(), photoTestConfig(token), "", []byte{1, 2, 3}, strings.Repeat("ü", 2000)); err != nil {
		t.Fatal(err)
	}
	got := <-uploads
	if n := utf8.RuneCountInString(got.caption); n != photoCaptionRunes || !strings.HasSuffix(got.caption, "…") {
		t.Fatalf("caption has %d runes", n)
	}
	if got.filename != "photo.jpg" {
		t.Fatalf("default file name = %q", got.filename)
	}
}

func TestSendPhotoRefusesMissingConfigurationAndPhoto(t *testing.T) {
	if err := SendPhoto(context.Background(), &config.Config{}, "a.jpg", []byte{1}, "x"); err == nil {
		t.Fatal("unconfigured Telegram must fail")
	}
	if err := SendPhoto(context.Background(), photoTestConfig("123456:photo-test-token"), "a.jpg", nil, "x"); err == nil {
		t.Fatal("an empty photo must fail")
	}
	if err := SendPhoto(context.Background(), nil, "a.jpg", []byte{1}, "x"); err == nil {
		t.Fatal("a nil config must fail")
	}
}

func TestSendPhotoReportsRejectionsWithoutTheToken(t *testing.T) {
	const token = "123456:never-show-this-token"
	server, _ := photoServer(t, token, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`)
	usePhotoEndpoint(t, server.URL)
	err := SendPhoto(context.Background(), photoTestConfig(token), "a.jpg", []byte{1}, "x")
	if err == nil || !strings.Contains(err.Error(), "chat not found") || strings.Contains(err.Error(), "never-show-this-token") {
		t.Fatalf("rejected upload err = %v", err)
	}

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	usePhotoEndpoint(t, closed.URL)
	err = SendPhoto(context.Background(), photoTestConfig(token), "a.jpg", []byte{1}, "x")
	if err == nil || strings.Contains(err.Error(), "never-show-this-token") {
		t.Fatalf("network error = %v", err)
	}
}
