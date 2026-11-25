package server

import (
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sources/system"
	"aurago/internal/tools"
)

func TestHomeDashSystemSnapshotFromServerMetrics(t *testing.T) {
	s := &Server{}
	if _, ok := s.homeDashSystemSnapshot(); ok {
		t.Fatal("snapshot before the first collector sample")
	}
	at := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"cpu": true, "memory": true, "disk": true, "network": true},
		CPU:       tools.CPUMetrics{UsagePercent: 12.5},
		Memory:    tools.MemoryMetrics{UsedPercent: 40},
		Disk:      tools.DiskMetrics{UsedPercent: 91},
		Network:   tools.NetworkMetrics{BytesSent: 10, BytesRecv: 20},
	}, at)
	got, ok := s.homeDashSystemSnapshot()
	if !ok || !got.SampledAt.Equal(at) || got.CPUPercent != 12.5 || got.MemoryPercent != 40 || got.DiskPercent != 91 ||
		!got.CPUOK || !got.MemoryOK || !got.DiskOK || !got.NetworkOK || got.NetBytesRecv != 20 || got.NetBytesSent != 10 ||
		!got.BootTime.IsZero() {
		t.Fatalf("snapshot = %+v %v", got, ok)
	}
	s.storeHostBootTime(at.Add(-time.Hour))
	if got, _ := s.homeDashSystemSnapshot(); !got.BootTime.Equal(at.Add(-time.Hour)) {
		t.Fatalf("boot time = %v", got.BootTime)
	}
}

func TestHomeDashSourceSpecsListSystemAndHomeAssistant(t *testing.T) {
	specs := (&Server{}).homeDashSourceSpecs()
	if len(specs) != 2 || specs[0].ID != system.SourceID || specs[0].ConfigKey != "system" ||
		specs[1].ID != hass.SourceID || specs[1].ConfigKey != "home_assistant" || specs[1].Factory == nil || specs[1].Settings == nil {
		t.Fatalf("specs = %+v", specs)
	}
	cfg := &config.Config{}
	cfg.HomeDash.Sources.System = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 7}
	if set := specs[0].Settings(cfg); !set.Enabled || set.Control || set.PollSeconds != 7 {
		t.Fatalf("settings = %+v (control must stay off: the source is read-only)", set)
	}
}

func haTestConfig() *config.Config {
	c := &config.Config{}
	c.HomeAssistant.Enabled, c.HomeAssistant.URL, c.HomeAssistant.AccessToken = true, " http://ha.lan:8123 ", "tok"
	c.HomeAssistant.ReadOnly = true
	c.HomeAssistant.AllowedServices, c.HomeAssistant.BlockedServices = []string{"light.turn_on"}, []string{"lock.unlock"}
	c.HomeDash.Sources.HomeAssistant = config.HomeDashSourceConfig{Enabled: true, Control: true, PollSeconds: 5}
	return c
}

func TestHomeDashHASettings(t *testing.T) {
	cfg := haTestConfig()
	set := homeDashHASettings(cfg)
	if !set.Enabled || !set.Control || set.PollSeconds != 5 || set.ConfigKey != "home_assistant" ||
		set.Params[hass.ParamURL] != "http://ha.lan:8123" || set.Params[hass.ParamToken] != "tok" || set.Params[hass.ParamReadOnly] != "true" ||
		set.Lists[hass.ListAllowedServices][0] != "light.turn_on" || set.Lists[hass.ListBlockedServices][0] != "lock.unlock" {
		t.Fatalf("settings = %+v", set)
	}
	cfg.HomeAssistant.AllowedServices[0] = "changed"
	if set.Lists[hass.ListAllowedServices][0] != "light.turn_on" {
		t.Fatal("service lists must be copies")
	}
	for name, mut := range map[string]func(*config.Config){
		"integration off": func(c *config.Config) { c.HomeAssistant.Enabled = false },
		"source off":      func(c *config.Config) { c.HomeDash.Sources.HomeAssistant.Enabled = false },
		"no url":          func(c *config.Config) { c.HomeAssistant.URL = "  " },
		"no token":        func(c *config.Config) { c.HomeAssistant.AccessToken = "" },
	} {
		c := haTestConfig()
		mut(c)
		if homeDashHASettings(c).Enabled {
			t.Errorf("%s: enabled", name)
		}
	}
	c := haTestConfig()
	c.HomeDash.Sources.HomeAssistant.Control = false
	if homeDashHASettings(c).Control {
		t.Fatal("control switch ignored")
	}
	src, ok := hass.Factory(hass.FactoryOptions{})(homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{hass.SourceID: set}})
	if !ok || src.ID() != hass.SourceID {
		t.Fatalf("factory with the settings = %v %v", src, ok)
	}
}

// Amendment X2: a configured but unusable URL is not validated here; it reaches the adapter, which
// reports unavailable/invalid_url.
func TestHomeDashHASettingsPassBadURLToTheAdapter(t *testing.T) {
	c := haTestConfig()
	c.HomeAssistant.URL = "ftp://ha.lan"
	set := homeDashHASettings(c)
	if !set.Enabled || set.Params[hass.ParamURL] != "ftp://ha.lan" {
		t.Fatalf("settings = %+v", set)
	}
	src, ok := hass.Factory(hass.FactoryOptions{})(homedash.RuntimeConfig{Sources: map[homedash.SourceID]homedash.SourceSettings{hass.SourceID: set}})
	if !ok || src == nil {
		t.Fatalf("a bad URL must still build the adapter, got %v %v", src, ok)
	}
}

// A change of the token or of a service list must change the fingerprint (hot reload reconfigures the source).
func TestHomeDashHASettingsFingerprintCoversTokenAndLists(t *testing.T) {
	ref := homeDashHASettings(haTestConfig()).Fingerprint()
	if homeDashHASettings(haTestConfig()).Fingerprint() != ref {
		t.Fatal("fingerprint is not stable")
	}
	for name, mut := range map[string]func(*config.Config){
		"token":   func(c *config.Config) { c.HomeAssistant.AccessToken = "tok2" },
		"allowed": func(c *config.Config) { c.HomeAssistant.AllowedServices = []string{"light.turn_off"} },
		"blocked": func(c *config.Config) { c.HomeAssistant.BlockedServices = nil },
		"url":     func(c *config.Config) { c.HomeAssistant.URL = "http://ha2.lan" },
		"ro":      func(c *config.Config) { c.HomeAssistant.ReadOnly = false },
	} {
		c := haTestConfig()
		mut(c)
		if homeDashHASettings(c).Fingerprint() == ref {
			t.Errorf("%s: fingerprint unchanged", name)
		}
	}
}
