// Package outputcompress provides command-aware output compression for tool results.
// It reduces token consumption by filtering, deduplicating, and summarising
// verbose shell, test, and API outputs before they enter the LLM context window.
package outputcompress

import (
	"strings"
)

// CompressionStats records how much a single compression pass saved.
type CompressionStats struct {
	ToolName        string  // tool that produced the output
	CommandHint     string  // first token of the command (for shell tools)
	RawChars        int     // character count before compression
	CompressedChars int     // character count after compression
	Ratio           float64 // CompressedChars / RawChars (0.0 = perfect compression, 1.0 = no change)
	FilterUsed      string  // name of the filter that was applied
}

// Config controls compression behaviour.
type Config struct {
	Enabled       bool // master toggle (default: true)
	MinChars      int  // only compress if output exceeds this many characters (default: 500)
	PreserveErrors bool // never compress outputs that contain error markers (default: true)
}

// DefaultConfig returns the recommended MVP configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		MinChars:      500,
		PreserveErrors: true,
	}
}

// Compress is the main entry point. It analyses the tool name and optional
// command string to pick the best filter, then returns the compressed output
// along with statistics.
//
// If cfg.Enabled is false or the output is shorter than cfg.MinChars, the
// input is returned unchanged with Ratio=1.0.
func Compress(toolName, command, output string, cfg Config) (string, CompressionStats) {
	rawLen := len(output)
	stats := CompressionStats{
		ToolName:    toolName,
		CommandHint: commandSignature(command),
		RawChars:    rawLen,
		FilterUsed:  "none",
	}

	if !cfg.Enabled || rawLen == 0 {
		stats.CompressedChars = rawLen
		stats.Ratio = 1.0
		return output, stats
	}

	// Skip compression for short outputs
	if rawLen < cfg.MinChars {
		stats.CompressedChars = rawLen
		stats.Ratio = 1.0
		return output, stats
	}

	// Preserve error outputs when configured
	if cfg.PreserveErrors && isErrorOutput(output) {
		stats.CompressedChars = rawLen
		stats.Ratio = 1.0
		stats.FilterUsed = "skipped-error"
		return output, stats
	}

	// Pick filter based on tool name and command
	result := output
	filter := "generic"

	switch {
	case isShellTool(toolName):
		result, filter = compressShellOutput(command, output)
	case isPythonTool(toolName):
		result, filter = compressPythonOutput(output)
	case isAPITool(toolName):
		result, filter = compressAPIOutput(output)
	default:
		result = compressGeneric(output)
		filter = "generic"
	}

	stats.CompressedChars = len(result)
	stats.FilterUsed = filter
	if rawLen > 0 {
		stats.Ratio = float64(stats.CompressedChars) / float64(rawLen)
	} else {
		stats.Ratio = 1.0
	}
	return result, stats
}

// commandSignature extracts the first two tokens from a command string
// (e.g. "git status", "docker ps", "go test").
func commandSignature(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return parts[0]
}

// isShellTool returns true for tools that execute shell commands.
func isShellTool(name string) bool {
	switch name {
	case "execute_shell", "execute_sudo", "execute_remote_shell",
		"remote_execution", "ssh_exec", "service_manager":
		return true
	}
	return false
}

// isPythonTool returns true for Python execution tools.
func isPythonTool(name string) bool {
	switch name {
	case "execute_python", "execute_sandbox":
		return true
	}
	return false
}

// isAPITool returns true for tools that return structured API responses.
func isAPITool(name string) bool {
	switch name {
	case "docker", "docker_compose", "proxmox", "homeassistant",
		"kubernetes", "api_request":
		return true
	}
	return false
}

// isErrorOutput detects common error markers in tool output.
func isErrorOutput(output string) bool {
	// Check for error markers that should never be compressed away
	errorMarkers := []string{
		"[EXECUTION ERROR]",
		"[PERMISSION DENIED]",
		"[TOOL BLOCKED]",
		"fatal:",
		"panic:",
		"SIGSEGV",
	}
	for _, marker := range errorMarkers {
		if strings.Contains(output, marker) {
			return true
		}
	}
	return false
}
