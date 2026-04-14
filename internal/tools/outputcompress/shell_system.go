package outputcompress

import (
	"fmt"
	"strconv"
	"strings"
)

func compressSystemctl(sub, output string) (string, string) {
	switch sub {
	case "status":
		return compressSystemctlStatus(output), "systemctl-status"
	case "list-units", "list-unit-files":
		return compressSystemctlList(output), "systemctl-list"
	case "journalctl":
		return compressLogs(output), "journalctl"
	default:
		return compressGeneric(output), "systemctl-generic"
	}
}

// compressSystemctlStatus extracts key fields from systemctl status output.
func compressSystemctlStatus(output string) string {
	result := StripANSI(output)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	inLogs := false
	logLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Key fields to always include
		if strings.HasPrefix(trimmed, "●") || strings.HasPrefix(trimmed, "Active:") ||
			strings.HasPrefix(trimmed, "Main PID:") || strings.HasPrefix(trimmed, "Tasks:") ||
			strings.HasPrefix(trimmed, "Memory:") || strings.HasPrefix(trimmed, "CPU:") ||
			strings.HasPrefix(trimmed, "Loaded:") {
			sb.WriteString(line + "\n")
			continue
		}

		// Detect log section (indented lines after Process/Status)
		if strings.HasPrefix(line, "    ") && trimmed != "" {
			inLogs = true
		} else if !strings.HasPrefix(line, " ") && trimmed != "" {
			inLogs = false
		}

		if inLogs {
			// Include error/warning lines from logs
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
				strings.Contains(lower, "warn") || strings.Contains(lower, "fatal") {
				sb.WriteString(line + "\n")
			}
			logLines++
		}
	}

	if logLines > 20 {
		sb.WriteString(fmt.Sprintf("... and %d more log lines\n", logLines-20))
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressSystemctlList summarises systemctl list-units output.
func compressSystemctlList(output string) string {
	result := StripANSI(output)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	running, failed, exited, other := 0, 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running"):
			running++
		case strings.Contains(lower, "failed"):
			failed++
		case strings.Contains(lower, "exited"):
			exited++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary: %d Running, %d Failed, %d Exited, %d Other\n", running, failed, exited, other))

	// Include failed units
	for _, line := range lines[1:] {
		if strings.Contains(strings.ToLower(line), "failed") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// ─── JS/Test Filter ─────────────────────────────────────────────────────────

// compressJsTest extracts failures and summary from JS test runner output.
func compressDiskFree(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	// Always apply high-usage filter; only skip for trivially small output
	if len(lines) <= 2 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	highThreshold := 80.0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Parse use% field (typically last or second-to-last column)
		fields := strings.Fields(line)
		for _, f := range fields {
			f = strings.TrimSuffix(f, "%")
			pct, err := parseFloat(f)
			if err == nil && pct >= highThreshold {
				sb.WriteString(line + "\n")
				break
			}
		}
	}

	if sb.Len() <= len(lines[0])+1 {
		sb.WriteString("All filesystems below 80% usage\n")
	}
	return sb.String()
}

// compressDiskUsage summarises du output, showing largest directories.
func compressDiskUsage(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep top 15 largest entries
	var sb strings.Builder
	limit := 15
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		sb.WriteString(lines[i] + "\n")
	}
	if len(lines) > limit {
		sb.WriteString(fmt.Sprintf("... and %d more entries\n", len(lines)-limit))
	}
	return sb.String()
}

// compressProcessList summarises ps output, showing top processes.
func compressProcessList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	// Keep first 20 processes + any with high CPU/memory
	showCount := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		highResource := false
		for _, f := range fields {
			f = strings.TrimSuffix(f, "%")
			val, err := parseFloat(f)
			if err == nil && val > 50 {
				highResource = true
				break
			}
		}
		if showCount < 20 || highResource {
			sb.WriteString(line + "\n")
		}
		showCount++
	}
	if showCount > 20 {
		sb.WriteString(fmt.Sprintf("... %d total processes\n", showCount))
	}
	return sb.String()
}

// compressNetworkConnections summarises ss/netstat output.
func compressNetworkConnections(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	// Count by state
	states := make(map[string]int)
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		var state string
		switch {
		case strings.Contains(lower, "listen"):
			state = "LISTEN"
		case strings.Contains(lower, "established"):
			state = "ESTABLISHED"
		case strings.Contains(lower, "time-wait") || strings.Contains(lower, "time_wait"):
			state = "TIME-WAIT"
		case strings.Contains(lower, "close-wait") || strings.Contains(lower, "close_wait"):
			state = "CLOSE-WAIT"
		default:
			state = "OTHER"
		}
		states[state]++

		// Always include LISTEN lines
		if state == "LISTEN" {
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d LISTEN, %d ESTABLISHED, %d TIME-WAIT, %d CLOSE-WAIT, %d OTHER\n",
		states["LISTEN"], states["ESTABLISHED"], states["TIME-WAIT"], states["CLOSE-WAIT"], states["OTHER"]))
	return sb.String()
}

// compressIpAddr summarises ip addr output.
func compressIpAddr(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Interface headers (e.g., "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP>")
		if strings.Contains(line, ": ") && !strings.HasPrefix(trimmed, "link/") &&
			!strings.HasPrefix(trimmed, "inet") {
			sb.WriteString(line + "\n")
			continue
		}
		// IP addresses
		if strings.HasPrefix(trimmed, "inet ") || strings.HasPrefix(trimmed, "inet6 ") {
			sb.WriteString(line + "\n")
		}
		// State info
		if strings.Contains(strings.ToLower(trimmed), "state ") {
			sb.WriteString(line + "\n")
		}
	}
	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressIpRoute summarises ip route output.
func compressIpRoute(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 3 {
		return result
	}

	var sb strings.Builder
	defaultRoutes := 0
	otherRoutes := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "default ") {
			sb.WriteString(line + "\n")
			defaultRoutes++
		} else {
			otherRoutes++
		}
	}
	sb.WriteString(fmt.Sprintf("... and %d other routes\n", otherRoutes))
	return sb.String()
}

// parseFloat is a helper to strictly parse a float64 from a string.
// Uses strconv.ParseFloat which rejects trailing characters like "200G".
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ─── V8: Network diagnostics compressors ─────────────────────────────────────

// compressPing compresses ping output to a compact summary.
// Keeps: host status, packet loss, RTT stats. Shows error lines.
// Removes: individual ICMP reply lines (unless errors/unreachable).
//
// Input format (Linux):
//
//	PING host (IP): 56 data bytes
//	64 bytes from IP: icmp_seq=1 ttl=64 time=0.123 ms
//	--- host ping statistics ---
//	5 packets transmitted, 5 received, 0% packet loss
//	round-trip min/avg/max = 0.1/0.2/0.5 ms
