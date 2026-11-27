package chatrttest_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/chibisensei/chatrt/chatrttest"
)

func newManager(t *testing.T, l *chatrttest.Launcher) *chatrt.Manager {
	t.Helper()
	cfg := chatrt.Config{Enabled: true, Backend: "auto", IdleMinutes: 10, ContextSize: 4096, CacheDir: filepath.Join(t.TempDir(), "runtime")}
	m := chatrt.NewManager(func() chatrt.Config { return cfg }, l, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	return m
}

func model(t *testing.T, key string) chatrt.ModelRef {
	t.Helper()
	path := filepath.Join(t.TempDir(), "m.gguf")
	if err := os.WriteFile(path, []byte("GGUFdata"), 0o600); err != nil {
		t.Fatal(err)
	}
	return chatrt.ModelRef{Key: key, Path: path, Bytes: 8}
}

func TestFakeLauncherThroughManager(t *testing.T) {
	l := chatrttest.New(false)
	l.Replies["base:"] = "Base says hi."
	m := newManager(t, l)
	ctx := context.Background()

	if _, err := m.Acquire(ctx, model(t, "tuned:r1")); !errors.Is(err, chatrt.ErrNotInstalled) {
		t.Fatalf("before install: %v", err)
	}
	if err := m.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(ctx); st.State != "ready" || st.Download.Bytes != 15_857_307 || st.Download.Host != "github.com" {
		t.Fatalf("status %+v", st)
	}

	chat := func(key string) (string, chatrt.ChatStats) {
		lease, err := m.Acquire(ctx, model(t, key))
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Release()
		var deltas []string
		stats, err := lease.Chat(ctx, chatrt.ChatRequest{Messages: []chatrt.Message{{Role: "user", Content: "hi"}}}, func(s string) { deltas = append(deltas, s) })
		if err != nil {
			t.Fatal(err)
		}
		if len(deltas) < 2 {
			t.Fatalf("reply was not streamed word by word: %q", deltas)
		}
		return strings.Join(deltas, ""), stats
	}
	text, stats := chat("tuned:r1")
	if text != "Fake answer." || stats.FinishReason != "stop" || stats.PromptTokens != 10 || stats.CompletionTokens != 2 || stats.Backend != "cpu" {
		t.Fatalf("tuned: %q %+v", text, stats)
	}
	if text, _ := chat("base:x"); text != "Base says hi." {
		t.Fatalf("base: %q", text)
	}
	if n := len(l.Requests("tuned:")); n != 1 {
		t.Fatalf("tuned requests %d", n)
	}
	if n := len(l.Launches()); n != 2 {
		t.Fatalf("launches %d", n)
	}
	m.StopAll(ctx)
	if l.Stopped() != 2 {
		t.Fatalf("stopped %d", l.Stopped())
	}
}

func TestFakeLauncherFailuresAndHold(t *testing.T) {
	l := chatrttest.New(true)
	l.FailLaunch["base:"] = errors.New("cannot start")
	l.ChatErr["tuned:"] = chatrt.ErrContext
	m := newManager(t, l)
	ctx := context.Background()

	if _, err := m.Acquire(ctx, model(t, "base:x")); err == nil {
		t.Fatal("the scripted launch failure did not show")
	}
	lease, err := m.Acquire(ctx, model(t, "tuned:r1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lease.Chat(ctx, chatrt.ChatRequest{}, nil); !errors.Is(err, chatrt.ErrContext) {
		t.Fatalf("scripted chat error: %v", err)
	}
	lease.Release()

	l.ChatErr = map[string]error{}
	l.Hold = make(chan struct{})
	lease, err = m.Acquire(ctx, model(t, "tuned:r1"))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	cctx, cancel := context.WithCancel(ctx)
	errc := make(chan error, 1)
	go func() { _, err := lease.Chat(cctx, chatrt.ChatRequest{}, nil); errc <- err }()
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("held chat: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("held chat did not end on cancel")
	}
	select {
	case key := <-l.Cancelled():
		if key != "tuned:r1" {
			t.Fatalf("cancelled key %q", key)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no cancel signal")
	}
}
