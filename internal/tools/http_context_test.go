package tools

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestNativeHTTPStopCancelsRequestAndResponseBody(t *testing.T) {
	calls := map[string]func(context.Context, string) error{
		"paperless": func(ctx context.Context, url string) error {
			resp, err := paperlessRequest(PaperlessConfig{Context: ctx, URL: url}, http.MethodGet, "blocked", nil, "")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			_, err = io.ReadAll(resp.Body)
			return err
		},
		"adguard": func(ctx context.Context, url string) error {
			_, _, err := adguardRequest(AdGuardConfig{Context: ctx, URL: url}, http.MethodGet, "/blocked", "")
			return err
		},
		"ansible": func(ctx context.Context, url string) error {
			_, _, err := ansibleRequest(AnsibleConfig{Context: ctx, URL: url, Timeout: 240}, http.MethodGet, "/blocked", nil)
			return err
		},
		"ollama": func(ctx context.Context, url string) error {
			_, _, err := ollamaRequest(OllamaConfig{Context: ctx, URL: url}, http.MethodGet, "/blocked", "")
			return err
		},
	}
	for name, call := range calls {
		for _, streaming := range []bool{false, true} {
			mode := "headers"
			if streaming {
				mode = "body"
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				entered, stopped := make(chan struct{}), make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if streaming {
						w.WriteHeader(http.StatusOK)
						w.(http.Flusher).Flush()
					}
					close(entered)
					<-r.Context().Done()
					close(stopped)
				}))
				defer server.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- call(ctx, server.URL) }()
				waitNativeHTTPTest(t, entered)
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("cancellation = %v", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("Stop did not cancel native HTTP")
				}
				waitNativeHTTPTest(t, stopped)
			})
		}
	}
}

func TestNativeHTTPSharedClientKeepsSessionsIndependent(t *testing.T) {
	entered := make(chan string, 2)
	releaseB := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- r.URL.Path
		if r.URL.Path == "/api/b" {
			select {
			case <-releaseB:
				_, _ = io.WriteString(w, "session-b-success")
			case <-r.Context().Done():
			}
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	type response struct {
		body string
		err  error
	}
	run := func(ctx context.Context, endpoint string, done chan<- response) {
		resp, err := paperlessRequest(PaperlessConfig{Context: ctx, URL: server.URL}, http.MethodGet, endpoint, nil, "")
		if err != nil {
			done <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		done <- response{body: string(body), err: err}
	}
	a, b := make(chan response, 1), make(chan response, 1)
	go run(ctxA, "a", a)
	go run(ctxB, "b", b)
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("requests did not start")
		}
	}
	cancelA()
	select {
	case got := <-a:
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("session A = %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session A did not stop")
	}
	select {
	case got := <-b:
		t.Fatalf("Stop A interrupted B: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseB)
	select {
	case got := <-b:
		if got.err != nil || got.body != "session-b-success" {
			t.Fatalf("session B = %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session B did not complete")
	}
}

func TestNativeHTTPOAuthRefreshLockHonorsStop(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mu      *sync.Mutex
		refresh func(context.Context) error
	}{
		{"google", &googleWorkspaceTokenRefreshMu, func(ctx context.Context) error { return (&GWorkspaceClient{Context: ctx}).refreshIfNeeded() }},
		{"onedrive", &oneDriveTokenRefreshMu, func(ctx context.Context) error { return (&OneDriveClient{Context: ctx}).refreshIfNeeded() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.mu.Lock()
			defer tc.mu.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- tc.refresh(ctx) }()
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("refresh = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Stop waits for another session's OAuth refresh")
			}
		})
	}
}

func TestNativeHTTPProviderHelpersRejectCanceledOwner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, call := range map[string]func() error{
		"tailscale": func() error {
			_, _, err := tailscaleRequest(TailscaleConfig{Context: ctx}, "GET", "/api/v2/tailnet/-/devices", "")
			return err
		},
		"netlify": func() error {
			_, _, err := netlifyRequest(NetlifyConfig{Context: ctx}, "GET", "/sites", nil)
			return err
		},
		"netlify-upload": func() error {
			_, _, err := netlifyRequestRaw(NetlifyConfig{Context: ctx}, "POST", "/sites/fixture/deploys", "application/zip", nil)
			return err
		},
		"vercel": func() error {
			_, _, err := vercelRequest(VercelConfig{Context: ctx}, "GET", "/v9/projects", nil)
			return err
		},
		"virustotal": func() error {
			_, err := virustotalDoRequest("fixture", "GET", "https://www.virustotal.com/api/v3/files/fixture", nil, "", ctx)
			return err
		},
		"koofr": func() error {
			_, err := doKoofrRequest("GET", "https://app.koofr.net/api/v2/mounts", "", "", "", nil, ctx)
			return err
		},
		"ntfy": func() error {
			cfg := &config.Config{}
			cfg.Notifications.Ntfy.Enabled = true
			cfg.Notifications.Ntfy.URL, cfg.Notifications.Ntfy.Topic = "https://ntfy.sh", "fixture"
			return sendNtfy(cfg, "fixture", "fixture", "normal", ctx)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled owner = %v", err)
			}
		})
	}
}

func TestNativeHTTPNilContextRetainsStandaloneAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "{}") }))
	defer server.Close()
	resp, err := paperlessRequest(PaperlessConfig{URL: server.URL}, http.MethodGet, "health", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func waitNativeHTTPTest(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("native HTTP fixture timed out")
	}
}
