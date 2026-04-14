// Package outputcompress – process tool output compressors.
//
// list_processes and read_process_logs return plain-text outputs:
//   - list_processes: "Tool Output: Active processes:\n- PID: N, Started: ts\n..."
//   - read_process_logs: "Tool Output: [LOGS for PID N]\n<output>"
//
// Strategy:
//   - list_processes: strip "Tool Output:" prefix, compact PID list
//   - read_process_logs: strip prefix, apply TailFocus + DeduplicateLines to log body
package outputcompress

import (
	"fmt"
	"strings"
)

// compressProcessOutput routes process tool outputs to sub-compressors.
func compressProcessOutput(toolName, output string) (string, string) {
	switch toolName {
	case "list_processes":
		return compressListProcesses(output), "proc-list"
	case "read_process_logs":
		return compressReadProcessLogs(output), "proc-logs"
	default:
		return compressGeneric(output), "proc-generic"
	}
}

// compressListProcesses compacts the list_processes output.
//
// From:
//
//	Tool Output: Active processes:
//	- PID: 12345, Started: 2024-01-15T10:30:00Z
//	- PID: 67890, Started: 2024-01-15T10:35:00Z
//
// To:
//
//	3 processes: 12345, 67890, 11111
//
// Empty case ("No active background processes.") is returned as-is.
func compressListProcesses(output string) string {
	clean := strings.TrimSpace(output)

	// Strip "Tool Output: " prefix
	clean = strings.TrimPrefix(clean, "Tool Output: ")

	// Empty case
	if strings.HasPrefix(clean, "No active background processes") {
		return "No active processes."
	}

	// Parse PID lines
	lines := strings.Split(clean, "\n")
	var pids []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Match "- PID: N, Started: ts"
		if strings.HasPrefix(line, "- PID:") {
			// Extract just the PID number
			rest := strings.TrimPrefix(line, "- PID:")
			rest = strings.TrimSpace(rest)
			if comma := strings.Index(rest, ","); comma >= 0 {
				pids = append(pids, rest[:comma])
			} else {
				pids = append(pids, rest)
			}
		}
	}

	if len(pids) == 0 {
		return clean
	}

	return fmt.Sprintf("%d processes: %s", len(pids), strings.Join(pids, ", "))
}

// compressReadProcessLogs compacts read_process_logs output.
//
// From:
//
//	Tool Output: [LOGS for PID 12345]
//	<very long log output with repeated lines>
//
// To:
//
//	[PID 12345] (last 80 lines of 500, 200 duplicates removed)
//	<compressed log body>
//
// The log body is compressed using TailFocus (keeps head/tail with gap marker)
// and DeduplicateLines to remove consecutive duplicates.
func compressReadProcessLogs(output string) string {
	clean := strings.TrimSpace(output)

	// Strip "Tool Output: " prefix
	clean = strings.TrimPrefix(clean, "Tool Output: ")

	// Extract PID from header line "[LOGS for PID N]"
	var pidInfo string
	var body string
	if strings.HasPrefix(clean, "[LOGS for PID") {
		idx := strings.Index(clean, "]")
		if idx >= 0 {
			pidInfo = clean[:idx+1] // "[LOGS for PID N]"
			body = clean[idx+1:]
			// Reformat PID info to shorter form
			pidInner := strings.TrimPrefix(pidInfo, "[LOGS for PID")
			pidInner = strings.TrimSuffix(pidInner, "]")
			pidInfo = "[PID " + strings.TrimSpace(pidInner) + "]"
			body = strings.TrimLeft(body, "\n")
		} else {
			body = clean
		}
	} else {
		body = clean
	}

	if body == "" {
		if pidInfo != "" {
			return pidInfo + " (empty)"
		}
		return "(empty)"
	}

	totalLines := strings.Count(body, "\n") + 1

	// Apply deduplication first (removes consecutive duplicate lines)
	deduped := DeduplicateLines(body)
	dedupLines := strings.Count(deduped, "\n") + 1
	dupRemoved := totalLines - dedupLines

	// Apply tail-focus for very long outputs
	const maxLines = 80
	compressed := deduped
	if dedupLines > maxLines {
		compressed = TailFocus(deduped, 5, maxLines-5, 3)
	}

	finalLines := strings.Count(compressed, "\n") + 1

	// Build header
	var header string
	if pidInfo != "" {
		header = pidInfo
	}
	if finalLines < totalLines {
		header += fmt.Sprintf(" (showing %d of %d lines", finalLines, totalLines)
		if dupRemoved > 0 {
			header += fmt.Sprintf(", %d duplicates removed", dupRemoved)
		}
		header += ")"
	} else if dupRemoved > 0 {
		header += fmt.Sprintf(" (%d duplicates removed)", dupRemoved)
	}

	if header != "" {
		return header + "\n" + compressed
	}
	return compressed
}
