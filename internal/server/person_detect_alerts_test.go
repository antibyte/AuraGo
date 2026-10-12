package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/tools"
)

type personAlertSendLog struct {
	mu     sync.Mutex
	texts  []string
	photos []string
	images []string
	paths  []string
}

func (l *personAlertSendLog) snapshot() (texts, photos, images, paths []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.texts...), append([]string(nil), l.photos...),
		append([]string(nil), l.images...), append([]string(nil), l.paths...)
}

// stubPersonAlertSenders replaces the three senders for the test; textResult is what the
// SendNotification stub answers.
func stubPersonAlertSenders(t *testing.T, textResult string, telegramErr, discordErr error) *personAlertSendLog {
	t.Helper()
	log := &personAlertSendLog{}
	oldText, oldPhoto, oldImage := personAlertSendNotification, personAlertTelegramPhoto, personAlertDiscordImage
	t.Cleanup(func() {
		personAlertSendNotification, personAlertTelegramPhoto, personAlertDiscordImage = oldText, oldPhoto, oldImage
	})
	personAlertSendNotification = func(_ *config.Config, _ *slog.Logger, channel, title, message, priority string, _ tools.DiscordSendFunc, _ ...tools.TelnyxSendFunc) string {
		log.mu.Lock()
		defer log.mu.Unlock()
		log.texts = append(log.texts, channel+"|"+title+"|"+message+"|"+priority)
		return textResult
	}
	personAlertTelegramPhoto = func(_ context.Context, _ *config.Config, name string, photo []byte, caption string) error {
		log.mu.Lock()
		defer log.mu.Unlock()
		log.photos = append(log.photos, fmt.Sprintf("%s|%s|%d", name, caption, len(photo)))
		return telegramErr
	}
	personAlertDiscordImage = func(channelID, localPath, caption string, _ *slog.Logger) error {
		data, err := os.ReadFile(localPath)
		log.mu.Lock()
		defer log.mu.Unlock()
		log.images = append(log.images, fmt.Sprintf("%s|%s|%d|%v", channelID, caption, len(data), err == nil))
		log.paths = append(log.paths, localPath)
		return discordErr
	}
	return log
}

const personAlertSentResult = `{"status":"success","results":[{"channel":"x","status":"sent"}]}`

var personAlertTestPhoto = []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0xff, 0xd9}

func personAlertTestServer() *Server {
	cfg := &config.Config{}
	cfg.Telegram.BotToken, cfg.Telegram.UserID = "123456:alert-test", 7
	cfg.Discord.Enabled, cfg.Discord.DefaultChannelID = true, "chan-1"
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func waitPersonAlertSlotsIdle(t *testing.T, s *Server, channels ...string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		busy := 0
		for _, channel := range channels {
			busy += s.personAlertSlots.busy(channel)
		}
		if busy == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d sends still hold their slots", busy)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPersonAlertTelegramSendsOnePhotoWithCaption(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	s := personAlertTestServer()
	if err := s.sendPersonAlertMessage(context.Background(), " Telegram ", "Title", "Body", personAlertTestPhoto); err != nil {
		t.Fatal(err)
	}
	texts, photos, _, _ := log.snapshot()
	if len(texts) != 0 || len(photos) != 1 || photos[0] != "person-detection.jpg|Title\nBody|7" {
		t.Fatalf("texts %q, photos %q", texts, photos)
	}
}

// A photo the service refuses for its size or format (Telegram's 10 MB and dimension limits,
// Discord's upload limit) is replaced by the text message.
func TestPersonAlertPhotoFailureFallsBackToText(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult,
		errors.New("telegram photo: Bad Request: PHOTO_INVALID_DIMENSIONS"),
		errors.New("failed to send Discord image: HTTP 413 Request Entity Too Large"))
	s := personAlertTestServer()
	for _, channel := range []string{"telegram", "discord"} {
		if err := s.sendPersonAlertMessage(context.Background(), channel, "Title", "Body", personAlertTestPhoto); err != nil {
			t.Fatalf("%s: %v", channel, err)
		}
	}
	texts, photos, images, _ := log.snapshot()
	if len(photos) != 1 || len(images) != 1 || strings.Join(texts, ";") != "telegram|Title|Body|high;discord|Title|Body|high" {
		t.Fatalf("texts %q, photos %q, images %q", texts, photos, images)
	}
}

// Adaptation: a photo that fails because the message deadline passed gets no late text
// fallback; the action has been reported as a timeout already.
func TestPersonAlertNoTextFallbackAfterTheDeadline(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	personAlertTelegramPhoto = func(ctx context.Context, _ *config.Config, _ string, _ []byte, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}
	s := personAlertTestServer()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := s.sendPersonAlertMessage(ctx, "telegram", "Title", "Body", personAlertTestPhoto); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}
	waitPersonAlertSlotsIdle(t, s, "telegram")
	if texts, _, _, _ := log.snapshot(); len(texts) != 0 {
		t.Fatalf("a late text went out: %q", texts)
	}
}

func TestPersonAlertDiscordPhotoUsesARemovedTempFile(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	s := personAlertTestServer()
	if err := s.sendPersonAlertMessage(context.Background(), "discord", "Title", "Body", personAlertTestPhoto); err != nil {
		t.Fatal(err)
	}
	texts, _, images, paths := log.snapshot()
	if len(texts) != 0 || len(images) != 1 || images[0] != "chan-1|Title\nBody|7|true" {
		t.Fatalf("texts %q, images %q", texts, images)
	}
	if !strings.HasSuffix(paths[0], "person-detection.jpg") {
		t.Fatalf("photo path = %q", paths[0])
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatalf("temp photo %s still exists: %v", paths[0], err)
	}
}

func TestPersonAlertAllSendsTextAndPhotos(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	s := personAlertTestServer()
	if err := s.sendPersonAlertMessage(context.Background(), "", "Title", "Body", personAlertTestPhoto); err != nil {
		t.Fatal(err)
	}
	texts, photos, images, _ := log.snapshot()
	if strings.Join(texts, ";") != "all|Title|Body|high" || len(photos) != 1 || photos[0] != "person-detection.jpg|Title|7" ||
		len(images) != 1 || images[0] != "chan-1|Title|7|true" {
		t.Fatalf("texts %q, photos %q, images %q", texts, photos, images)
	}
	// Without Telegram and Discord the photo goes nowhere; the text still goes to "all".
	s.Cfg.Telegram.BotToken = ""
	s.Cfg.Discord.Enabled = false
	if err := s.sendPersonAlertMessage(context.Background(), "all", "Title", "Body", personAlertTestPhoto); err != nil {
		t.Fatal(err)
	}
	texts, photos, images, _ = log.snapshot()
	if len(texts) != 2 || len(photos) != 1 || len(images) != 1 {
		t.Fatalf("texts %q, photos %q, images %q", texts, photos, images)
	}
}

func TestPersonAlertTextChannelsIgnoreThePhoto(t *testing.T) {
	log := stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	s := personAlertTestServer()
	if err := s.sendPersonAlertMessage(context.Background(), "ntfy", "Title", "Body", personAlertTestPhoto); err != nil {
		t.Fatal(err)
	}
	texts, photos, images, _ := log.snapshot()
	if strings.Join(texts, ";") != "ntfy|Title|Body|high" || len(photos)+len(images) != 0 {
		t.Fatalf("texts %q, photos %q, images %q", texts, photos, images)
	}
}

func TestPersonAlertUnavailableChannelsAreSkipped(t *testing.T) {
	stubPersonAlertSenders(t, personAlertSentResult, nil, nil)
	s := personAlertTestServer()
	s.Cfg.Telegram.UserID = 0
	s.Cfg.Discord.ReadOnly = true
	for _, channel := range []string{"telegram", "discord", "pager"} {
		var skip *persondetect.SkipError
		if err := s.sendPersonAlertMessage(context.Background(), channel, "T", "B", nil); !errors.As(err, &skip) || skip.Reason != "channel unavailable" {
			t.Fatalf("%s: err = %v", channel, err)
		}
	}
}

func TestPersonAlertOutcome(t *testing.T) {
	cases := []struct {
		raw, want string
		skipped   bool
	}{
		{`{"status":"success","results":[{"channel":"ntfy","status":"error","detail":"down"},{"channel":"push","status":"sent"}]}`, "", false},
		{`{"status":"success","results":[{"channel":"ntfy","status":"error","detail":"ntfy is not enabled in config"}]}`, "ntfy: ntfy is not enabled in config", false},
		{`{"status":"error","message":"no notification channels are enabled"}`, "channel unavailable", true},
		{`{"status":"error","message":"message is required"}`, "message is required", false},
		{`not json`, "unexpected notification result", false},
		// Adaptation: the detail reaches the event row and the agent, so URLs (an ntfy topic
		// is a secret) are cut out of it.
		{`{"status":"success","results":[{"channel":"ntfy","status":"error","detail":"ntfy request failed: Post \"https://ntfy.example/secret-topic\": dial tcp: timeout"}]}`,
			`ntfy: ntfy request failed: Post "[url]": dial tcp: timeout`, false},
	}
	for _, c := range cases {
		err := personAlertOutcome(c.raw)
		var skip *persondetect.SkipError
		switch {
		case c.want == "":
			if err != nil {
				t.Errorf("%s: err = %v, want sent", c.raw, err)
			}
		case c.skipped:
			if !errors.As(err, &skip) || skip.Reason != c.want {
				t.Errorf("%s: err = %v, want skipped %q", c.raw, err, c.want)
			}
		default:
			if err == nil || err.Error() != c.want || errors.As(err, &skip) {
				t.Errorf("%s: err = %v, want failure %q", c.raw, err, c.want)
			}
		}
	}
}

func TestPersonAlertMessagesAreBoundedPerChannel(t *testing.T) {
	started, unblock := make(chan string, 16), make(chan struct{})
	old := personAlertSendNotification
	t.Cleanup(func() { personAlertSendNotification = old })
	personAlertSendNotification = func(_ *config.Config, _ *slog.Logger, channel, _, _, _ string, _ tools.DiscordSendFunc, _ ...tools.TelnyxSendFunc) string {
		started <- channel
		<-unblock
		return personAlertSentResult
	}
	s := personAlertTestServer()
	var wg sync.WaitGroup
	for range personAlertMaxInFlight {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.sendPersonAlertMessage(context.Background(), "ntfy", "T", "B", nil)
		}()
	}
	for range personAlertMaxInFlight {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("the ntfy sends did not start")
		}
	}
	if err := s.sendPersonAlertMessage(context.Background(), "ntfy", "T", "B", nil); err == nil || err.Error() != "busy" {
		t.Fatalf("fifth ntfy send = %v, want busy", err)
	}
	go func() { _ = s.sendPersonAlertMessage(context.Background(), "pushover", "T", "B", nil) }()
	select {
	case channel := <-started:
		if channel != "pushover" {
			t.Fatalf("started %q, want pushover", channel)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("busy ntfy slots blocked pushover")
	}
	close(unblock)
	wg.Wait()
	waitPersonAlertSlotsIdle(t, s, "ntfy", "pushover")
}

func TestPersonAlertMessageHonorsTheDeadline(t *testing.T) {
	unblock := make(chan struct{})
	old := personAlertSendNotification
	t.Cleanup(func() { personAlertSendNotification = old })
	personAlertSendNotification = func(*config.Config, *slog.Logger, string, string, string, string, tools.DiscordSendFunc, ...tools.TelnyxSendFunc) string {
		<-unblock
		return personAlertSentResult
	}
	s := personAlertTestServer()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := s.sendPersonAlertMessage(ctx, "ntfy", "T", "B", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}
	if busy := s.personAlertSlots.busy("ntfy"); busy != 1 {
		t.Fatalf("a stalled send must keep its slot, busy = %d", busy)
	}
	close(unblock)
	waitPersonAlertSlotsIdle(t, s, "ntfy")
}
