package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func awaitTelegram[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("Telegram lifecycle did not complete")
		var zero T
		return zero
	}
}

func TestTelegramReloadDrainsWorkersAndPreservesVerifiedBotCursor(t *testing.T) {
	type poll struct {
		offset    int
		reply     chan []tgbotapi.Update
		cancelled chan struct{}
	}
	polls := make(chan poll, 10)
	var active, peak atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			id := 11
			if strings.Contains(r.URL.Path, "other") {
				id = 22
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": id, "is_bot": true, "username": "fixture"}})
		case strings.HasSuffix(r.URL.Path, "/deleteWebhook"):
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			offset, _ := strconv.Atoi(r.Form.Get("offset"))
			p := poll{offset, make(chan []tgbotapi.Update, 1), make(chan struct{})}
			polls <- p
			select {
			case updates := <-p.reply:
				active.Add(-1)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
			case <-r.Context().Done():
				active.Add(-1)
				close(p.cancelled)
			}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(remote.Close)
	cfg := &config.Config{}
	cfg.Telegram.BotToken, cfg.Telegram.UserID, cfg.Telegram.MaxConcurrentWorkers = "first", 7, 1
	offsets := &UpdateOffsets{}
	started, cancelled, release := make(chan int, 4), make(chan struct{}, 4), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	start := func(token string) *Bot {
		current := cfg.Clone()
		current.Telegram.BotToken = token
		b := startLongPolling(context.Background(), current, logger, nil, offsets, remote.URL+"/bot%s/%s", remote.Client(), func(ctx context.Context, _ *tgbotapi.BotAPI, u tgbotapi.Update) {
			started <- u.UpdateID
			<-ctx.Done()
			cancelled <- struct{}{}
			<-release
		})
		t.Cleanup(b.Stop)
		return b
	}
	update := func(id int) tgbotapi.Update {
		return tgbotapi.Update{UpdateID: id, Message: &tgbotapi.Message{From: &tgbotapi.User{ID: 7}}}
	}
	b := start("first")
	p := awaitTelegram(t, polls)
	if p.offset != 0 {
		t.Fatal(p.offset)
	}
	p.reply <- []tgbotapi.Update{update(40), update(41)}
	if id := awaitTelegram(t, started); id != 40 {
		t.Fatal(id)
	}
	changed := cfg.Clone()
	changed.Telegram.BotToken = "rotated"
	b.CancelIfConfigChanged(changed)
	awaitTelegram(t, cancelled)
	stopped := make(chan struct{})
	go func() { b.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned before its worker")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	awaitTelegram(t, stopped)
	b.Stop()
	if got := offsets.get(11); got != 41 {
		t.Fatalf("cursor = %d, want accepted update 40 only", got)
	}

	b = start("rotated")
	p = awaitTelegram(t, polls)
	if p.offset != 41 {
		t.Fatalf("rotated token cursor = %d", p.offset)
	}
	p.reply <- []tgbotapi.Update{update(40), update(41)}
	if id := awaitTelegram(t, started); id != 41 {
		t.Fatalf("replayed update %d", id)
	}
	pending := awaitTelegram(t, polls)
	b.Stop()
	awaitTelegram(t, cancelled)
	awaitTelegram(t, pending.cancelled)
	if got := offsets.get(11); got != 42 {
		t.Fatal(got)
	}

	b = start("other")
	p = awaitTelegram(t, polls)
	if p.offset != 0 {
		t.Fatalf("different bot inherited cursor %d", p.offset)
	}
	b.Stop()
	awaitTelegram(t, p.cancelled)
	if peak.Load() != 1 {
		t.Fatalf("overlapping pollers: %d", peak.Load())
	}
	if status := integrationstatus.TelegramStatus(); status.ConsecutivePollErrors != 0 {
		t.Fatalf("reload counted as failure: %+v", status)
	}
}

func TestTelegramConfigRevocationCancelsInitialization(t *testing.T) {
	for _, change := range []string{"token", "remove", "user", "workers", "egg"} {
		t.Run(change, func(t *testing.T) {
			started, cancelled := make(chan struct{}), make(chan struct{})
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
				close(cancelled)
			}))
			defer remote.Close()
			cfg := &config.Config{}
			cfg.Telegram.BotToken = "fixture"
			cfg.Telegram.UserID = 7
			b := startLongPolling(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil)
			defer b.Stop()
			awaitTelegram(t, started)
			b.CancelIfConfigChanged(cfg.Clone())
			if !b.MatchesConfig(cfg) {
				t.Fatal("unchanged settings revoked bot")
			}
			next := cfg.Clone()
			switch change {
			case "token":
				next.Telegram.BotToken = "rotated"
			case "remove":
				next.Telegram.BotToken = ""
			case "user":
				next.Telegram.UserID++
			case "workers":
				next.Telegram.MaxConcurrentWorkers++
			case "egg":
				next.EggMode.Enabled = true
			}
			b.CancelIfConfigChanged(next)
			awaitTelegram(t, cancelled)
			b.Stop()
			if status := integrationstatus.TelegramStatus(); status.LastErrorCode != "" {
				t.Fatalf("cancellation became a fault: %+v", status)
			}
		})
	}
}

func TestTelegramInvalidTokenIsUnavailableAndDisabledConfigStartsNothing(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
	}))
	defer remote.Close()
	cfg := &config.Config{}
	cfg.Telegram.BotToken = "invalid"
	cfg.Telegram.UserID = 7
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := startLongPolling(context.Background(), cfg, logger, nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil)
	awaitTelegram(t, b.done)
	b.Stop()
	if status := integrationstatus.TelegramStatus(); status.State != "unavailable" || status.LastErrorCode != "telegram_init_failed" || status.ConsecutivePollErrors != 0 {
		t.Fatalf("invalid token status: %+v", status)
	}
	for _, egg := range []bool{false, true} {
		cfg.EggMode.Enabled = egg
		if !egg {
			cfg.Telegram.BotToken = ""
		} else {
			cfg.Telegram.BotToken = "fixture"
		}
		if b := startLongPolling(context.Background(), cfg, logger, nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil); b != nil {
			b.Stop()
			t.Fatal("disabled config started bot")
		}
	}
}

func TestTelegramOffsetSurvivesRestartWithPersistentStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileOffsetStore(dir)
	if err != nil || store == nil {
		t.Fatalf("NewFileOffsetStore = %v, %v", store, err)
	}
	offsets := &UpdateOffsets{}
	offsets.SetStore(store)
	if err := offsets.Save(11, 42); err != nil {
		t.Fatalf("save cursor: %v", err)
	}

	// A restarted process reads the cursor back from the same data directory.
	restarted, err := NewFileOffsetStore(dir)
	if err != nil || restarted == nil {
		t.Fatalf("NewFileOffsetStore = %v, %v", restarted, err)
	}
	fresh := &UpdateOffsets{}
	fresh.SetStore(restarted)
	offset, ok := fresh.Load(11)
	if !ok || offset != 42 {
		t.Fatalf("restored cursor = %d (%t), want 42", offset, ok)
	}
	if got := fresh.get(11); got != 42 {
		t.Fatalf("cached cursor = %d, want 42", got)
	}

	data, err := os.ReadFile(filepath.Join(dir, telegramOffsetsFileName))
	if err != nil {
		t.Fatalf("read cursor file: %v", err)
	}
	var document struct {
		Offsets map[string]int `json:"offsets"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse cursor file: %v", err)
	}
	if document.Offsets["11"] != 42 {
		t.Fatalf("cursor document = %#v, want 42 for bot 11", document.Offsets)
	}
}

func TestTelegramCorruptedCursorFileStartsAtZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, telegramOffsetsFileName), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write cursor file: %v", err)
	}
	store, err := NewFileOffsetStore(dir)
	if err != nil || store == nil {
		t.Fatalf("NewFileOffsetStore = %v, %v", store, err)
	}
	offsets := &UpdateOffsets{}
	offsets.SetStore(store)
	offset, ok := offsets.Load(11)
	if ok || offset != 0 {
		t.Fatalf("cursor = %d (%t), want 0 for a corrupted cursor file", offset, ok)
	}
	// The store must recover on the next write.
	if err := offsets.Save(11, 5); err != nil {
		t.Fatalf("save cursor after corruption: %v", err)
	}
	if got := offsets.get(11); got != 5 {
		t.Fatalf("cursor = %d, want 5", got)
	}
}

func TestTelegramEmptyDataDirKeepsInMemoryCursor(t *testing.T) {
	store, err := NewFileOffsetStore("   ")
	if err != nil {
		t.Fatalf("NewFileOffsetStore: %v", err)
	}
	if store != nil {
		t.Fatal("an empty data directory must not create a cursor store")
	}
	offsets := &UpdateOffsets{}
	if err := offsets.Save(11, 5); err != nil {
		t.Fatalf("in-memory save: %v", err)
	}
	if offset, ok := offsets.Load(11); !ok || offset != 5 {
		t.Fatalf("cursor = %d (%t), want 5", offset, ok)
	}
}

func TestTelegramBotStartsWithZeroOffsetWhenCursorFileIsCorrupted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, telegramOffsetsFileName), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write cursor file: %v", err)
	}
	offsets := make(chan int, 4)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 11, "is_bot": true, "username": "fixture"}})
		case strings.HasSuffix(r.URL.Path, "/deleteWebhook"):
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			offset, _ := strconv.Atoi(r.Form.Get("offset"))
			offsets <- offset
			<-r.Context().Done()
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(remote.Close)
	cfg := &config.Config{}
	cfg.Telegram.BotToken, cfg.Telegram.UserID = "fixture", 7
	cfg.Directories.DataDir = dir
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := startLongPolling(context.Background(), cfg, logger, nil, nil, remote.URL+"/bot%s/%s", remote.Client(), nil)
	t.Cleanup(b.Stop)
	if got := awaitTelegram(t, offsets); got != 0 {
		t.Fatalf("polling offset = %d, want 0 after a corrupted cursor file", got)
	}
}

type telegramBatchPoller struct {
	batch   []tgbotapi.Update
	served  bool
	release chan struct{}
}

func (p *telegramBatchPoller) GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	if !p.served {
		p.served = true
		return p.batch, nil
	}
	<-p.release
	return nil, nil
}

func TestTelegramCursorIsPersistedWhilePolling(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseAll)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	batch := make([]tgbotapi.Update, 0, 12)
	for i := 1; i <= 12; i++ {
		batch = append(batch, tgbotapi.Update{UpdateID: i})
	}
	poller := &telegramBatchPoller{batch: batch, release: release}
	persisted := make(chan int, 16)
	done := make(chan int, 1)
	go func() {
		done <- runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, func(tgbotapi.Update) bool { return true }, func(offset int) {
			select {
			case persisted <- offset:
			default:
			}
		})
	}()
	// The cursor advances past the handed-out update and is written after
	// every one of them, so the first writes carry the next expected id.
	for want := 2; want <= 4; want++ {
		select {
		case offset := <-persisted:
			if offset != want {
				t.Fatalf("persisted cursor = %d, want %d", offset, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cursor was not persisted while polling")
		}
	}
	cancel()
	releaseAll()
	awaitTelegram(t, done)
}

// telegramRestartPoller serves one batch and then stops the poller, so a
// restart check does not block on the long poll.
type telegramRestartPoller struct {
	batch  []tgbotapi.Update
	served bool
	cancel context.CancelFunc
}

func (p *telegramRestartPoller) GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	if !p.served {
		p.served = true
		return p.batch, nil
	}
	p.cancel()
	return nil, nil
}

func TestTelegramDeliveryFailureCounterCountsEverySendPath(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 11, "is_bot": true, "username": "fixture"}})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"ok":false,"description":"boom"}`)
	}))
	t.Cleanup(remote.Close)
	bot, err := tgbotapi.NewBotAPIWithClient("token", remote.URL+"/bot%s/%s", remote.Client())
	if err != nil {
		t.Fatalf("construct Telegram bot API: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	before := integrationstatus.TelegramStatus().DeliveryFailures
	if err := deliverMessage(bot, 4711, "hello", "answer", logger); err == nil {
		t.Fatal("deliverMessage must report the failed delivery")
	}
	if got := integrationstatus.TelegramStatus().DeliveryFailures; got != before+1 {
		t.Fatalf("delivery failures = %d, want %d", got, before+1)
	}

	broker := &TelegramBroker{bot: bot, chatID: 4711, logger: logger}
	broker.deliver("status")
	if got := integrationstatus.TelegramStatus().DeliveryFailures; got != before+2 {
		t.Fatalf("delivery failures = %d, want %d", got, before+2)
	}
	if status := integrationstatus.TelegramStatus(); status.LastDeliveryErrorAt == "" {
		t.Fatal("the last delivery error must be timestamped")
	}
}

// telegramFuncHeader matches a top level function declaration and captures its
// name, with or without a receiver.
var telegramFuncHeader = regexp.MustCompile(`^func (?:\([^)]*\)\s*)?([A-Za-z0-9_]+)\(`)

// Every Telegram send has to run through deliverMessage or the broker, because
// only there a lost message is counted. A direct send would stay invisible in
// the delivery counter of the runtime status.
func TestTelegramSendsAllPassThroughTheDeliveryCounter(t *testing.T) {
	source, err := os.ReadFile("bot.go")
	if err != nil {
		t.Fatalf("read bot.go: %v", err)
	}
	counting := map[string]bool{"deliverMessage": true, "deliver": true}
	seen := map[string]bool{}
	current := ""
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if match := telegramFuncHeader.FindStringSubmatch(trimmed); match != nil {
			current = match[1]
		}
		if !strings.Contains(line, "sendTelegramMessage(") {
			continue
		}
		if telegramFuncHeader.MatchString(trimmed) {
			continue // the helper itself
		}
		seen[current] = true
	}
	for name := range seen {
		if !counting[name] {
			t.Fatalf("bot.go sends through %q, which does not count the delivery", name)
		}
	}
	if !seen["deliverMessage"] || !seen["deliver"] {
		t.Fatalf("expected the sends inside deliverMessage and the broker, saw %v", seen)
	}
}

func TestTelegramCursorSurvivesRestartWithoutReplay(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseAll)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	batch := []tgbotapi.Update{{UpdateID: 1}, {UpdateID: 2}, {UpdateID: 3}}
	poller := &telegramBatchPoller{batch: batch, release: release}
	persisted := make(chan int, 8)
	done := make(chan int, 1)
	go func() {
		done <- runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, func(tgbotapi.Update) bool { return true }, func(offset int) {
			select {
			case persisted <- offset:
			default:
			}
		})
	}()
	last := 0
	for want := 2; want <= 4; want++ {
		select {
		case offset := <-persisted:
			last = offset
			if offset != want {
				t.Fatalf("persisted cursor = %d, want %d", offset, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cursor was not persisted while polling")
		}
	}
	cancel()
	releaseAll()
	awaitTelegram(t, done)

	// A restart continues at the persisted cursor, so the updates of the first
	// batch must not reach the worker a second time.
	restartCtx, restartCancel := context.WithCancel(context.Background())
	t.Cleanup(restartCancel)
	handled := 0
	runPollingLoop(restartCtx, &telegramRestartPoller{batch: batch, cancel: restartCancel}, tgbotapi.NewUpdate(last), nil, nil, func(tgbotapi.Update) bool {
		handled++
		return true
	}, nil)
	if handled != 0 {
		t.Fatalf("restart replayed %d updates, want 0", handled)
	}
}
