package outputcompress

import (
	"fmt"
	"strings"
)


// ─── Kubernetes Filters ─────────────────────────────────────────────────────

func compressK8s(sub, output string) (string, string) {
	switch sub {
	case "logs":
		return compressK8sLogs(output), "k8s-logs"
	case "get":
		return compressK8sGet(output), "k8s-get"
	case "describe":
		return compressK8sDescribe(output), "k8s-describe"
	case "top":
		return compressGeneric(output), "k8s-top"
	default:
		return compressGeneric(output), "k8s-generic"
	}
}

// compressK8sLogs applies log-specific compression with level grouping.
func compressK8sLogs(output string) string {
	return compressLogOutput(output)
}

// compressK8sGet summarises kubectl get output into status groups.
func compressK8sGet(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	running, pending, failed, other := 0, 0, 0, 0
	for _, line := range lines[1:] { // skip header
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running"):
			running++
		case strings.Contains(lower, "pending") || strings.Contains(lower, "containercreating"):
			pending++
		case strings.Contains(lower, "error") || strings.Contains(lower, "crashloop") || strings.Contains(lower, "failed"):
			failed++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Status Summary: %d Running, %d Pending, %d Failed, %d Other\n", running, pending, failed, other))

	// Include failed/pending lines for context
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "crashloop") ||
			strings.Contains(lower, "failed") || strings.Contains(lower, "pending") ||
			strings.Contains(lower, "containercreating") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressK8sDescribe extracts key information from kubectl describe output.
func compressK8sDescribe(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	inEvents := false
	inConditions := false
	eventCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always include Name, Status, and key labels
		if strings.HasPrefix(trimmed, "Name:") ||
			strings.HasPrefix(trimmed, "Status:") ||
			strings.HasPrefix(trimmed, "Node:") ||
			strings.HasPrefix(trimmed, "Labels:") {
			sb.WriteString(line + "\n")
			continue
		}

		// Track Events section
		if strings.HasPrefix(trimmed, "Events:") {
			inEvents = true
			inConditions = false
			sb.WriteString(line + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "Conditions:") {
			inConditions = true
			inEvents = false
			sb.WriteString(line + "\n")
			continue
		}

		// In Conditions section, include all lines
		if inConditions && (strings.HasPrefix(trimmed, "Type") || strings.HasPrefix(trimmed, "Ready") ||
			strings.HasPrefix(trimmed, "  ")) {
			sb.WriteString(line + "\n")
			continue
		}

		// In Events section, include warnings and last few events
		if inEvents {
			if strings.Contains(strings.ToLower(trimmed), "warning") || strings.Contains(strings.ToLower(trimmed), "error") {
				sb.WriteString(line + "\n")
			}
			eventCount++
		}
	}

	if eventCount > 10 && sb.Len() > 0 {
		// Add summary if too many events
		sb.WriteString(fmt.Sprintf("... and %d more events\n", eventCount-10))
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result // too little extracted, return original
	}
	return compressed
}

// ─── Test Runner Filters ────────────────────────────────────────────────────

// compressGoTest extracts failures and summary from go test output.
