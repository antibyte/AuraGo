package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadHomeDashYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func homeDashDefaults() HomeDashConfig {
	src := func(control bool, poll int) HomeDashSourceConfig {
		return HomeDashSourceConfig{Enabled: true, Control: control, PollSeconds: poll}
	}
	return HomeDashConfig{
		Enabled: true, Agent: HomeDashAgentConfig{Access: "write"},
		Kiosk:   HomeDashKioskConfig{MaxDevices: 10, PairingTTLMinutes: 15, AllowControl: true},
		Limits:  HomeDashLimitsConfig{MaxDashboards: 50, MaxStreams: 32, MaxKioskStreams: 16, MaxSubscriptions: 300, CommandsPerMinute: 60},
		History: HomeDashHistoryConfig{RingPoints: 360, RingEntities: 500, SampleSeconds: 60},
		Sources: HomeDashSourcesConfig{
			HomeAssistant: src(true, 5), Go2RTC: src(false, 30), FritzBox: src(true, 15), MQTT: src(true, 0),
			System: src(false, 5), Docker: src(false, 60), Proxmox: src(false, 30), ThreeDPrinters: src(false, 30),
			UptimeKuma: src(false, 60), AdGuard: src(false, 30), TrueNAS: src(false, 60), Bluetooth: src(false, 0),
		},
	}
}

func TestHomeDashDefaults(t *testing.T) {
	for name, content := range map[string]string{"empty": "{}\n", "null section": "homedash:\n"} {
		cfg, err := loadHomeDashYAML(t, content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.HomeDash != homeDashDefaults() {
			t.Fatalf("%s: homedash = %+v", name, cfg.HomeDash)
		}
	}
	cfg, err := loadHomeDashYAML(t, "homedash:\n  enabled: false\n  kiosk:\n    allow_control: false\n")
	if err != nil || cfg.HomeDash.Enabled || cfg.HomeDash.Kiosk.AllowControl {
		t.Fatalf("explicit false overridden: %+v %v", cfg.HomeDash, err)
	}
}

func TestNormalizeHomeDashConfigDefaultsAndRanges(t *testing.T) {
	var zero HomeDashConfig
	if err := NormalizeHomeDashConfig(&zero); err != nil {
		t.Fatal(err)
	}
	if zero.Limits != homeDashDefaults().Limits || zero.Agent.Access != "write" || zero.Sources.Docker.PollSeconds != 60 || zero.Kiosk.DeviceTTLDays != 0 {
		t.Fatalf("zero values not defaulted: %+v", zero)
	}
	for i, mut := range []func(*HomeDashConfig){
		func(c *HomeDashConfig) { c.Limits.MaxDashboards = 500 },
		func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = 3650 },
		func(c *HomeDashConfig) { c.Sources.HomeAssistant.PollSeconds = 2 },
		func(c *HomeDashConfig) { c.Agent.Access = " READ " },
		func(c *HomeDashConfig) { c.Sources.MQTT.PollSeconds = 999 }, // no poll for mqtt: ignored
	} {
		c := homeDashDefaults()
		mut(&c)
		if err := NormalizeHomeDashConfig(&c); err != nil {
			t.Errorf("ok case %d rejected: %v", i, err)
		}
	}
	for _, tc := range []struct {
		key string
		mut func(*HomeDashConfig)
	}{
		{"limits.max_dashboards", func(c *HomeDashConfig) { c.Limits.MaxDashboards = 501 }},
		{"limits.max_streams", func(c *HomeDashConfig) { c.Limits.MaxStreams = 3 }},
		{"limits.max_kiosk_streams", func(c *HomeDashConfig) { c.Limits.MaxKioskStreams = 101 }},
		{"limits.max_subscriptions", func(c *HomeDashConfig) { c.Limits.MaxSubscriptions = 49 }},
		{"limits.commands_per_minute", func(c *HomeDashConfig) { c.Limits.CommandsPerMinute = 601 }},
		{"kiosk.max_devices", func(c *HomeDashConfig) { c.Kiosk.MaxDevices = 101 }},
		{"kiosk.pairing_ttl_minutes", func(c *HomeDashConfig) { c.Kiosk.PairingTTLMinutes = 4 }},
		{"kiosk.device_ttl_days", func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = -1 }},
		{"kiosk.device_ttl_days", func(c *HomeDashConfig) { c.Kiosk.DeviceTTLDays = 3651 }},
		{"history.ring_points", func(c *HomeDashConfig) { c.History.RingPoints = 59 }},
		{"history.ring_entities", func(c *HomeDashConfig) { c.History.RingEntities = 5001 }},
		{"history.sample_seconds", func(c *HomeDashConfig) { c.History.SampleSeconds = 9 }},
		{"sources.home_assistant.poll_seconds", func(c *HomeDashConfig) { c.Sources.HomeAssistant.PollSeconds = 1 }},
		{"sources.docker.poll_seconds", func(c *HomeDashConfig) { c.Sources.Docker.PollSeconds = 601 }},
		{"sources.uptime_kuma.poll_seconds", func(c *HomeDashConfig) { c.Sources.UptimeKuma.PollSeconds = 29 }},
		{"agent.access", func(c *HomeDashConfig) { c.Agent.Access = "admin" }},
	} {
		c := homeDashDefaults()
		tc.mut(&c)
		if err := NormalizeHomeDashConfig(&c); err == nil || !strings.Contains(err.Error(), "homedash: "+tc.key) {
			t.Errorf("%s: err = %v", tc.key, err)
		}
	}
	if _, err := loadHomeDashYAML(t, "homedash:\n  limits:\n    max_streams: 1\n"); err == nil {
		t.Error("Load accepted limits.max_streams = 1")
	}
}

func TestHomeDashTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	node, ok := doc["homedash"]
	if !ok {
		t.Fatal("config_template.yaml has no homedash: block")
	}
	raw, _ := yaml.Marshal(&node)
	var tpl HomeDashConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode: %v", err)
	}
	if err := NormalizeHomeDashConfig(&tpl); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadHomeDashYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.HomeDash {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.HomeDash)
	}
}
