package chatrt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestServerArgsAndEnv(t *testing.T) {
	cpu := serverArgs(LaunchSpec{Backend: "cpu", ModelPath: "/m/a.gguf", ContextSize: 4096, Threads: 3}, 5555)
	wantCPU := []string{"--model", "/m/a.gguf", "--alias", "chibi", "--host", "127.0.0.1", "--port", "5555",
		"--ctx-size", "4096", "--parallel", "1", "--n-gpu-layers", "0", "--threads", "3", "--threads-batch", "3",
		"--jinja", "--reasoning", "off", "--no-webui", "--no-slots"}
	if !reflect.DeepEqual(cpu, wantCPU) {
		t.Fatalf("cpu args %v", cpu)
	}
	cuda := serverArgs(LaunchSpec{Backend: "cuda", ModelPath: "/m/a.gguf", ContextSize: 2048, Threads: 0}, 6000)
	if cuda[13] != "999" || cuda[15] != "1" || cuda[9] != "2048" {
		t.Fatalf("cuda args %v", cuda)
	}
	const key = "k3y-v4lue-not-in-argv"
	for _, a := range append(cpu, cuda...) {
		if strings.Contains(a, key) || a == "--api-key" {
			t.Fatalf("argv carries the key or --api-key: %v", a)
		}
	}

	env := serverEnv([]string{"PATH=/bin", "LLAMA_ARG_HOST=0.0.0.0", "llama_arg_port=1", "LLAMA_API_KEY=old", "HOME=/h"}, key)
	count := 0
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "LLAMA_") {
			if e != "LLAMA_API_KEY="+key {
				t.Fatalf("inherited LLAMA variable kept: %q", e)
			}
			count++
		}
	}
	if count != 1 || !reflect.DeepEqual(env[:2], []string{"PATH=/bin", "HOME=/h"}) {
		t.Fatalf("env %v", env)
	}
}

func TestNewServerCommandUsesFilteredEnvironment(t *testing.T) {
	t.Setenv("AURAGO_MASTER_KEY", "master-key-must-not-leak")
	t.Setenv("LLAMA_ARG_HOST", "0.0.0.0")
	cmd := newServerCommand("llama-server", nil, "the-key", []string{"--version"})
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "AURAGO_MASTER_KEY=") || strings.HasPrefix(e, "LLAMA_ARG_HOST=") {
			t.Fatalf("child environment carries %q", strings.SplitN(e, "=", 2)[0])
		}
	}
	if cmd.Env == nil {
		t.Fatal("nil environment would inherit everything")
	}
	for _, a := range cmd.Args {
		if strings.Contains(a, "the-key") {
			t.Fatal("key in argv")
		}
	}
}

func TestWaitHealthy(t *testing.T) {
	oldInterval, oldLimit := healthInterval, healthLimit
	healthInterval, healthLimit = 5*time.Millisecond, 5*time.Second
	defer func() { healthInterval, healthLimit = oldInterval, oldLimit }()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" || r.Header.Get("Authorization") != "" {
			t.Errorf("health request %s auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status": "ok" }`))
	}))
	defer srv.Close()
	if err := waitHealthy(context.Background(), make(chan struct{}), srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 3 {
		t.Fatalf("ready after %d calls", calls.Load())
	}

	const secret = "health-secret-value-0123456789"
	security.RegisterSensitive(secret)
	done := make(chan struct{})
	close(done)
	err := waitHealthy(context.Background(), done, "http://127.0.0.1:1", func() string { return "fatal: cannot load model " + secret })
	if err == nil || !errors.Is(err, errProcessExited) || !strings.Contains(err.Error(), "cannot load model") {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("the error carries a registered secret")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitHealthy(ctx, make(chan struct{}), "http://127.0.0.1:1", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait: %v", err)
	}
}

func TestLogBufferBounded(t *testing.T) {
	b := newLogBuffer(10)
	for i := 0; i < 5; i++ {
		if n, err := b.Write([]byte("abcdef")); n != 6 || err != nil {
			t.Fatal(n, err)
		}
	}
	got := b.String()
	if len(got) > 10 || !strings.HasSuffix(got, "abcdef") {
		t.Fatalf("buffer %q", got)
	}
	if newLogBuffer(5).String() != "" {
		t.Fatal("empty buffer not empty")
	}
}
