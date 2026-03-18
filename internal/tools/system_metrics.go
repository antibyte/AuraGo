package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"
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

// HostInfo holds OS / uptime information.
type HostInfo struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	Uptime          uint64 `json:"uptime_seconds"`
	UptimeHuman     string `json:"uptime_human"`
	BootTime        uint64 `json:"boot_time"`
}

// TempSensor represents a single temperature reading.
type TempSensor struct {
	SensorKey   string  `json:"sensor_key"`
	Temperature float64 `json:"temperature_celsius"`
	High        float64 `json:"high,omitempty"`
	Critical    float64 `json:"critical,omitempty"`
}

// NetworkInterfaceStats holds per-interface I/O counters.
type NetworkInterfaceStats struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	ErrIn       uint64 `json:"errors_in"`
	ErrOut      uint64 `json:"errors_out"`
	DropIn      uint64 `json:"drops_in"`
	DropOut     uint64 `json:"drops_out"`
}

// DiskIOStats holds per-disk I/O counters.
type DiskIOStats struct {
	Name       string `json:"name"`
	ReadCount  uint64 `json:"read_count"`
	WriteCount uint64 `json:"write_count"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadMs     uint64 `json:"read_ms"`
	WriteMs    uint64 `json:"write_ms"`
}

// NetworkConnection represents a single active network connection.
type NetworkConnection struct {
	FD     uint32 `json:"fd"`
	Family string `json:"family"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Laddr  string `json:"local_addr"`
	Raddr  string `json:"remote_addr"`
	PID    int32  `json:"pid"`
}

// humanUptime converts seconds to a human-readable string.
func humanUptime(secs uint64) string {
	d := secs / 86400
	h := (secs % 86400) / 3600
	m := (secs % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// GetSystemMetrics collects platform-independent system metrics.
// target selects which metrics to return: "all", "cpu", "memory", "disk",
// "processes", "host", "sensors", "network_detail", "connections", "disk_io".
func GetSystemMetrics(target string) string {
	encode := func(r MetricsResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch target {
	case "host":
		return encode(MetricsResult{Status: "success", Data: getHostInfo()})
	case "sensors":
		return encode(MetricsResult{Status: "success", Data: getSensorData()})
	case "network_detail":
		return encode(MetricsResult{Status: "success", Data: getNetworkDetail()})
	case "connections":
		return encode(MetricsResult{Status: "success", Data: getNetworkConnections()})
	case "disk_io":
		return encode(MetricsResult{Status: "success", Data: getDiskIOStats()})
	}

	// "all", "cpu", "memory", "disk", "processes" — original behaviour
	metrics := SystemMetrics{}

	// CPU
	if target == "all" || target == "cpu" || target == "" {
		usage, err := cpu.Percent(time.Second, false)
		if err == nil && len(usage) > 0 {
			metrics.CPU.UsagePercent = usage[0]
		}
		info, err := cpu.Info()
		if err == nil && len(info) > 0 {
			metrics.CPU.Cores = int(info[0].Cores)
			metrics.CPU.ModelName = info[0].ModelName
		}
	}

	// Memory
	if target == "all" || target == "memory" || target == "" {
		vm, err := mem.VirtualMemory()
		if err == nil {
			metrics.Memory.Total = vm.Total
			metrics.Memory.Available = vm.Available
			metrics.Memory.Used = vm.Used
			metrics.Memory.UsedPercent = vm.UsedPercent
		}
	}

	// Disk
	if target == "all" || target == "disk" || target == "" {
		usageDisk, err := disk.Usage("/")
		if err == nil {
			metrics.Disk.Total = usageDisk.Total
			metrics.Disk.Free = usageDisk.Free
			metrics.Disk.Used = usageDisk.Used
			metrics.Disk.UsedPercent = usageDisk.UsedPercent
		}
	}

	// Network (aggregate)
	if target == "all" || target == "" {
		io, err := net.IOCounters(false)
		if err == nil && len(io) > 0 {
			metrics.Network.BytesSent = io[0].BytesSent
			metrics.Network.BytesRecv = io[0].BytesRecv
		}
	}

	return encode(MetricsResult{
		Status:  "success",
		Message: "System metrics collected successfully",
		Data:    metrics,
	})
}

// getHostInfo returns OS / uptime information.
func getHostInfo() HostInfo {
	info, err := host.Info()
	if err != nil {
		return HostInfo{}
	}
	return HostInfo{
		Hostname:        info.Hostname,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		Uptime:          info.Uptime,
		UptimeHuman:     humanUptime(info.Uptime),
		BootTime:        info.BootTime,
	}
}

// getSensorData returns temperature readings (best-effort; may be empty on VMs).
func getSensorData() []TempSensor {
	temps, err := sensors.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return []TempSensor{}
	}
	result := make([]TempSensor, 0, len(temps))
	for _, t := range temps {
		result = append(result, TempSensor{
			SensorKey:   t.SensorKey,
			Temperature: t.Temperature,
			High:        t.High,
			Critical:    t.Critical,
		})
	}
	return result
}

// getNetworkDetail returns per-interface I/O counters.
func getNetworkDetail() []NetworkInterfaceStats {
	counters, err := net.IOCounters(true)
	if err != nil {
		return nil
	}
	result := make([]NetworkInterfaceStats, 0, len(counters))
	for _, c := range counters {
		result = append(result, NetworkInterfaceStats{
			Name:        c.Name,
			BytesSent:   c.BytesSent,
			BytesRecv:   c.BytesRecv,
			PacketsSent: c.PacketsSent,
			PacketsRecv: c.PacketsRecv,
			ErrIn:       c.Errin,
			ErrOut:      c.Errout,
			DropIn:      c.Dropin,
			DropOut:     c.Dropout,
		})
	}
	return result
}

// getNetworkConnections returns active TCP/UDP connections.
func getNetworkConnections() []NetworkConnection {
	conns, err := net.Connections("inet")
	if err != nil {
		return nil
	}
	result := make([]NetworkConnection, 0, len(conns))
	for _, c := range conns {
		family := "IPv4"
		if c.Family == 10 {
			family = "IPv6"
		}
		connType := "TCP"
		if c.Type == 2 {
			connType = "UDP"
		}
		laddr := fmt.Sprintf("%s:%d", c.Laddr.IP, c.Laddr.Port)
		raddr := ""
		if c.Raddr.IP != "" {
			raddr = fmt.Sprintf("%s:%d", c.Raddr.IP, c.Raddr.Port)
		}
		result = append(result, NetworkConnection{
			FD:     c.Fd,
			Family: family,
			Type:   connType,
			Status: c.Status,
			Laddr:  laddr,
			Raddr:  raddr,
			PID:    c.Pid,
		})
	}
	return result
}

// getDiskIOStats returns per-disk I/O counters.
func getDiskIOStats() []DiskIOStats {
	counters, err := disk.IOCounters()
	if err != nil {
		return []DiskIOStats{}
	}
	result := make([]DiskIOStats, 0, len(counters))
	for _, c := range counters {
		result = append(result, DiskIOStats{
			Name:       c.Name,
			ReadCount:  c.ReadCount,
			WriteCount: c.WriteCount,
			ReadBytes:  c.ReadBytes,
			WriteBytes: c.WriteBytes,
			ReadMs:     c.ReadTime,
			WriteMs:    c.WriteTime,
		})
	}
	return result
}
