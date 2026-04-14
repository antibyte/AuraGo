package outputcompress

import (
	"fmt"
	"strings"
)

// ─── Container Filters ──────────────────────────────────────────────────────

func compressContainer(sub, output string) (string, string) {
	switch sub {
	case "ps":
		return compressDockerPS(output), "docker-ps"
	case "logs":
		return compressDockerLogs(output), "docker-logs"
	case "images":
		return compressGeneric(output), "docker-images"
	default:
		return compressGeneric(output), "docker-generic"
	}
}

// compressDockerPS strips container hashes and unnecessary columns.
func compressDockerPS(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 1 {
		return result
	}

	// Try to parse as table - keep Name, Status, Ports, Image
	var sb strings.Builder
	for i, line := range lines {
		if line == "" {
			continue
		}
		// Keep header
		if i == 0 {
			sb.WriteString(line + "\n")
			continue
		}
		// Strip container ID hash (first column, 12+ hex chars)
		fields := strings.Fields(line)
		if len(fields) >= 2 && isContainerID(fields[0]) {
			// Rebuild without the ID column
			sb.WriteString(strings.Join(fields[1:], " ") + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

// compressDockerLogs applies log-specific compression.
func compressDockerLogs(output string) string {
	return compressLogOutput(output)
}
func compressDockerCompose(sub string, output string) (string, string) {
	switch sub {
	case "ps":
		return compressComposePs(output), "compose-ps"
	case "logs":
		return compressDockerLogs(output), "compose-logs"
	case "config":
		return compressComposeConfig(output), "compose-config"
	case "events":
		return compressComposeEvents(output), "compose-events"
	case "images":
		return compressComposePs(output), "compose-images" // similar table format
	case "top":
		return compressGeneric(output), "compose-top"
	default:
		return compressGeneric(output), "compose-generic"
	}
}

// compressComposePs summarises docker compose ps output.
func compressComposePs(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	running, stopped, other := 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running") || strings.Contains(lower, "up"):
			running++
		case strings.Contains(lower, "stopped") || strings.Contains(lower, "exited") ||
			strings.Contains(lower, "down") || strings.Contains(lower, "dead"):
			stopped++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Services: %d Running, %d Stopped, %d Other\n", running, stopped, other))

	// Include header + stopped/error services
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "exited") ||
			strings.Contains(lower, "down") || strings.Contains(lower, "dead") ||
			strings.Contains(lower, "error") || strings.Contains(lower, "restart") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressComposeConfig summarises docker compose config output.
func compressComposeConfig(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 20 {
		return result
	}

	var sb strings.Builder
	serviceCount := 0
	networkCount := 0
	volumeCount := 0
	inServices := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "services:") {
			inServices = true
			sb.WriteString(line + "\n")
			continue
		}
		if inServices {
			// Detect individual service names (2-space indent + name + colon)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
				serviceCount++
				sb.WriteString(line + "\n")
			}
			if strings.HasPrefix(trimmed, "networks:") || strings.HasPrefix(trimmed, "volumes:") {
				inServices = false
			}
		}
		if strings.HasPrefix(trimmed, "networks:") {
			networkCount++
			sb.WriteString(line + "\n")
		}
		if strings.HasPrefix(trimmed, "volumes:") {
			volumeCount++
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nConfig: %d services, %d networks, %d volumes (full config omitted)\n",
		serviceCount, networkCount, volumeCount))
	return sb.String()
}

// compressComposeEvents extracts key events from docker compose events output.
func compressComposeEvents(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	result = stripTimestamps(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	eventCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "die") || strings.Contains(lower, "error") ||
			strings.Contains(lower, "kill") || strings.Contains(lower, "stop") ||
			strings.Contains(lower, "restart") || strings.Contains(lower, "health") {
			sb.WriteString(line + "\n")
		}
		eventCount++
	}

	sb.WriteString(fmt.Sprintf("... %d total events\n", eventCount))
	return sb.String()
}

// ─── Helm Filters ────────────────────────────────────────────────────────────

// compressHelm routes helm subcommands.
func isContainerID(s string) bool {
	if len(s) < 12 {
		return false
	}
	for _, c := range s[:12] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// stripTimestamps removes common log timestamp prefixes.
