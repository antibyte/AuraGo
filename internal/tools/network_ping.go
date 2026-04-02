// Package tools – network_ping: ICMP ping with statistics via pro-bing.
package tools

import (
	"encoding/json"
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// pingResult is the JSON payload returned by NetworkPing.
type pingResult struct {
	Status      string   `json:"status"`
	Host        string   `json:"host"`
	IPAddr      string   `json:"ip_addr,omitempty"`
	PacketsSent int      `json:"packets_sent"`
	PacketsRecv int      `json:"packets_recv"`
	PacketLoss  float64  `json:"packet_loss_percent"`
	MinRTT      string   `json:"min_rtt,omitempty"`
	AvgRTT      string   `json:"avg_rtt,omitempty"`
	MaxRTT      string   `json:"max_rtt,omitempty"`
	StdDevRTT   string   `json:"stddev_rtt,omitempty"`
	RTTs        []string `json:"rtts,omitempty"`
	Message     string   `json:"message,omitempty"`
}

// NetworkPing sends ICMP echo requests to host and returns statistics.
// count     – number of packets (1–20; defaults to 4)
// timeoutSecs – total timeout in seconds (1–60; defaults to 10)
//
// Note: ICMP raw sockets typically require elevated privileges on Linux/macOS.
// On Windows this usually works without elevation. The tool sets Privileged(true);
// if the process lacks the necessary capability the error is reported clearly.
func NetworkPing(targetHost string, count, timeoutSecs int) string {
	encode := func(r pingResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if targetHost == "" {
		return encode(pingResult{Status: "error", Message: "host is required"})
	}

	// Clamp parameters to sane ranges
	if count <= 0 || count > 20 {
		count = 4
	}
	if timeoutSecs <= 0 || timeoutSecs > 60 {
		timeoutSecs = 10
	}

	pinger, err := probing.NewPinger(targetHost)
	if err != nil {
		return encode(pingResult{
			Status:  "error",
			Host:    targetHost,
			Message: fmt.Sprintf("failed to create pinger: %v", err),
		})
	}

	pinger.Count = count
	pinger.Timeout = time.Duration(timeoutSecs) * time.Second
	pinger.SetPrivileged(true) // try privileged ICMP first (requires root / CAP_NET_RAW on Linux)

	if err := pinger.Run(); err != nil {
		// Fall back to unprivileged UDP ping (works without elevated privileges on most Linux systems)
		pinger2, err2 := probing.NewPinger(targetHost)
		if err2 != nil {
			return encode(pingResult{
				Status:  "error",
				Host:    targetHost,
				Message: fmt.Sprintf("ping failed: %v — ensure the process has permission to send ICMP packets (root / CAP_NET_RAW on Linux)", err),
			})
		}
		pinger2.Count = count
		pinger2.Timeout = time.Duration(timeoutSecs) * time.Second
		pinger2.SetPrivileged(false) // unprivileged UDP ping
		if err2 = pinger2.Run(); err2 != nil {
			return encode(pingResult{
				Status:  "error",
				Host:    targetHost,
				Message: fmt.Sprintf("ping failed (privileged: %v; unprivileged: %v) — ensure the host is reachable and ICMP is allowed", err, err2),
			})
		}
		pinger = pinger2
	}

	stats := pinger.Statistics()
	rtts := make([]string, 0, len(stats.Rtts))
	for _, r := range stats.Rtts {
		rtts = append(rtts, r.String())
	}

	return encode(pingResult{
		Status:      "success",
		Host:        stats.Addr,
		IPAddr:      stats.IPAddr.String(),
		PacketsSent: stats.PacketsSent,
		PacketsRecv: stats.PacketsRecv,
		PacketLoss:  stats.PacketLoss,
		MinRTT:      stats.MinRtt.String(),
		AvgRTT:      stats.AvgRtt.String(),
		MaxRTT:      stats.MaxRtt.String(),
		StdDevRTT:   stats.StdDevRtt.String(),
		RTTs:        rtts,
	})
}
