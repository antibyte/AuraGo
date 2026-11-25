package server

import (
	"aurago/internal/config"
	"aurago/internal/homedash"
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

// homeDashSourceSpecs lists the sources of this build. M2: system only.
func (s *Server) homeDashSourceSpecs() []homeDashSourceSpec {
	return []homeDashSourceSpec{
		{
			ID:        system.SourceID,
			ConfigKey: "system",
			Settings:  homeDashSystemSettings,
			Factory:   system.Factory(s.homeDashSystemSnapshot, s.homeDashSystemHistory),
		},
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
