package server

import (
	"slices"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sources/hass"
	"aurago/internal/homedash/sources/system"
)

// homeDashSourceSpec is one HomeDash source with its server dependencies. ConfigKey is the key under
// homedash.sources and names the blocking switch in blocked_by. Settings must not block or lock
// (it runs under CfgMu during replaceConfigSnapshot); its Enabled already combines the homedash
// switch with the integration's own enabled flag. Every adapter package adds one row here.
type homeDashSourceSpec struct {
	ID        homedash.SourceID
	ConfigKey string
	Settings  func(cfg *config.Config) homedash.SourceSettings
	Factory   homedash.Factory
}

// homeDashSourceSpecs lists the sources of this build. M3: system and Home Assistant.
func (s *Server) homeDashSourceSpecs() []homeDashSourceSpec {
	return []homeDashSourceSpec{
		{
			ID:        system.SourceID,
			ConfigKey: "system",
			Settings:  homeDashSystemSettings,
			Factory:   system.Factory(s.homeDashSystemSnapshot, s.homeDashSystemHistory),
		},
		homeDashHASpec(hass.FactoryOptions{Logger: s.Logger}),
	}
}

// homeDashHASpec is the Home Assistant row; tests pass shorter websocket timings.
func homeDashHASpec(o hass.FactoryOptions) homeDashSourceSpec {
	return homeDashSourceSpec{ID: hass.SourceID, ConfigKey: "home_assistant", Settings: homeDashHASettings, Factory: hass.Factory(o)}
}

// homeDashHASettings: the source runs only when homedash.sources.home_assistant.enabled, the Home
// Assistant integration (home_assistant.enabled), a URL and a token are all present (decision D1;
// homedash.enabled is the hub's own switch). The URL is only checked for being non-empty: a configured
// but unusable URL reaches the adapter, which reports unavailable/invalid_url (amendment X2). Control is
// homedash.sources.home_assistant.control. The integration's read-only switch and service lists travel
// to the adapter, which applies the shared service policy on every call. The vault-only token is part
// of Params and so of the settings fingerprint; it is never logged. No lock is taken (runs under CfgMu).
func homeDashHASettings(cfg *config.Config) homedash.SourceSettings {
	src, ha := cfg.HomeDash.Sources.HomeAssistant, cfg.HomeAssistant
	url := strings.TrimSpace(ha.URL)
	return homedash.SourceSettings{
		Enabled:     src.Enabled && ha.Enabled && url != "" && ha.AccessToken != "",
		Control:     src.Control,
		PollSeconds: src.PollSeconds,
		ConfigKey:   "home_assistant",
		Params:      map[string]string{hass.ParamURL: url, hass.ParamToken: ha.AccessToken, hass.ParamReadOnly: strconv.FormatBool(ha.ReadOnly)},
		Lists:       map[string][]string{hass.ListAllowedServices: slices.Clone(ha.AllowedServices), hass.ListBlockedServices: slices.Clone(ha.BlockedServices)},
	}
}

// homeDashSystemSettings: the host source has no integration switch and no control.
func homeDashSystemSettings(cfg *config.Config) homedash.SourceSettings {
	src := cfg.HomeDash.Sources.System
	return homedash.SourceSettings{Enabled: src.Enabled, Control: false, PollSeconds: src.PollSeconds, ConfigKey: "system"}
}

// homeDashSystemSnapshot maps the collector's latest sample (no second sampler) for the system source.
func (s *Server) homeDashSystemSnapshot() (system.Snapshot, bool) {
	m, ok := s.loadSystemMetricsSnapshot()
	if !ok {
		return system.Snapshot{}, false
	}
	out := system.Snapshot{
		SampledAt: m.SampledAt, CPUPercent: m.CPUPercent, MemoryPercent: m.MemoryPercent, DiskPercent: m.DiskPercent,
		CPUOK: m.CPUAvailable, MemoryOK: m.MemoryAvailable, DiskOK: m.DiskAvailable, NetworkOK: m.NetworkAvailable,
		NetBytesRecv: m.NetBytesRecv, NetBytesSent: m.NetBytesSent,
	}
	if boot, ok := s.loadHostBootTime(); ok {
		out.BootTime = boot
	}
	return out, true
}
