package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"aurago/internal/remote"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

func (e *Executor) CollectSysinfo() remote.HeartbeatPayload {
	name, _ := os.Hostname()
	hb := remote.HeartbeatPayload{Hostname: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: BuildVersion, MetricsAvailable: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if v, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(v) > 0 {
		hb.CPUPercent = v[0]
		hb.MetricsAvailable = append(hb.MetricsAvailable, "cpu_percent")
	}
	if v, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		hb.MemPercent = v.UsedPercent
		hb.MetricsAvailable = append(hb.MetricsAvailable, "mem_percent")
	}
	path := string(os.PathSeparator)
	if runtime.GOOS == "windows" {
		path = filepath.VolumeName(os.Getenv("SystemRoot")) + `\`
	}
	if v, err := disk.UsageWithContext(ctx, path); err == nil {
		hb.DiskUsedGB = float64(v.Used) / (1 << 30)
		hb.DiskTotalGB = float64(v.Total) / (1 << 30)
		hb.MetricsAvailable = append(hb.MetricsAvailable, "disk_used_gb", "disk_total_gb")
	}
	if v, err := host.UptimeWithContext(ctx); err == nil {
		hb.Uptime = int64(v)
		hb.MetricsAvailable = append(hb.MetricsAvailable, "uptime_seconds")
	}
	return hb
}
