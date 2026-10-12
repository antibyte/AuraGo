package remote

import (
	"encoding/json"
	"slices"
)

const ClientSecurityRevision = 1
const ClientUpgradeRequired = "CLIENT_UPGRADE_REQUIRED"
const MaxTextOutputBytes = 1 << 20

func ClassicCapabilities() []string {
	return []string{OpSysinfo, OpFileRead, OpFileWrite, OpFileList, OpFileDelete, OpShellExec, OpFileEdit, OpJsonEdit, OpYamlEdit, OpXmlEdit, OpFileSearch, OpFileReadAdv}
}

// FrameLimit allows the worst JSON escaping overhead without removing the file limit.
func FrameLimit(maxFileSizeMB int) int64 {
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = DefaultMaxFileSizeMB
	}
	if maxFileSizeMB > 500 {
		maxFileSizeMB = 500
	}
	return int64(maxFileSizeMB)*6*1024*1024 + 64*1024
}

// Unavailable measurements are unknown, not synthetic zero-percent readings.
func (h HeartbeatPayload) MarshalJSON() ([]byte, error) {
	values := map[string]interface{}{
		"cpu_percent": h.CPUPercent, "mem_percent": h.MemPercent,
		"disk_used_gb": h.DiskUsedGB, "disk_total_gb": h.DiskTotalGB, "uptime_seconds": h.Uptime,
		"hostname": h.Hostname, "os": h.OS, "arch": h.Arch, "version": h.Version,
		"metrics_available": append([]string{}, h.MetricsAvailable...),
	}
	for _, key := range []string{"cpu_percent", "mem_percent", "disk_used_gb", "disk_total_gb", "uptime_seconds"} {
		if !slices.Contains(h.MetricsAvailable, key) {
			values[key] = nil
		}
	}
	return json.Marshal(values)
}
