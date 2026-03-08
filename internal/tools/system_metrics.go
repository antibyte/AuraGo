package tools

import (
	"encoding/json"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// MetricsResult is the JSON response returned to the LLM.
type MetricsResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SystemMetrics holds all collected metrics.
type SystemMetrics struct {
	CPU     CPUMetrics     `json:"cpu"`
	Memory  MemoryMetrics  `json:"memory"`
	Disk    DiskMetrics    `json:"disk"`
	Network NetworkMetrics `json:"network"`
}

type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
	ModelName    string  `json:"model_name"`
}

type MemoryMetrics struct {
	Total       uint64  `json:"total"`
	Available   uint64  `json:"available"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskMetrics struct {
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

type NetworkMetrics struct {
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
}

// GetSystemMetrics collects platform-independent system metrics.
func GetSystemMetrics() string {
	encode := func(r MetricsResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	metrics := SystemMetrics{}

	// CPU
	usage, err := cpu.Percent(time.Second, false)
	if err == nil && len(usage) > 0 {
		metrics.CPU.UsagePercent = usage[0]
	}
	info, err := cpu.Info()
	if err == nil && len(info) > 0 {
		metrics.CPU.Cores = int(info[0].Cores)
		metrics.CPU.ModelName = info[0].ModelName
	}

	// Memory
	vm, err := mem.VirtualMemory()
	if err == nil {
		metrics.Memory.Total = vm.Total
		metrics.Memory.Available = vm.Available
		metrics.Memory.Used = vm.Used
		metrics.Memory.UsedPercent = vm.UsedPercent
	}

	// Disk
	usageDisk, err := disk.Usage("/")
	if err == nil {
		metrics.Disk.Total = usageDisk.Total
		metrics.Disk.Free = usageDisk.Free
		metrics.Disk.Used = usageDisk.Used
		metrics.Disk.UsedPercent = usageDisk.UsedPercent
	}

	// Network
	io, err := net.IOCounters(false)
	if err == nil && len(io) > 0 {
		metrics.Network.BytesSent = io[0].BytesSent
		metrics.Network.BytesRecv = io[0].BytesRecv
	}

	return encode(MetricsResult{
		Status:  "success",
		Message: "System metrics collected successfully",
		Data:    metrics,
	})
}
