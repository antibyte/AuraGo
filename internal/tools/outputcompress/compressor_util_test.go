package outputcompress

import (
	"fmt"
	"strings"
	"testing"
)

// ─── Deduplication Tests ────────────────────────────────────────────────────

func TestDeduplicateLines_Empty(t *testing.T) {
	result := DeduplicateLines("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestDeduplicateLines_NoRepeats(t *testing.T) {
	input := "line1\nline2\nline3"
	result := DeduplicateLines(input)
	if result != input {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestDeduplicateLines_AllSame(t *testing.T) {
	input := strings.Repeat("same line\n", 100)
	result := DeduplicateLines(input)
	if !strings.Contains(result, "identical lines omitted") {
		t.Errorf("expected dedup marker, got %q", result)
	}
	if strings.Count(result, "same line") > 3 {
		t.Error("should only keep a few copies of the repeated line")
	}
}

func TestDeduplicateLines_SmallRepeat(t *testing.T) {
	input := "line\nline\nline"
	result := DeduplicateLines(input)
	// Nach der Optimierung werden alle Wiederholungen ab 2 mit Marker komprimiert
	if !strings.Contains(result, "identical lines omitted") {
		t.Errorf("repeats should be collapsed with marker, got %q", result)
	}
	if !strings.Contains(result, "[3 identical lines omitted]") {
		t.Errorf("should show count of 3, got %q", result)
	}
}

func TestDeduplicateLines_Mixed(t *testing.T) {
	input := "header\n" + strings.Repeat("repeated\n", 10) + "footer"
	result := DeduplicateLines(input)
	if !strings.Contains(result, "header") {
		t.Error("should preserve header")
	}
	if !strings.Contains(result, "footer") {
		t.Error("should preserve footer")
	}
	if !strings.Contains(result, "identical lines omitted") {
		t.Error("should collapse repeated middle section")
	}
}
// ─── Whitespace Collapse Tests ──────────────────────────────────────────────

func TestCollapseWhitespace_Empty(t *testing.T) {
	result := CollapseWhitespace("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestCollapseWhitespace_MultipleBlanks(t *testing.T) {
	input := "line1\n\n\n\n\nline2"
	result := CollapseWhitespace(input)
	if strings.Contains(result, "\n\n\n") {
		t.Errorf("should not have consecutive blank lines, got %q", result)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") {
		t.Error("should preserve content lines")
	}
}

func TestCollapseWhitespace_TrailingSpaces(t *testing.T) {
	input := "line1   \nline2\t\nline3  "
	result := CollapseWhitespace(input)
	for _, line := range strings.Split(result, "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("trailing whitespace not removed: %q", line)
		}
	}
}
// ─── TailFocus Tests ────────────────────────────────────────────────────────

func TestTailFocus_ShortInput(t *testing.T) {
	input := "line1\nline2\nline3"
	result := TailFocus(input, 1, 1, 2)
	if result != input {
		t.Error("short input should be returned unchanged")
	}
}

func TestTailFocus_LongInput(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	input := strings.Join(lines, "\n")

	result := TailFocus(input, 10, 10, 5)
	if !strings.Contains(result, "lines omitted") {
		t.Error("should contain omission marker")
	}
	if strings.Count(result, "\n") >= 200 {
		t.Error("should have fewer lines than input")
	}
}
// ─── ANSI Stripping Tests ───────────────────────────────────────────────────

func TestStripANSI_NoEscape(t *testing.T) {
	input := "normal text"
	result := StripANSI(input)
	if result != input {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestStripANSI_ColorCodes(t *testing.T) {
	input := "\x1b[32mgreen text\x1b[0m normal"
	result := StripANSI(input)
	if strings.Contains(result, "\x1b[") {
		t.Errorf("ANSI codes should be removed, got %q", result)
	}
	if result != "green text normal" {
		t.Errorf("expected 'green text normal', got %q", result)
	}
}
// ─── Command Signature Tests ────────────────────────────────────────────────

func TestCommandSignature(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"git status", "git status"},
		{"git log --oneline", "git log"},
		{"docker ps -a", "docker ps"},
		{"go test ./...", "go test"},
		{"", ""},
		{"echo", "echo"},
		{"  git   status  ", "git status"},
	}

	for _, tt := range tests {
		got := commandSignature(tt.cmd)
		if got != tt.want {
			t.Errorf("commandSignature(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}
// ─── Tool Classification Tests ──────────────────────────────────────────────

func TestIsShellTool(t *testing.T) {
	tests := []struct {
		tool string
		want bool
	}{
		{"execute_shell", true},
		{"execute_sudo", true},
		{"execute_remote_shell", true},
		{"remote_execution", true},
		{"ssh_exec", true},
		{"execute_python", false},
		{"docker", false},
		{"filesystem", false},
	}
	for _, tt := range tests {
		got := isShellTool(tt.tool)
		if got != tt.want {
			t.Errorf("isShellTool(%q) = %v, want %v", tt.tool, got, tt.want)
		}
	}
}

func TestIsPythonTool(t *testing.T) {
	if !isPythonTool("execute_python") {
		t.Error("execute_python should be a python tool")
	}
	if !isPythonTool("execute_sandbox") {
		t.Error("execute_sandbox should be a python tool")
	}
	if isPythonTool("execute_shell") {
		t.Error("execute_shell should not be a python tool")
	}
}

func TestIsAPITool(t *testing.T) {
	apiTools := []string{"docker", "docker_compose", "proxmox", "homeassistant", "home_assistant",
		"kubernetes", "api_request", "github", "sql_query",
		"filesystem", "filesystem_op", "file_reader_advanced", "smart_file_read",
		"list_processes", "read_process_logs", "manage_daemon", "manage_plan"}
	for _, tool := range apiTools {
		if !isAPITool(tool) {
			t.Errorf("%q should be an API tool", tool)
		}
	}
	nonAPITools := []string{"execute_shell", "execute_python", "execute_sudo"}
	for _, tool := range nonAPITools {
		if isAPITool(tool) {
			t.Errorf("%q should not be an API tool", tool)
		}
	}
}
// ─── Error Detection Tests ─────────────────────────────────────────────────

func TestIsErrorOutput(t *testing.T) {
	errorOutputs := []string{
		"Tool Output: [EXECUTION ERROR] command failed",
		"[PERMISSION DENIED] not allowed",
		"[TOOL BLOCKED] security check failed",
		"fatal: not a git repository",
		"panic: runtime error: index out of range",
	}
	for _, output := range errorOutputs {
		if !isErrorOutput(output) {
			t.Errorf("should detect error in: %q", output)
		}
	}

	normalOutputs := []string{
		"Tool Output:\nSTDOUT:\nhello world",
		"Everything is fine",
		"success: operation completed",
	}
	for _, output := range normalOutputs {
		if isErrorOutput(output) {
			t.Errorf("should not detect error in: %q", output)
		}
	}
}
// ─── Container ID Detection Tests ───────────────────────────────────────────

func TestIsContainerID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123def456", true},
		{"a1b2c3d4e5f6", true},
		{"ABC123def456", false},  // uppercase
		{"short", false},         // too short
		{"g123456789012", false}, // 'g' is not hex
		{"123456789012", true},
	}
	for _, tt := range tests {
		got := isContainerID(tt.input)
		if got != tt.want {
			t.Errorf("isContainerID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
// ─── CompressionStats Tests ─────────────────────────────────────────────────

func TestCompressionStats_Ratio(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 10, PreserveErrors: false}

	// Generate output that will be compressed
	output := strings.Repeat("same line repeated many times\n", 200)
	result, stats := Compress("execute_shell", "echo test", output, cfg)

	if stats.RawChars != len(output) {
		t.Errorf("RawChars = %d, want %d", stats.RawChars, len(output))
	}
	if stats.CompressedChars != len(result) {
		t.Errorf("CompressedChars = %d, want %d", stats.CompressedChars, len(result))
	}
	if stats.Ratio <= 0 || stats.Ratio > 1.0 {
		t.Errorf("Ratio = %f, want (0, 1.0]", stats.Ratio)
	}
	if stats.ToolName != "execute_shell" {
		t.Errorf("ToolName = %q, want %q", stats.ToolName, "execute_shell")
	}
}
// ─── JSON Compaction Tests ──────────────────────────────────────────────────

func TestCompactJSON(t *testing.T) {
	// Need 20+ lines to trigger compaction
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString(`  "name": "test",` + "\n")
	sb.WriteString(`  "value": null,` + "\n")
	sb.WriteString(`  "items": [],` + "\n")
	sb.WriteString(`  "description": "",` + "\n")
	sb.WriteString(`  "active": true,` + "\n")
	sb.WriteString(`  "count": 42,` + "\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf(`  "extra_%d": null,`, i) + "\n")
	}
	sb.WriteString(`  "blank": ""` + "\n")
	sb.WriteString("}")
	result := compactJSON(sb.String())
	if strings.Contains(result, ": null") {
		t.Error("should remove null fields")
	}
	if strings.Contains(result, ": []") {
		t.Error("should remove empty array fields")
	}
	if !strings.Contains(result, `"active": true`) {
		t.Error("should keep non-empty fields")
	}
	// New implementation removes fields without marker - just verify fields are gone
	if strings.Contains(result, `"description":`) {
		t.Error("should remove empty string fields")
	}
}
// ─── Timestamp Stripping Tests ──────────────────────────────────────────────

func TestStripTimestamps_ISO(t *testing.T) {
	input := "2026-04-13T12:00:00.000Z [INFO] server started\n2026-04-13T12:00:01Z [DEBUG] processing"
	result := stripTimestamps(input)
	if strings.Contains(result, "2026-04-13") {
		t.Errorf("ISO timestamps should be stripped, got %q", result)
	}
	if !strings.Contains(result, "[INFO]") {
		t.Error("should preserve log level")
	}
}

func TestStripTimestamps_Bracketed(t *testing.T) {
	input := "[2026-04-13 12:00:00] message here"
	result := stripTimestamps(input)
	if strings.Contains(result, "2026") {
		t.Errorf("bracketed timestamps should be stripped, got %q", result)
	}
}
// ─── Analytics Tests ─────────────────────────────────────────────────────────

func TestAnalytics_RecordAndSnapshot(t *testing.T) {
	ResetCompressionStats()

	// Record some compression events
	RecordCompressionStats(CompressionStats{
		ToolName:        "execute_shell",
		CommandHint:     "git status",
		RawChars:        10000,
		CompressedChars: 2000,
		Ratio:           0.2,
		FilterUsed:      "git-status",
	})
	RecordCompressionStats(CompressionStats{
		ToolName:        "execute_shell",
		CommandHint:     "docker ps",
		RawChars:        5000,
		CompressedChars: 1000,
		Ratio:           0.2,
		FilterUsed:      "docker-ps",
	})
	RecordCompressionStats(CompressionStats{
		ToolName:        "execute_python",
		CommandHint:     "",
		RawChars:        8000,
		CompressedChars: 4000,
		Ratio:           0.5,
		FilterUsed:      "python",
	})

	// Record a skip
	RecordCompressionSkipped()
	RecordCompressionSkipped()

	snap := GetCompressionSnapshot()
	if !snap.Enabled {
		t.Error("snapshot should report enabled")
	}
	if snap.CompressionsApplied != 3 {
		t.Errorf("expected 3 compressions, got %d", snap.CompressionsApplied)
	}
	if snap.CompressionsSkipped != 2 {
		t.Errorf("expected 2 skips, got %d", snap.CompressionsSkipped)
	}
	if snap.TotalRawChars != 23000 {
		t.Errorf("expected 23000 raw chars, got %d", snap.TotalRawChars)
	}
	if snap.TotalSavedChars != 16000 {
		t.Errorf("expected 16000 saved chars, got %d", snap.TotalSavedChars)
	}
	if snap.TotalCompressedChars != 7000 {
		t.Errorf("expected 7000 compressed chars, got %d", snap.TotalCompressedChars)
	}
	expectedRatio := float64(16000) / float64(23000)
	if snap.AverageSavingsRatio < expectedRatio-0.01 || snap.AverageSavingsRatio > expectedRatio+0.01 {
		t.Errorf("expected ratio ~%.3f, got %.3f", expectedRatio, snap.AverageSavingsRatio)
	}

	// Check top tools
	if len(snap.TopTools) < 2 {
		t.Errorf("expected at least 2 top tools, got %d", len(snap.TopTools))
	}
	if snap.TopTools[0].Tool != "execute_shell" {
		t.Errorf("expected execute_shell as top tool, got %q", snap.TopTools[0].Tool)
	}

	// Check top filters
	if len(snap.TopFilters) < 2 {
		t.Errorf("expected at least 2 top filters, got %d", len(snap.TopFilters))
	}
}

func TestAnalytics_IgnoresZeroSavings(t *testing.T) {
	ResetCompressionStats()

	RecordCompressionStats(CompressionStats{
		ToolName:        "test",
		RawChars:        1000,
		CompressedChars: 1000,
		Ratio:           1.0,
		FilterUsed:      "generic",
	})

	snap := GetCompressionSnapshot()
	if snap.CompressionsApplied != 0 {
		t.Errorf("zero-savings should not be recorded, got %d", snap.CompressionsApplied)
	}
}

func TestAnalytics_Reset(t *testing.T) {
	ResetCompressionStats()

	RecordCompressionStats(CompressionStats{
		ToolName:        "test",
		RawChars:        5000,
		CompressedChars: 2000,
		Ratio:           0.4,
		FilterUsed:      "generic",
	})

	snap := GetCompressionSnapshot()
	if snap.CompressionsApplied != 1 {
		t.Error("should have 1 compression before reset")
	}

	ResetCompressionStats()
	snap = GetCompressionSnapshot()
	if snap.CompressionsApplied != 0 {
		t.Error("should have 0 compressions after reset")
	}
	if snap.TotalRawChars != 0 {
		t.Error("should have 0 raw chars after reset")
	}
}

func TestAnalytics_TopToolsOrdering(t *testing.T) {
	ResetCompressionStats()

	// Tool A: 10000 raw → 5000 compressed = 5000 saved
	RecordCompressionStats(CompressionStats{
		ToolName: "tool_a", RawChars: 10000, CompressedChars: 5000, Ratio: 0.5, FilterUsed: "generic",
	})
	// Tool B: 20000 raw → 5000 compressed = 15000 saved
	RecordCompressionStats(CompressionStats{
		ToolName: "tool_b", RawChars: 20000, CompressedChars: 5000, Ratio: 0.25, FilterUsed: "generic",
	})

	snap := GetCompressionSnapshot()
	if len(snap.TopTools) < 2 {
		t.Fatalf("expected 2 top tools, got %d", len(snap.TopTools))
	}
	if snap.TopTools[0].Tool != "tool_b" {
		t.Errorf("tool_b should be first (15000 saved), got %q with %d saved",
			snap.TopTools[0].Tool, snap.TopTools[0].SavedChars)
	}
}

func TestAnalytics_RecentCompressions(t *testing.T) {
	ResetCompressionStats()

	for i := 0; i < 25; i++ {
		RecordCompressionStats(CompressionStats{
			ToolName:        fmt.Sprintf("tool_%d", i),
			RawChars:        1000 + i,
			CompressedChars: 500,
			Ratio:           0.5,
			FilterUsed:      "generic",
		})
	}

	snap := GetCompressionSnapshot()
	// Recent should be limited to 20
	if len(snap.RecentCompressions) > 20 {
		t.Errorf("expected at most 20 recent, got %d", len(snap.RecentCompressions))
	}
	// Most recent should be tool_24
	last := snap.RecentCompressions[len(snap.RecentCompressions)-1]
	if last.ToolName != "tool_24" {
		t.Errorf("expected last recent to be tool_24, got %q", last.ToolName)
	}
}
func TestIsLogContent(t *testing.T) {
	// Log content
	logOutput := "2024-01-15 10:30:00 INFO  Starting server\n2024-01-15 10:30:01 DEBUG Connected\n2024-01-15 10:30:02 ERROR Timeout\n"
	if !isLogContent(logOutput) {
		t.Error("expected log content detected")
	}

	// Non-log content
	plainOutput := "Hello World\nThis is a config file\nWith some settings\n"
	if isLogContent(plainOutput) {
		t.Error("expected non-log content")
	}

	// JSON log format
	jsonLog := `{"level":"info","msg":"started","ts":"2024-01-15T10:30:00Z"}
{"level":"error","msg":"failed","ts":"2024-01-15T10:30:01Z"}
{"level":"debug","msg":"retry","ts":"2024-01-15T10:30:02Z"}`
	if !isLogContent(jsonLog) {
		t.Error("expected JSON log content detected")
	}
}
func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{1073741824, "1.0GB"},
	}
	for _, tt := range tests {
		got := formatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
func TestDeduplicateLines_AllDuplicates(t *testing.T) {
	// Test wenn alle Zeilen identisch sind
	input := strings.Repeat("same line\n", 100)
	result := DeduplicateLines(input)

	// Sollte nur einmal erscheinen mit Marker
	if strings.Count(result, "same line") > 1 {
		t.Errorf("expected single occurrence, got %d", strings.Count(result, "same line"))
	}
	if !strings.Contains(result, "identical lines omitted") {
		t.Error("expected dedup marker")
	}
}
func TestTailFocusConstants(t *testing.T) {
	// Test dass die Konstanten konsistent sind
	if tailFocusHeadDefault+tailFocusTailDefault+tailFocusMinGap > minLinesForTailFocus {
		t.Error("minLinesForTailFocus should be >= sum of default head+tail+gap")
	}
	if tailFocusLogsHead+tailFocusLogsTail+tailFocusLogsMinGap > minLinesForTailFocus {
		t.Error("logs threshold should be reasonable")
	}
	if tailFocusCodeHead+tailFocusCodeTail+tailFocusCodeMinGap > minLinesForTailFocus {
		t.Error("code threshold should be reasonable")
	}
}
func TestStripANSI_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SGR codes",
			input:    "\x1b[31mred\x1b[0m \x1b[1mbold\x1b[22m",
			expected: "red bold",
		},
		{
			name:     "256 color",
			input:    "\x1b[38;5;196mred\x1b[0m",
			expected: "red",
		},
		{
			name:     "True color",
			input:    "\x1b[38;2;255;0;0mRGB red\x1b[0m",
			expected: "RGB red",
		},
		{
			name:     "Cursor movement",
			input:    "\x1b[2J\x1b[HClear screen",
			expected: "Clear screen",
		},
		{
			name:     "Window title",
			input:    "\x1b]0;Terminal Title\x07Content",
			expected: "Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripANSI(tt.input)
			if result != tt.expected {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
func TestIsErrorOutput_Extended(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantError bool
	}{
		{"fatal error", "fatal: unable to access", true},
		{"panic", "panic: runtime error", true},
		{"error prefix", "error: file not found", true},
		{"ERROR uppercase", "ERROR: permission denied", true},
		{"failed to", "failed to connect", true},
		{"exception", "exception: ValueError", true},
		{"traceback", "Traceback (most recent call last)", true},
		{"permission denied", "permission denied", true},
		{"normal output", "success: operation completed", false},
		{"warning", "warning: deprecated", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isErrorOutput(tt.output)
			if result != tt.wantError {
				t.Errorf("isErrorOutput(%q) = %v, want %v", tt.output, result, tt.wantError)
			}
		})
	}
}
func TestCompactJSON_ProperParsing(t *testing.T) {
	input := `{
		"name": "test",
		"value": null,
		"empty": "",
		"null_array": [],
		"null_object": {},
		"valid": "data"
	}`

	result := compactJSON(input)

	// Should remove null, empty string, empty array, empty object
	if strings.Contains(result, ": null") {
		t.Error("null fields should be removed")
	}
	if strings.Contains(result, ": \"\"") {
		t.Error("empty string fields should be removed")
	}
	if strings.Contains(result, ": []") {
		t.Error("empty array fields should be removed")
	}
	if strings.Contains(result, ": {}") {
		t.Error("empty object fields should be removed")
	}
	// Should keep valid data
	if !strings.Contains(result, "valid") {
		t.Error("valid fields should be kept")
	}
}
func TestCompactJSON_InvalidFallback(t *testing.T) {
	// Invalid JSON should use fallback
	input := `{ invalid json }`
	result := compactJSON(input)

	// Should return input unchanged (fallback returns input for invalid JSON)
	if !strings.Contains(result, "invalid json") {
		t.Error("invalid JSON should be handled gracefully")
	}
}
func TestRemoveEmptyValues_Nested(t *testing.T) {
	input := map[string]interface{}{
		"keep":         "value",
		"null":         nil,
		"empty_string": "",
		"empty_array":  []interface{}{},
		"empty_object": map[string]interface{}{},
		"nested": map[string]interface{}{
			"keep_nested": "value",
			"null_nested": nil,
		},
		"array_with_empty": []interface{}{
			"keep",
			nil,
			"",
			[]interface{}{},
		},
	}

	result := removeEmptyValues(input)

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result should be map")
	}

	if _, exists := resultMap["keep"]; !exists {
		t.Error("should keep valid value")
	}
	if _, exists := resultMap["null"]; exists {
		t.Error("should remove null")
	}
	if _, exists := resultMap["empty_string"]; exists {
		t.Error("should remove empty string")
	}
	if _, exists := resultMap["nested"]; !exists {
		t.Error("should keep nested object")
	}
}
