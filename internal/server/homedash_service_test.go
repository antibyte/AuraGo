package server

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestHomeDashRuntimeConfigFromConfig(t *testing.T) {
	base := func() *config.Config {
		c := &config.Config{}
		c.HomeDash.Enabled = true
		c.HomeDash.Sources.System.Enabled = true
		if err := config.NormalizeHomeDashConfig(&c.HomeDash); err != nil {
			t.Fatal(err)
		}
		return c
	}
	specs := []homeDashSourceSpec{{ID: "system", ConfigKey: "system", Settings: homeDashSystemSettings}}
	tests := []struct {
		name   string
		mutate func(*config.Config)
		ok     func(homedash.RuntimeConfig) bool
	}{
		{"defaults", nil, func(rc homedash.RuntimeConfig) bool {
			s := rc.Sources["system"]
			return rc.Enabled && !rc.EggMode && !rc.ReadOnly && !rc.DesktopReadOnly && rc.Limits.MaxStreams == 32 &&
				rc.Limits.MaxSubscriptions == 300 && rc.Limits.MaxDashboards == 50 && rc.History.SampleSeconds == 60 &&
				s.Enabled && !s.Control && s.PollSeconds == 5 && s.ConfigKey == "system"
		}},
		{"disabled", func(c *config.Config) { c.HomeDash.Enabled = false }, func(rc homedash.RuntimeConfig) bool { return !rc.Enabled }},
		{"egg mode", func(c *config.Config) { c.EggMode.Enabled = true }, func(rc homedash.RuntimeConfig) bool { return rc.EggMode }},
		{"desktop read-only", func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, func(rc homedash.RuntimeConfig) bool { return rc.DesktopReadOnly }},
		{"homedash read-only", func(c *config.Config) { c.HomeDash.ReadOnly = true }, func(rc homedash.RuntimeConfig) bool { return rc.ReadOnly }},
		{"kiosk", func(c *config.Config) { c.HomeDash.Kiosk.Enabled = true; c.HomeDash.Kiosk.AllowControl = false },
			func(rc homedash.RuntimeConfig) bool { return rc.KioskEnabled && !rc.KioskAllowControl }},
		{"system off", func(c *config.Config) { c.HomeDash.Sources.System.Enabled = false },
			func(rc homedash.RuntimeConfig) bool { return !rc.Sources["system"].Enabled }},
	}
	for _, tc := range tests {
		c := base()
		if tc.mutate != nil {
			tc.mutate(c)
		}
		if rc := homeDashRuntimeConfig(c, specs); !tc.ok(rc) {
			t.Errorf("%s: runtime config = %+v", tc.name, rc)
		}
	}
}

func TestHomeDashInitAndCloseOpenTheStore(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.HomeDash.Enabled = true
	if err := config.NormalizeHomeDashConfig(&cfg.HomeDash); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.initHomeDash(context.Background())
	svc := s.homeDash.Load()
	if svc == nil {
		t.Fatal("initHomeDash published no service")
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, config.HomeDashDBFilename)); err != nil {
		t.Fatalf("homedash.db: %v", err)
	}
	if got := svc.hub.Sources(); len(got) != 2 || got[0].Source != "system" || got[1].Source != "ha" || got[1].State != homedash.StatusDisabled {
		t.Fatalf("sources = %+v", got)
	}
	s.closeHomeDash()
	s.closeHomeDash()
	if s.homeDash.Load() != nil {
		t.Fatal("service still published after closeHomeDash")
	}
	none := &Server{Cfg: &config.Config{}}
	none.initHomeDash(context.Background())
	if none.homeDash.Load() != nil {
		t.Fatal("without a data directory HomeDash must stay unavailable")
	}
}

func TestHomeDashReconfiguresOnConfigSnapshotAndGatesCapability(t *testing.T) {
	fake := sourcetest.New("fake", sourcetest.Entity("fake:a", homedash.KindSensor))
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	caps := desktopCapabilities{env.s}
	status := func() string { return env.svc.hub.Sources()[0].State }
	if status() != homedash.StatusIdle || !caps.HasCapability("homedash") {
		t.Fatalf("start: status %q, capability %v", status(), caps.HasCapability("homedash"))
	}
	cfg := *env.s.ConfigSnapshot()
	cfg.HomeDash.Enabled = false
	env.s.replaceConfigSnapshot(&cfg)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return status() == homedash.StatusDisabled }, "source disabled by hot reload")
	if caps.HasCapability("homedash") {
		t.Fatal("capability on although homedash.enabled is false")
	}
	cfg.HomeDash.Enabled = true
	env.s.replaceConfigSnapshot(&cfg)
	sourcetest.WaitFor(t, 2*time.Second, func() bool { return status() == homedash.StatusIdle }, "source enabled again")
	cfg.VirtualDesktop.Enabled = false
	env.s.replaceConfigSnapshot(&cfg)
	if caps.HasCapability("homedash") {
		t.Fatal("capability on without the virtual desktop")
	}
}
