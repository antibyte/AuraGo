package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

func TestRetroGamesSettingsFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join("data", "dir")
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = false
	got := retroGamesSettingsFromConfig(cfg)
	want := retrogames.Settings{Enabled: true, AgentAccess: false, DataDir: filepath.Join("data", "dir", "retrogames")}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	if got := retroGamesSettingsFromConfig(nil); got.Enabled {
		t.Fatalf("nil config must disable: %+v", got)
	}
}

// The favorites, plays and broken markers live in the index database, so the
// backup list (and the agent's protected files) must name the file the store
// really opens.
func TestRetroGamesIndexIsInTheSQLiteBackupList(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.RetroGames.Enabled = true
	svc := retrogames.NewService(retrogames.Deps{Logger: slog.Default(), Fetcher: &retrogamestest.Fetcher{}})
	svc.Configure(retroGamesSettingsFromConfig(cfg))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})
	// Reading the library opens (creates) the index without syncing.
	if _, err := svc.Library(context.Background()); err != nil {
		t.Fatal(err)
	}
	var listed string
	for _, path := range config.SQLiteDatabasePaths(cfg) {
		if filepath.Base(path) == "retrogames.db" {
			listed = path
		}
	}
	if listed == "" {
		t.Fatalf("backups miss retrogames.db: %v", config.SQLiteDatabasePaths(cfg))
	}
	if info, err := os.Stat(listed); err != nil || info.IsDir() {
		t.Fatalf("the listed %s is not the store's database: info=%v err=%v", listed, info, err)
	}
	if protected := config.SQLiteProtectedPaths(cfg); !containsString(protected, listed) || !containsString(protected, listed+"-wal") {
		t.Fatalf("protected paths miss the index: %v", protected)
	}
}

// Without a data directory the service gets none, so its own "no data
// directory" guard applies instead of an index created in ./retrogames.
func TestRetroGamesSettingsFromConfigWithoutDataDir(t *testing.T) {
	for _, dataDir := range []string{"", "   ", "\t\n"} {
		cfg := &config.Config{}
		cfg.Directories.DataDir = dataDir
		cfg.RetroGames.Enabled = true
		cfg.RetroGames.AgentAccess = true
		got := retroGamesSettingsFromConfig(cfg)
		want := retrogames.Settings{Enabled: true, AgentAccess: true, DataDir: ""}
		if got != want {
			t.Fatalf("data_dir %q: settings = %+v, want %+v", dataDir, got, want)
		}
	}
}

func TestSyncRetroGamesSettingsFollowsConfig(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.RetroGames.Enabled = true
	s.initConfigSnapshot()
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.syncRetroGamesSettings()
	if !s.RetroGames.Enabled() {
		t.Fatal("service should be enabled")
	}
	off := s.Cfg.Clone()
	off.RetroGames.Enabled = false
	s.cfgSnapshot.Store(off)
	s.syncRetroGamesSettings()
	if s.RetroGames.Enabled() {
		t.Fatal("service should follow the switch off")
	}
}

// Overlapping syncs apply their snapshots in order. The hook holds the first
// sync after it read the older snapshot and before it configures the service;
// a second sync, started after a newer snapshot was published, must wait for
// it and then apply the newer one. Without retroGamesSyncMu the second sync
// would configure the newer settings first and the first would revert them.
func TestSyncRetroGamesSettingsAppliesSnapshotsInOrder(t *testing.T) {
	base := &config.Config{}
	base.Directories.DataDir = t.TempDir()
	base.RetroGames.Enabled = true
	base.RetroGames.AgentAccess = false
	s := &Server{Cfg: base}
	s.initConfigSnapshot()
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.syncRetroGamesSettings()
	if !s.RetroGames.Enabled() || s.RetroGames.AgentAccess() {
		t.Fatalf("startup state: enabled=%v agentAccess=%v, want enabled without agent access", s.RetroGames.Enabled(), s.RetroGames.AgentAccess())
	}
	older, newer := base.Clone(), base.Clone()
	older.RetroGames.Enabled = false
	newer.RetroGames.AgentAccess = true

	reached := make(chan struct{})
	proceed := make(chan struct{})
	release := sync.OnceFunc(func() { close(proceed) })
	defer release()
	var calls atomic.Int32
	s.retroGamesBeforeConfigure = func() {
		if calls.Add(1) == 1 {
			close(reached)
			<-proceed
		}
	}
	s.cfgSnapshot.Store(older)
	first := make(chan struct{})
	go func() {
		s.syncRetroGamesSettings()
		close(first)
	}()
	<-reached
	s.cfgSnapshot.Store(newer)
	second := make(chan struct{})
	go func() {
		s.syncRetroGamesSettings()
		close(second)
	}()
	select {
	case <-second:
		t.Fatal("a second sync configured the service while the first one was applying its snapshot")
	case <-time.After(100 * time.Millisecond):
	}
	if !s.RetroGames.Enabled() || s.RetroGames.AgentAccess() {
		t.Fatalf("service while the first sync is held: enabled=%v agentAccess=%v, want the startup state", s.RetroGames.Enabled(), s.RetroGames.AgentAccess())
	}
	release()
	<-first
	<-second
	if !s.RetroGames.Enabled() || !s.RetroGames.AgentAccess() {
		t.Fatalf("service: enabled=%v agentAccess=%v, want the newer snapshot's enabled with agent access", s.RetroGames.Enabled(), s.RetroGames.AgentAccess())
	}
}

// Every config publisher reaches the service, not only the config save, and
// the Desktop app is announced exactly when the switch flips.
func TestReplaceConfigSnapshotConfiguresRetroGames(t *testing.T) {
	hub := desktop.NewHub(8)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	base := &config.Config{}
	base.Directories.DataDir = t.TempDir()
	base.RetroGames.Enabled = true
	base.RetroGames.AgentAccess = true
	s := &Server{Cfg: base, Logger: slog.Default(), DesktopHub: hub}
	s.initConfigSnapshot()
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.syncRetroGamesSettings()

	off := base.Clone()
	off.RetroGames.Enabled = false
	s.replaceConfigSnapshot(off)
	if s.RetroGames.Enabled() {
		t.Fatal("service should follow the switch off")
	}
	expectRetroGamesAvailability(t, events, false)

	// Same switch, another setting: the service follows, nobody is told.
	same := off.Clone()
	same.RetroGames.AgentAccess = false
	s.replaceConfigSnapshot(same)
	expectNoRetroGamesAvailability(t, events)

	on := same.Clone()
	on.RetroGames.Enabled = true
	on.RetroGames.AgentAccess = true
	s.replaceConfigSnapshot(on)
	if !s.RetroGames.Enabled() || !s.RetroGames.AgentAccess() {
		t.Fatalf("service after the switch on: enabled=%v agentAccess=%v", s.RetroGames.Enabled(), s.RetroGames.AgentAccess())
	}
	expectRetroGamesAvailability(t, events, true)
}

func TestPublishRetroGamesAvailabilityBroadcastsAppAvailability(t *testing.T) {
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	server := &Server{DesktopHub: hub, Logger: slog.Default()}
	server.publishRetroGamesAvailability(true)
	expectRetroGamesAvailability(t, events, true)
	server.publishRetroGamesAvailability(false)
	expectRetroGamesAvailability(t, events, false)
	(*Server)(nil).publishRetroGamesAvailability(true)
}

// expectRetroGamesAvailability waits for the app_availability event of the
// Retro Games app and checks what it announces.
func expectRetroGamesAvailability(t *testing.T, events <-chan desktop.Event, available bool) {
	t.Helper()
	select {
	case event := <-events:
		payload, _ := event.Payload.(map[string]interface{})
		if event.Type != "desktop_changed" || payload["operation"] != "app_availability" || payload["app_id"] != "retro-games" || payload["available"] != available {
			t.Fatalf("event = %+v, want retro-games available=%v", event, available)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing desktop_changed")
	}
}

func expectNoRetroGamesAvailability(t *testing.T, events <-chan desktop.Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected desktop event %+v", event)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRetroGamesCapability(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	caps := desktopCapabilities{s: s}
	if caps.HasCapability("retro_games") {
		t.Fatal("no service: capability must be false")
	}
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.Cfg.RetroGames.Enabled = true
	if !caps.HasCapability("retro_games") {
		t.Fatal("enabled: capability must be true")
	}
	s.Cfg.RetroGames.Enabled = false
	if caps.HasCapability("retro_games") {
		t.Fatal("disabled: capability must be false")
	}
}

func TestShutdownRetroGamesIsNilSafe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	(&Server{}).shutdownRetroGames(ctx)
}
