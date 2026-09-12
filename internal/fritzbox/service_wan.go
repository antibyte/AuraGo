// Package fritzbox – WAN / internet connection service calls.
// Covers: connection status and external addresses (WANPPPConnection or
// WANIPConnection), physical link properties and byte counters
// (WANCommonInterfaceConfig) and the FRITZ!OS online monitor, which returns
// the most recent 20 throughput samples in 5-second intervals.
//
// All calls in this file are read-only. They are used by the Virtual Desktop
// Fritz!Box widget and never expose credentials, MAC addresses or port rules.
package fritzbox

import (
	"fmt"
	"strconv"
	"strings"
)

// WANStatus describes the current internet connection state.
type WANStatus struct {
	Status       string `json:"status"`        // "Connected", "Disconnected", "Connecting", ...
	Connected    bool   `json:"connected"`     // Status == "Connected"
	Uptime       int    `json:"uptime"`        // seconds since the connection was established
	LastError    string `json:"last_error"`    // "ERROR_NONE" when healthy
	ExternalIPv4 string `json:"external_ipv4"` // empty when unknown / IPv6-only
	ExternalIPv6 string `json:"external_ipv6"` // empty when the box has no IPv6 address
	IPv6Prefix   string `json:"ipv6_prefix"`   // delegated prefix, "addr/len" when available
	Service      string `json:"service"`       // "ppp" or "ip" – which TR-064 service answered
}

// WANLink describes the physical uplink and its byte counters.
type WANLink struct {
	AccessType         string `json:"access_type"`          // "DSL", "Ethernet", "X_AVM-DE_Cable", "X_AVM-DE_Fiber", ...
	LinkStatus         string `json:"link_status"`          // "Up", "Down", "Initializing", "Unavailable"
	LinkUp             bool   `json:"link_up"`              // LinkStatus == "Up"
	MaxDownstreamBps   int64  `json:"max_downstream_bps"`   // Layer 1 downstream capacity in bit/s
	MaxUpstreamBps     int64  `json:"max_upstream_bps"`     // Layer 1 upstream capacity in bit/s
	ByteSendRate       int64  `json:"byte_send_rate"`       // current upstream throughput in bytes/s
	ByteReceiveRate    int64  `json:"byte_receive_rate"`    // current downstream throughput in bytes/s
	TotalBytesSent     int64  `json:"total_bytes_sent"`     // 64-bit counter when the box provides it
	TotalBytesReceived int64  `json:"total_bytes_received"` // 64-bit counter when the box provides it
}

// OnlineMonitor holds the FRITZ!OS online monitor samples for one sync group.
// Every sample is an average over IntervalSeconds; slices are ordered oldest
// first so callers can append them to a time series directly.
type OnlineMonitor struct {
	IntervalSeconds        int     `json:"interval_seconds"` // fixed 5 s on FRITZ!OS
	GroupName              string  `json:"group_name"`       // e.g. "sync_dsl"
	GroupMode              string  `json:"group_mode"`       // e.g. "DSL", "CABLE", "FIBER"
	MaxDownstreamBytesPerS int64   `json:"max_downstream_bytes_per_s"`
	MaxUpstreamBytesPerS   int64   `json:"max_upstream_bytes_per_s"`
	DownstreamBytesPerS    []int64 `json:"downstream_bytes_per_s"` // oldest first
	UpstreamBytesPerS      []int64 `json:"upstream_bytes_per_s"`   // oldest first
	MulticastBytesPerS     []int64 `json:"multicast_bytes_per_s"`  // oldest first
}

// onlineMonitorInterval is the fixed sampling interval of the FRITZ!OS online monitor.
const onlineMonitorInterval = 5

// GetWANStatus returns the internet connection status plus external addresses.
// PPPoE uplinks (DSL) answer on WANPPPConnection, IP/cable/fiber uplinks on
// WANIPConnection; the first service that answers GetStatusInfo wins.
func (c *Client) GetWANStatus() (*WANStatus, error) {
	if !c.NetworkEnabled() {
		return nil, fmt.Errorf("fritzbox wan: network feature group is disabled")
	}
	type candidate struct {
		name, svc, ctl string
	}
	candidates := []candidate{
		{"ppp", svcWANPPPConn, ctlWANPPPConn},
		{"ip", svcWANIPConn, ctlWANIPConn},
	}
	var lastErr error
	for _, cand := range candidates {
		res, err := c.SOAP(cand.svc, cand.ctl, "GetStatusInfo", nil)
		if err != nil {
			lastErr = err
			continue
		}
		status := &WANStatus{
			Status:    res["NewConnectionStatus"],
			Connected: res["NewConnectionStatus"] == "Connected",
			Uptime:    atoiDefault(res["NewUptime"], 0),
			LastError: res["NewLastConnectionError"],
			Service:   cand.name,
		}
		// External addresses are best-effort: an IPv6-only or disconnected uplink
		// legitimately has no IPv4 address and older firmware lacks the IPv6 action.
		if ipRes, err := c.SOAP(cand.svc, cand.ctl, "GetExternalIPAddress", nil); err == nil {
			status.ExternalIPv4 = strings.TrimSpace(ipRes["NewExternalIPAddress"])
			if status.ExternalIPv4 == "0.0.0.0" {
				status.ExternalIPv4 = ""
			}
		}
		if v6Res, err := c.SOAP(cand.svc, cand.ctl, "X_AVM_DE_GetExternalIPv6Address", nil); err == nil {
			status.ExternalIPv6 = normalizeIPv6(v6Res["NewExternalIPv6Address"])
		}
		if pfxRes, err := c.SOAP(cand.svc, cand.ctl, "X_AVM_DE_GetIPv6Prefix", nil); err == nil {
			if prefix := normalizeIPv6(pfxRes["NewIPv6Prefix"]); prefix != "" {
				if length := strings.TrimSpace(pfxRes["NewPrefixLength"]); length != "" && length != "0" {
					prefix += "/" + length
				}
				status.IPv6Prefix = prefix
			}
		}
		return status, nil
	}
	return nil, fmt.Errorf("fritzbox wan: GetStatusInfo: %w", lastErr)
}

// GetWANLinkInfo returns the physical link properties and current byte counters.
func (c *Client) GetWANLinkInfo() (*WANLink, error) {
	if !c.NetworkEnabled() {
		return nil, fmt.Errorf("fritzbox wan: network feature group is disabled")
	}
	link := &WANLink{}
	props, err := c.SOAP(svcWANCommonIfConfig, ctlWANCommonIfConfig, "GetCommonLinkProperties", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox wan: GetCommonLinkProperties: %w", err)
	}
	link.AccessType = props["NewWANAccessType"]
	link.LinkStatus = props["NewPhysicalLinkStatus"]
	link.LinkUp = link.LinkStatus == "Up"
	link.MaxDownstreamBps = parseInt64(props["NewLayer1DownstreamMaxBitRate"])
	link.MaxUpstreamBps = parseInt64(props["NewLayer1UpstreamMaxBitRate"])

	// Byte counters are optional: some bridged setups reject GetAddonInfos.
	if addon, err := c.SOAP(svcWANCommonIfConfig, ctlWANCommonIfConfig, "GetAddonInfos", nil); err == nil {
		link.ByteSendRate = parseInt64(addon["NewByteSendRate"])
		link.ByteReceiveRate = parseInt64(addon["NewByteReceiveRate"])
		link.TotalBytesSent = firstPositiveInt64(addon["NewX_AVM_DE_TotalBytesSent64"], addon["NewTotalBytesSent"])
		link.TotalBytesReceived = firstPositiveInt64(addon["NewX_AVM_DE_TotalBytesReceived64"], addon["NewTotalBytesReceived"])
	}
	return link, nil
}

// GetOnlineMonitor returns the most recent throughput samples of sync group 0.
// FRITZ!OS reports the newest sample first; the result is reordered oldest first.
// Values are bytes per second despite the "_bps" suffix used by the firmware.
func (c *Client) GetOnlineMonitor() (*OnlineMonitor, error) {
	if !c.NetworkEnabled() {
		return nil, fmt.Errorf("fritzbox wan: network feature group is disabled")
	}
	res, err := c.SOAP(svcWANCommonIfConfig, ctlWANCommonIfConfig, "X_AVM-DE_GetOnlineMonitor",
		map[string]string{"NewSyncGroupIndex": "0"})
	if err != nil {
		return nil, fmt.Errorf("fritzbox wan: X_AVM-DE_GetOnlineMonitor: %w", err)
	}
	return &OnlineMonitor{
		IntervalSeconds:        onlineMonitorInterval,
		GroupName:              res["NewSyncGroupName"],
		GroupMode:              res["NewSyncGroupMode"],
		MaxDownstreamBytesPerS: parseInt64(res["Newmax_ds"]),
		MaxUpstreamBytesPerS:   parseInt64(res["Newmax_us"]),
		DownstreamBytesPerS:    parseMonitorSeries(res["Newds_current_bps"]),
		UpstreamBytesPerS:      parseMonitorSeries(res["Newus_current_bps"]),
		MulticastBytesPerS:     parseMonitorSeries(res["Newmc_current_bps"]),
	}, nil
}

// parseMonitorSeries splits a comma-separated newest-first sample list into an
// oldest-first int64 slice. Empty or malformed entries become 0 so the series
// keeps its fixed length and stays aligned with the sibling series.
func parseMonitorSeries(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, len(parts))
	for i, part := range parts {
		out[len(parts)-1-i] = parseInt64(part)
	}
	return out
}

func parseInt64(raw string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func firstPositiveInt64(candidates ...string) int64 {
	for _, raw := range candidates {
		if v := parseInt64(raw); v > 0 {
			return v
		}
	}
	return 0
}

func atoiDefault(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

// normalizeIPv6 drops the "::" and "0:0:..." placeholders FRITZ!OS returns
// when no IPv6 address is configured.
func normalizeIPv6(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "::" {
		return ""
	}
	if strings.Trim(raw, "0:") == "" {
		return ""
	}
	return raw
}
