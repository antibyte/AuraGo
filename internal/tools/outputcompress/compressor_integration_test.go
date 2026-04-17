package outputcompress

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ─── Compress Integration Tests ─────────────────────────────────────────────

func TestCompress_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}
	output := strings.Repeat("line\n", 100)
	result, stats := Compress("execute_shell", "git status", output, cfg)
	if result != output {
		t.Error("disabled compression should return input unchanged")
	}
	if stats.Ratio != 1.0 {
		t.Errorf("expected ratio 1.0, got %f", stats.Ratio)
	}
}

func TestCompress_ShortOutput(t *testing.T) {
	cfg := DefaultConfig()
	output := "short output"
	result, stats := Compress("execute_shell", "echo hi", output, cfg)
	if result != output {
		t.Error("short output should not be compressed")
	}
	if stats.Ratio != 1.0 {
		t.Errorf("expected ratio 1.0, got %f", stats.Ratio)
	}
}

func TestCompress_ErrorOutput(t *testing.T) {
	cfg := DefaultConfig()
	output := strings.Repeat("[EXECUTION ERROR] something went wrong\n", 50)
	result, stats := Compress("execute_shell", "git status", output, cfg)
	if result != output {
		t.Error("error output should be preserved when PreserveErrors is true")
	}
	if stats.FilterUsed != "skipped-error" {
		t.Errorf("expected filter 'skipped-error', got %q", stats.FilterUsed)
	}
}

func TestCompress_ErrorOutputNotPreserved(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 10, PreserveErrors: false}
	output := strings.Repeat("[EXECUTION ERROR] something went wrong\n", 50)
	result, _ := Compress("execute_shell", "git status", output, cfg)
	if result == output {
		t.Error("error output should be compressed when PreserveErrors is false")
	}
}

func TestCompress_EmptyOutput(t *testing.T) {
	cfg := DefaultConfig()
	result, stats := Compress("execute_shell", "echo", "", cfg)
	if result != "" {
		t.Error("empty output should remain empty")
	}
	if stats.Ratio != 1.0 {
		t.Errorf("expected ratio 1.0, got %f", stats.Ratio)
	}
}

func TestCompress_ShellTool(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, ShellCompression: true}
	// Use generic command with large repeated output
	output := strings.Repeat("some log line repeated many times\n", 100)
	result, stats := Compress("execute_shell", "echo test", output, cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter, got %q", stats.FilterUsed)
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression, got %d >= %d", len(result), len(output))
	}
}

func TestCompress_PythonTool(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, PythonCompression: true}
	output := strings.Repeat("print('hello world')\n", 100)
	_, stats := Compress("execute_python", "", output, cfg)
	if stats.FilterUsed != "python" {
		t.Errorf("expected python filter, got %q", stats.FilterUsed)
	}
	if stats.Ratio >= 1.0 {
		t.Errorf("expected compression ratio < 1.0, got %f", stats.Ratio)
	}
}

func TestCompress_APITool(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: true}
	// Multi-line JSON with enough lines to trigger compaction
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	for i := 30; i < 60; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": "value_%d",`, i, i) + "\n")
	}
	sb.WriteString(`  "empty": []` + "\n")
	sb.WriteString(`  "blank": ""` + "\n")
	sb.WriteString("}")
	result, stats := Compress("docker", "", sb.String(), cfg)
	if stats.FilterUsed != "api" {
		t.Errorf("expected api filter, got %q", stats.FilterUsed)
	}
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression for API output")
	}
}

func TestCompress_UnknownTool(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true}
	output := strings.Repeat("some output line\n", 100)
	_, stats := Compress("unknown_tool", "", output, cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter, got %q", stats.FilterUsed)
	}
}
// ── V3: Sub-Toggle Tests ─────────────────────────────────────────────

func TestCompress_ShellSubToggleOff(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, ShellCompression: false}
	output := strings.Repeat("some log line\n", 100)
	_, stats := Compress("execute_shell", "echo test", output, cfg)
	// With ShellCompression off, shell tool falls through to generic
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter with shell_compression=false, got %q", stats.FilterUsed)
	}
}

func TestCompress_PythonSubToggleOff(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, PythonCompression: false}
	output := strings.Repeat("print('hello')\n", 100)
	_, stats := Compress("execute_python", "", output, cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter with python_compression=false, got %q", stats.FilterUsed)
	}
}

func TestCompress_APISubToggleOff(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: false}
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	sb.WriteString(`  "keep": "value"` + "\n")
	sb.WriteString("}\n")
	_, stats := Compress("web_request", "", sb.String(), cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter with api_compression=false, got %q", stats.FilterUsed)
	}
}

func TestDefaultConfig_AllSubTogglesOn(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.ShellCompression {
		t.Error("DefaultConfig should have ShellCompression=true")
	}
	if !cfg.PythonCompression {
		t.Error("DefaultConfig should have PythonCompression=true")
	}
	if !cfg.APICompression {
		t.Error("DefaultConfig should have APICompression=true")
	}
	if !cfg.PreserveErrors {
		t.Error("DefaultConfig should have PreserveErrors=true")
	}
	if cfg.MinChars != 500 {
		t.Errorf("DefaultConfig MinChars should be 500, got %d", cfg.MinChars)
	}
}

func TestCompress_BackwardCompat_ZeroValueConfig(t *testing.T) {
	// Simulates the case where config.go sets defaults via yamlHasPath.
	// After config.go processing, a zero-value config becomes the full default.
	cfg := DefaultConfig()
	output := strings.Repeat("line\n", 200)
	_, stats := Compress("execute_shell", "git status", output, cfg)
	if stats.FilterUsed == "none" {
		t.Error("DefaultConfig should produce active compression")
	}
}

func TestCompress_SubToggleMixed(t *testing.T) {
	// Shell on, Python off, API on
	cfg := Config{
		Enabled:           true,
		MinChars:          100,
		PreserveErrors:    true,
		ShellCompression:  true,
		PythonCompression: false,
		APICompression:    true,
	}
	shellOutput := strings.Repeat("log\n", 200)
	_, shellStats := Compress("execute_shell", "echo", shellOutput, cfg)
	if shellStats.FilterUsed != "generic" && shellStats.FilterUsed == "none" {
		t.Error("shell should compress with ShellCompression=true")
	}

	pythonOutput := strings.Repeat("print('hi')\n", 200)
	_, pyStats := Compress("execute_python", "", pythonOutput, cfg)
	if pyStats.FilterUsed != "generic" {
		t.Errorf("python should fall through to generic with PythonCompression=false, got %q", pyStats.FilterUsed)
	}
}
// ─── Grenzwert- und Performance-Tests ──────────────────────────────────────

func TestCompress_VeryLargeOutput(t *testing.T) {
	// Test mit sehr großem Output (~500KB)
	output := strings.Repeat("line\n", 100000)
	cfg := DefaultConfig()

	result, stats := Compress("execute_shell", "echo test", output, cfg)

	if stats.Ratio > 0.5 {
		t.Errorf("expected better compression for large output, got ratio %f", stats.Ratio)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression, got %d >= %d", len(result), len(output))
	}
}

func TestCompress_UnicodeOutput(t *testing.T) {
	// Test mit Unicode/Emoji
	output := "🎉 Test ✓ 中文 🚀\n" + strings.Repeat("line\n", 100)
	cfg := DefaultConfig()

	result, stats := Compress("execute_shell", "echo test", output, cfg)

	if stats.Ratio >= 1.0 {
		t.Errorf("expected some compression, got ratio %f", stats.Ratio)
	}
	if !strings.Contains(result, "🎉") {
		t.Error("unicode emoji should be preserved")
	}
	if !strings.Contains(result, "✓") {
		t.Error("unicode symbol should be preserved")
	}
	if !strings.Contains(result, "中文") {
		t.Error("chinese characters should be preserved")
	}
}

func TestCompress_SpecialCharacters(t *testing.T) {
	// Test mit speziellen Zeichen
	output := "Tab:\tBackspace:\bNewline:\nCarriage:\rNull:\x00\n" +
		strings.Repeat("line\n", 100)
	cfg := DefaultConfig()

	result, stats := Compress("execute_shell", "echo test", output, cfg)

	if stats.Ratio >= 1.0 {
		t.Errorf("expected some compression, got ratio %f", stats.Ratio)
	}
	// Result should not be empty
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestCompress_Performance(t *testing.T) {
	// Performance-Test: Kompression sollte schnell sein
	output := strings.Repeat("line\n", 10000) // ~50KB
	cfg := DefaultConfig()

	start := time.Now()
	_, _ = Compress("execute_shell", "echo test", output, cfg)
	elapsed := time.Since(start)

	// Kompression sollte unter 100ms bleiben
	if elapsed > 100*time.Millisecond {
		t.Errorf("compression too slow: %v (expected < 100ms)", elapsed)
	}
}

func TestCompress_Performance_Large(t *testing.T) {
	// Performance-Test mit größerem Output
	output := strings.Repeat("line\n", 100000) // ~500KB
	cfg := DefaultConfig()

	start := time.Now()
	_, _ = Compress("execute_shell", "echo test", output, cfg)
	elapsed := time.Since(start)

	// Auch große Kompression sollte unter 500ms bleiben
	if elapsed > 500*time.Millisecond {
		t.Errorf("large compression too slow: %v (expected < 500ms)", elapsed)
	}
}

func TestCompress_EmptyString(t *testing.T) {
	cfg := DefaultConfig()
	result, stats := Compress("execute_shell", "echo", "", cfg)

	if result != "" {
		t.Error("empty input should return empty output")
	}
	if stats.Ratio != 1.0 {
		t.Errorf("expected ratio 1.0 for empty input, got %f", stats.Ratio)
	}
}

func TestCompress_StatsFields(t *testing.T) {
	output := strings.Repeat("line\n", 100)
	cfg := DefaultConfig()

	_, stats := Compress("execute_shell", "echo test", output, cfg)

	// Alle neuen Felder sollten gesetzt sein
	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
	if stats.ProcessingTimeMs < 0 {
		t.Errorf("ProcessingTimeMs should be non-negative, got %d", stats.ProcessingTimeMs)
	}
	if stats.ToolName != "execute_shell" {
		t.Errorf("ToolName should be 'execute_shell', got %q", stats.ToolName)
	}
	// commandSignature extrahiert erste 2 Tokens
	if stats.CommandHint != "echo test" {
		t.Errorf("CommandHint should be 'echo test', got %q", stats.CommandHint)
	}
}
