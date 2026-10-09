package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/localwiki"
	"aurago/internal/security"
)

func TestDesktopCapabilityLocalWikipediaFollowsConfig(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	capabilities := desktopCapabilities{s: s}
	s.Cfg.LocalWikipedia.Enabled = true
	if capabilities.HasCapability("local_wikipedia") {
		t.Fatal("without a manager the app must stay hidden")
	}
	s.LocalWiki = localwiki.NewManager(localwiki.Deps{Logger: slog.Default()})
	if !capabilities.HasCapability("local_wikipedia") {
		t.Fatal("enabled integration with a manager must grant the capability")
	}
	s.Cfg.LocalWikipedia.Enabled = false
	if capabilities.HasCapability("local_wikipedia") {
		t.Fatal("a disabled integration must hide the app")
	}
}

func TestPublishLocalWikipediaAvailabilityBroadcastsAppAvailability(t *testing.T) {
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	server := &Server{DesktopHub: hub, Logger: slog.Default()}
	server.publishLocalWikipediaAvailability(true)
	expectLocalWikipediaAvailability(t, events, true)
}

// expectLocalWikipediaAvailability waits for the app_availability event of the
// Wikipedia app and checks what it announces.
func expectLocalWikipediaAvailability(t *testing.T, events <-chan desktop.Event, available bool) {
	t.Helper()
	select {
	case event := <-events:
		payload, _ := event.Payload.(map[string]interface{})
		if event.Type != "desktop_changed" || payload["operation"] != "app_availability" || payload["app_id"] != "local-wikipedia" || payload["available"] != available {
			t.Fatalf("event = %+v, want local-wikipedia available=%v", event, available)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing desktop_changed")
	}
}

func expectNoDesktopEvent(t *testing.T, events <-chan desktop.Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected event %+v", event)
	case <-time.After(150 * time.Millisecond):
	}
}

// A config publication that flips local_wikipedia.enabled announces the app's
// availability; other publications stay quiet. The broadcast never runs on
// the publication path: replaceConfigSnapshot usually runs under CfgMu, and it
// must not wait for DesktopMu (held here) or the event fan-out.
func TestReplaceConfigSnapshotAnnouncesLocalWikipediaAvailability(t *testing.T) {
	base := &config.Config{}
	base.Directories.DataDir = t.TempDir()
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	manager := localwiki.NewManager(localwiki.Deps{Logger: slog.Default()})
	manager.Configure(localwiki.SettingsFromConfig(base))
	s := &Server{Cfg: base, Logger: slog.Default(), LocalWiki: manager, DesktopHub: hub}
	s.initConfigSnapshot()

	enabled := base.Clone()
	enabled.LocalWikipedia.Enabled = true
	s.DesktopMu.Lock()
	published := make(chan struct{})
	go func() {
		s.CfgMu.Lock()
		s.replaceConfigSnapshot(enabled)
		s.CfgMu.Unlock()
		close(published)
	}()
	select {
	case <-published:
	case <-time.After(2 * time.Second):
		s.DesktopMu.Unlock()
		t.Fatal("the config publication waited for the Desktop broadcast")
	}
	s.DesktopMu.Unlock()
	expectLocalWikipediaAvailability(t, events, true)

	unchanged := enabled.Clone()
	unchanged.LocalWikipedia.Language = "fr"
	s.replaceConfigSnapshot(unchanged)
	expectNoDesktopEvent(t, events)

	disabled := unchanged.Clone()
	disabled.LocalWikipedia.Enabled = false
	s.replaceConfigSnapshot(disabled)
	expectLocalWikipediaAvailability(t, events, false)

	// Without a manager the app never shows, so there is nothing to announce.
	s.LocalWiki = nil
	again := disabled.Clone()
	again.LocalWikipedia.Enabled = true
	s.replaceConfigSnapshot(again)
	expectNoDesktopEvent(t, events)
}

// The config UI save reaches the announcement through replaceConfigSnapshot.
func TestConfigSaveAnnouncesLocalWikipediaAvailability(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  system_language: Deutsch\nlocal_wikipedia:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("57", 32), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	manager := localwiki.NewManager(localwiki.Deps{})
	manager.Configure(localwiki.SettingsFromConfig(loaded))
	s := &Server{Cfg: loaded, Logger: slog.Default(), Vault: vault, LocalWiki: manager, DesktopHub: hub}
	save := func(payload string) {
		t.Helper()
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(payload)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("save %s = %d %s", payload, recorder.Code, recorder.Body.String())
		}
	}

	save(`{"local_wikipedia":{"enabled":true}}`)
	expectLocalWikipediaAvailability(t, events, true)
	if !(desktopCapabilities{s: s}).HasCapability("local_wikipedia") {
		t.Fatal("the saved switch must grant the capability")
	}
	save(`{"local_wikipedia":{"language":"fr"}}`)
	expectNoDesktopEvent(t, events)
	save(`{"local_wikipedia":{"enabled":false}}`)
	expectLocalWikipediaAvailability(t, events, false)
}
