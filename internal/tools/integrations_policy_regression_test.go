package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdGuardFilteringTogglePreservesInterval(t *testing.T) {
	for _, interval := range []int{0, 1, 12, 24, 48} {
		t.Run(fmt.Sprint(interval), func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/control/filtering/status" {
					fmt.Fprintf(w, `{"interval":%d}`, interval)
					return
				}
				posts++
				var body struct {
					Enabled  bool
					Interval int
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Enabled || body.Interval != interval {
					t.Errorf("unexpected update: %+v, %v", body, err)
				}
				fmt.Fprint(w, `{}`)
			}))
			defer srv.Close()
			if out := AdGuardFilteringToggle(AdGuardConfig{URL: srv.URL}, true); !strings.Contains(out, `"status":"ok"`) || posts != 1 {
				t.Fatalf("update failed: %s, posts=%d", out, posts)
			}
		})
	}
	for _, reply := range []string{`{}`, `{"interval":-1}`, `{"interval":"24"}`, `invalid`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Error("mutation after invalid status")
			}
			fmt.Fprint(w, reply)
		}))
		out := AdGuardFilteringToggle(AdGuardConfig{URL: srv.URL}, false)
		srv.Close()
		if !strings.Contains(out, `"status":"error"`) {
			t.Fatalf("invalid status accepted: %s", out)
		}
	}
}

func TestPythonIntegrationSecretNamesCannotBeExported(t *testing.T) {
	for _, key := range []string{"cloudflared_token", "cloudflared_credentials", "cloudflare_api_token", "three_d_printer_klipper_fixture", "sql_fixture_password", " SQL_FIXTURE_PASSWORD ", "proxy_basic_auth_user", "proxy_basic_auth_pass"} {
		if IsPythonAccessibleSecret(key) {
			t.Errorf("system secret allowed: %s", key)
		}
	}
	if !IsPythonAccessibleSecret("my_tool_fixture_key") {
		t.Fatal("agent secret incorrectly blocked")
	}
}

func TestUptimeKumaStopCancelsAndDrainsFetch(t *testing.T) {
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	p := NewUptimeKumaPoller(UptimeKumaPollerConfig{Fetch: func(ctx context.Context) (UptimeKumaSnapshot, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return UptimeKumaSnapshot{}, ctx.Err()
	}})
	p.StartContext(context.Background())
	<-started
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("fetch not cancelled")
	}
	select {
	case <-done:
		t.Fatal("Stop returned before fetch drained")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish")
	}
	p.Start()
	p.Stop()
}
