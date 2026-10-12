package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"

	"github.com/gorilla/websocket"
)

func TestDesktopRetroNetPolicyProjectsGrantByScope(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.RetroNetEnabled = false
	if s.desktopSerialPolicy(nil).RetroNetEnabled {
		t.Fatal("an explicit virtual_desktop.retronet_enabled: false must keep Retro-Net off")
	}
	s.Cfg.VirtualDesktop.RetroNetEnabled = true
	bearer := func(token string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		return r
	}
	if !s.desktopSerialPolicy(nil).RetroNetEnabled {
		t.Fatal("administrator policy lacks Retro-Net")
	}
	if !s.desktopSerialPolicy(bearer(writeToken)).RetroNetEnabled {
		t.Fatal("a desktop:write token may dial and must see Retro-Net")
	}
	if s.desktopSerialPolicy(bearer(readToken)).RetroNetEnabled {
		t.Fatal("a token without desktop:write sees Retro-Net")
	}
	encoded, err := json.Marshal(s.desktopSerialPolicy(nil))
	if err != nil || !strings.Contains(string(encoded), `"retronet_enabled":true`) {
		t.Fatalf("policy JSON = %s (%v)", encoded, err)
	}

	var payload desktop.BootstrapPayload
	s.enrichDesktopBootstrap(&payload)
	if !payload.RetroNetEnabled {
		t.Fatal("bootstrap was not enriched with Retro-Net")
	}
	encoded, err = json.Marshal(payload)
	if err != nil || !strings.Contains(string(encoded), `"retronet_enabled":true`) {
		t.Fatalf("bootstrap JSON lacks retronet_enabled: %v", err)
	}
	if !filterDesktopBootstrap(s, bearer(writeToken), payload).RetroNetEnabled {
		t.Fatal("write-token bootstrap lost Retro-Net")
	}
	if filterDesktopBootstrap(s, bearer(readToken), payload).RetroNetEnabled {
		t.Fatal("read-token bootstrap leaked Retro-Net")
	}

	s.Cfg.VirtualDesktop.ReadOnly = true
	payload = desktop.BootstrapPayload{}
	s.enrichDesktopBootstrap(&payload)
	if s.desktopSerialPolicy(nil).RetroNetEnabled || payload.RetroNetEnabled {
		t.Fatal("readonly must disable Retro-Net")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.VirtualDesktop.Enabled = false
	if s.desktopSerialPolicy(nil).RetroNetEnabled {
		t.Fatal("a disabled desktop must disable Retro-Net")
	}
}

// Retro-Net is on by default, but the effective grant still needs an enabled,
// writable Desktop and desktop:write: a loaded config without the key lets a
// write token dial, while a read token, readonly, a disabled Desktop or an
// explicit false keep it off.
func TestDesktopRetroNetPolicyFollowsLoadedConfigDefault(t *testing.T) {
	for _, tc := range []struct {
		name, desktop string
		want          bool
	}{
		{"key missing", "  enabled: true\n", true},
		{"readonly", "  enabled: true\n  readonly: true\n", false},
		{"desktop disabled", "  enabled: false\n", false},
		{"explicit false", "  enabled: true\n  retronet_enabled: false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("server:\n  ui_language: en\nvirtual_desktop:\n"+tc.desktop), 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := config.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			s, readToken, writeToken := testDesktopPermissionServer(t)
			s.Cfg.VirtualDesktop = loaded.VirtualDesktop
			bearer := func(token string) *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
				r.Header.Set("Authorization", "Bearer "+token)
				return r
			}
			if got := s.desktopSerialPolicy(nil).RetroNetEnabled; got != tc.want {
				t.Fatalf("administrator RetroNetEnabled = %v, want %v", got, tc.want)
			}
			if got := s.desktopSerialPolicy(bearer(writeToken)).RetroNetEnabled; got != tc.want {
				t.Fatalf("write token RetroNetEnabled = %v, want %v", got, tc.want)
			}
			if s.desktopSerialPolicy(bearer(readToken)).RetroNetEnabled {
				t.Fatal("a token without desktop:write sees Retro-Net")
			}
		})
	}
}

func TestDesktopRetroNetPolicyPushedWithoutRecreatingService(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.RetroNetEnabled = true
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	host := httptest.NewServer(handleDesktopWS(s))
	defer host.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/api/desktop/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	var event struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	var boot desktop.BootstrapPayload
	if err := json.Unmarshal(event.Payload, &boot); err != nil || event.Type != "welcome" || !boot.RetroNetEnabled {
		t.Fatalf("welcome lacks Retro-Net: %s (%v)", event.Payload, err)
	}
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.RetroNetEnabled = false
	s.CfgMu.Unlock()
	for {
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "desktop_policy" {
			break
		}
	}
	var policy desktopLivePolicy
	if err := json.Unmarshal(event.Payload, &policy); err != nil || policy.RetroNetEnabled || !policy.Enabled {
		t.Fatalf("Retro-Net revocation not projected: %s (%v)", event.Payload, err)
	}
	after, _, err := s.getDesktopService(context.Background())
	if err != nil || svc != after {
		t.Fatalf("the Retro-Net toggle recreated the desktop service: %v", err)
	}
}
