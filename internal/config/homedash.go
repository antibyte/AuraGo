package config

import (
	"fmt"
	"strings"
)

// HomeDashConfig configures the HomeDash dashboards, wall-display kiosk and entity hub.
// Bool defaults (enabled, kiosk.allow_control, sources.*.enabled, control for HA/FritzBox/MQTT)
// are set by the loader before YAML decoding.
type HomeDashConfig struct {
	Enabled  bool                  `yaml:"enabled" json:"enabled"`
	ReadOnly bool                  `yaml:"readonly" json:"readonly"`
	Agent    HomeDashAgentConfig   `yaml:"agent" json:"agent"`
	Kiosk    HomeDashKioskConfig   `yaml:"kiosk" json:"kiosk"`
	Limits   HomeDashLimitsConfig  `yaml:"limits" json:"limits"`
	History  HomeDashHistoryConfig `yaml:"history" json:"history"`
	Sources  HomeDashSourcesConfig `yaml:"sources" json:"sources"`
}

type HomeDashAgentConfig struct {
	Access string `yaml:"access" json:"access"` // off | read | write (default write)
}

type HomeDashKioskConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	MaxDevices        int  `yaml:"max_devices" json:"max_devices"`                 // 1..100, default 10
	PairingTTLMinutes int  `yaml:"pairing_ttl_minutes" json:"pairing_ttl_minutes"` // 5..60, default 15
	DeviceTTLDays     int  `yaml:"device_ttl_days" json:"device_ttl_days"`         // 0 = until revoked, else 1..3650
	AllowControl      bool `yaml:"allow_control" json:"allow_control"`
}

type HomeDashLimitsConfig struct {
	MaxDashboards     int `yaml:"max_dashboards" json:"max_dashboards"`
	MaxStreams        int `yaml:"max_streams" json:"max_streams"`
	MaxKioskStreams   int `yaml:"max_kiosk_streams" json:"max_kiosk_streams"`
	MaxSubscriptions  int `yaml:"max_subscriptions" json:"max_subscriptions"`
	CommandsPerMinute int `yaml:"commands_per_minute" json:"commands_per_minute"`
}

type HomeDashHistoryConfig struct {
	RingPoints    int `yaml:"ring_points" json:"ring_points"`
	RingEntities  int `yaml:"ring_entities" json:"ring_entities"`
	SampleSeconds int `yaml:"sample_seconds" json:"sample_seconds"`
}

// HomeDashSourceConfig is one source. Sources without control in v1 ignore Control; mqtt and
// bluetooth ignore PollSeconds.
type HomeDashSourceConfig struct {
	Enabled     bool `yaml:"enabled" json:"enabled"`
	Control     bool `yaml:"control" json:"control"`
	PollSeconds int  `yaml:"poll_seconds" json:"poll_seconds"`
}

type HomeDashSourcesConfig struct {
	HomeAssistant  HomeDashSourceConfig `yaml:"home_assistant" json:"home_assistant"`
	Go2RTC         HomeDashSourceConfig `yaml:"go2rtc" json:"go2rtc"`
	FritzBox       HomeDashSourceConfig `yaml:"fritzbox" json:"fritzbox"`
	MQTT           HomeDashSourceConfig `yaml:"mqtt" json:"mqtt"`
	System         HomeDashSourceConfig `yaml:"system" json:"system"`
	Docker         HomeDashSourceConfig `yaml:"docker" json:"docker"`
	Proxmox        HomeDashSourceConfig `yaml:"proxmox" json:"proxmox"`
	ThreeDPrinters HomeDashSourceConfig `yaml:"three_d_printers" json:"three_d_printers"`
	UptimeKuma     HomeDashSourceConfig `yaml:"uptime_kuma" json:"uptime_kuma"`
	AdGuard        HomeDashSourceConfig `yaml:"adguard" json:"adguard"`
	TrueNAS        HomeDashSourceConfig `yaml:"truenas" json:"truenas"`
	Bluetooth      HomeDashSourceConfig `yaml:"bluetooth" json:"bluetooth"`
}

// NormalizeHomeDashConfig applies defaults for zero values and rejects values outside their
// range or an unknown agent.access. It does not change booleans.
func NormalizeHomeDashConfig(c *HomeDashConfig) error {
	c.Agent.Access = strings.ToLower(strings.TrimSpace(c.Agent.Access))
	switch c.Agent.Access {
	case "":
		c.Agent.Access = "write"
	case "off", "read", "write":
	default:
		return fmt.Errorf("homedash: agent.access must be off, read or write")
	}
	if c.Kiosk.DeviceTTLDays < 0 || c.Kiosk.DeviceTTLDays > 3650 {
		return fmt.Errorf("homedash: kiosk.device_ttl_days must be 0–3650")
	}
	s := &c.Sources
	for _, f := range []struct {
		p           *int
		def, lo, hi int
		key         string
	}{
		{&c.Kiosk.MaxDevices, 10, 1, 100, "kiosk.max_devices"},
		{&c.Kiosk.PairingTTLMinutes, 15, 5, 60, "kiosk.pairing_ttl_minutes"},
		{&c.Limits.MaxDashboards, 50, 1, 500, "limits.max_dashboards"},
		{&c.Limits.MaxStreams, 32, 4, 128, "limits.max_streams"},
		{&c.Limits.MaxKioskStreams, 16, 1, 100, "limits.max_kiosk_streams"},
		{&c.Limits.MaxSubscriptions, 300, 50, 1000, "limits.max_subscriptions"},
		{&c.Limits.CommandsPerMinute, 60, 10, 600, "limits.commands_per_minute"},
		{&c.History.RingPoints, 360, 60, 2880, "history.ring_points"},
		{&c.History.RingEntities, 500, 50, 5000, "history.ring_entities"},
		{&c.History.SampleSeconds, 60, 10, 600, "history.sample_seconds"},
		{&s.HomeAssistant.PollSeconds, 5, 2, 60, "sources.home_assistant.poll_seconds"},
		{&s.Go2RTC.PollSeconds, 30, 10, 300, "sources.go2rtc.poll_seconds"},
		{&s.FritzBox.PollSeconds, 15, 10, 300, "sources.fritzbox.poll_seconds"},
		{&s.System.PollSeconds, 5, 2, 60, "sources.system.poll_seconds"},
		{&s.Docker.PollSeconds, 60, 15, 600, "sources.docker.poll_seconds"},
		{&s.Proxmox.PollSeconds, 30, 10, 600, "sources.proxmox.poll_seconds"},
		{&s.ThreeDPrinters.PollSeconds, 30, 10, 600, "sources.three_d_printers.poll_seconds"},
		{&s.UptimeKuma.PollSeconds, 60, 30, 600, "sources.uptime_kuma.poll_seconds"},
		{&s.AdGuard.PollSeconds, 30, 15, 600, "sources.adguard.poll_seconds"},
		{&s.TrueNAS.PollSeconds, 60, 30, 600, "sources.truenas.poll_seconds"},
	} {
		if *f.p == 0 {
			*f.p = f.def
		}
		if *f.p < f.lo || *f.p > f.hi {
			return fmt.Errorf("homedash: %s must be %d–%d", f.key, f.lo, f.hi)
		}
	}
	return nil
}
