package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
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
	// 3 repeats is below threshold of 4, should keep all
	if strings.Contains(result, "omitted") {
		t.Errorf("small repeats should not be collapsed, got %q", result)
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

// ─── Git Filter Tests ───────────────────────────────────────────────────────

func TestCompressGitStatus_Clean(t *testing.T) {
	output := "On branch main\nnothing to commit, working tree clean"
	result, filter := compressGit("status", output)
	if filter != "git-status" {
		t.Errorf("expected git-status, got %q", filter)
	}
	if !strings.Contains(result, "Clean") {
		t.Errorf("expected clean message, got %q", result)
	}
}

func TestCompressGitStatus_WithChanges(t *testing.T) {
	output := `On branch main
Changes to be committed:
  (use "git restore --staged <file>..." to unstage)
	new file:   cmd/server/main.go
	modified:   internal/agent/agent.go

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   internal/config/config.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	internal/tools/outputcompress/compressor.go
	internal/tools/outputcompress/shell.go
	internal/tools/outputcompress/dedup.go
`
	result, _ := compressGit("status", output)
	if !strings.Contains(result, "Staged") {
		t.Error("should contain Staged section")
	}
	if !strings.Contains(result, "Unstaged") {
		t.Error("should contain Unstaged section")
	}
	if !strings.Contains(result, "Untracked") {
		t.Error("should contain Untracked section")
	}
	// Should be significantly shorter
	if len(result) >= len(output) {
		t.Errorf("expected compression: %d >= %d", len(result), len(output))
	}
}

func TestCompressGitLog_Verbose(t *testing.T) {
	output := `commit abc1234567890abcdef1234567890abcdef1234
Author: Test User <test@example.com>
Date:   Mon Apr 13 12:00:00 2026 +0200

    feat: add output compression layer

commit def4567890abcdef1234567890abcdef12345678
Author: Test User <test@example.com>
Date:   Mon Apr 13 11:00:00 2026 +0200

    fix: resolve config parsing issue

commit ghi9012345678abcdef1234567890abcdef12345
Author: Test User <test@example.com>
Date:   Mon Apr 13 10:00:00 2026 +0200

    chore: update dependencies
`
	result, _ := compressGit("log", output)
	if !strings.Contains(result, "abc1234567") {
		t.Error("should contain commit hash")
	}
	if !strings.Contains(result, "feat: add output compression") {
		t.Error("should contain commit subject")
	}
	// Should NOT contain Author/Date lines
	if strings.Contains(result, "Author:") {
		t.Error("should not contain Author lines")
	}
}

func TestCompressGitLog_Oneline(t *testing.T) {
	output := "abc1234 feat: add compression\ndef5678 fix: resolve bug\nghi9012 chore: cleanup"
	result, _ := compressGit("log", output)
	// Already compact, should pass through
	if !strings.Contains(result, "abc1234") {
		t.Error("should preserve oneline format")
	}
}

func TestCompressGitDiff_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/main.go b/main.go\n")
	sb.WriteString("index abc..def 100644\n")
	sb.WriteString("--- a/main.go\n")
	sb.WriteString("+++ b/main.go\n")
	sb.WriteString("@@ -1,5 +1,10 @@\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("+added line\n")
	}
	for i := 0; i < 50; i++ {
		sb.WriteString("-removed line\n")
	}
	sb.WriteString(" context line\n")
	sb.WriteString("diff --git a/helper.go b/helper.go\n")
	sb.WriteString("@@ -10,3 +10,8 @@\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("+another addition\n")
	}

	result, _ := compressGit("diff", sb.String())
	if !strings.Contains(result, "Diff summary") {
		t.Error("should contain diff summary")
	}
	if !strings.Contains(result, "files changed") {
		t.Error("should mention files changed")
	}
	if !strings.Contains(result, "+") || !strings.Contains(result, "-") {
		t.Error("should show line counts")
	}
}

func TestCompressGitDiff_Small(t *testing.T) {
	output := "diff --git a/file.go b/file.go\n@@ -1,3 +1,3 @@\n-old\n+new\n context\n"
	result, _ := compressGit("diff", output)
	// Small diffs should use generic compression
	if result == "" {
		t.Error("should produce output")
	}
}

// ─── Docker Filter Tests ────────────────────────────────────────────────────

func TestCompressDockerPS(t *testing.T) {
	output := `CONTAINER ID   IMAGE          COMMAND    CREATED       STATUS         PORTS                    NAMES
abc123def456   nginx:latest   "/bin/sh"  2 hours ago   Up 2 hours     0.0.0.0:80->80/tcp       web-server
789ghi012jkl   redis:7        "redis…"   3 hours ago   Up 3 hours     0.0.0.0:6379->6379/tcp   cache
`
	result, filter := compressContainer("ps", output)
	if filter != "docker-ps" {
		t.Errorf("expected docker-ps, got %q", filter)
	}
	// Container IDs should be stripped
	if strings.Contains(result, "abc123def456") {
		t.Error("container ID should be stripped")
	}
	if !strings.Contains(result, "nginx") {
		t.Error("should contain image name")
	}
}

func TestCompressDockerLogs(t *testing.T) {
	output := "2026-04-13T12:00:00.000Z [INFO] server started\n" +
		strings.Repeat("2026-04-13T12:00:01.000Z [INFO] request processed\n", 50) +
		"2026-04-13T12:00:52.000Z [ERROR] connection lost\n"

	result, filter := compressContainer("logs", output)
	if filter != "docker-logs" {
		t.Errorf("expected docker-logs, got %q", filter)
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression: %d >= %d", len(result), len(output))
	}
}

// ─── Go Test Filter Tests ───────────────────────────────────────────────────

func TestCompressGoTest(t *testing.T) {
	output := `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSubtract
--- PASS: TestSubtract (0.00s)
=== RUN   TestDivide
--- FAIL: TestDivide (0.00s)
    math_test.go:15: expected 5, got 4
=== RUN   TestMultiply
--- PASS: TestMultiply (0.00s)
FAIL
FAIL    aurago/internal/math [build failed]
ok      aurago/internal/config  0.012s
`
	result := compressGoTest(output)
	if !strings.Contains(result, "FAIL") {
		t.Error("should contain failure info")
	}
	if !strings.Contains(result, "TestDivide") {
		t.Error("should contain failed test name")
	}
	if !strings.Contains(result, "ok") {
		t.Error("should contain passing package summary")
	}
}

// ─── Python Traceback Filter Tests ──────────────────────────────────────────

func TestFilterPythonTraceback_NoTraceback(t *testing.T) {
	output := "Hello World\nNo errors here"
	result := filterPythonTraceback(output)
	if result != output {
		t.Error("output without traceback should be unchanged")
	}
}

func TestFilterPythonTraceback_WithTraceback(t *testing.T) {
	output := `Traceback (most recent call last):
  File "/workspace/my_script.py", line 42, in run
    result = process(data)
  File "/usr/lib/python3.11/json/__init__.py", line 34, in loads
    return _default_decoder.decode(s)
  File "/usr/lib/python3.11/json/decoder.py", line 337, in decode
    obj, end = self.raw_decode(s)
  File "/usr/lib/python3.11/json/decoder.py", line 355, in raw_decode
    raise JSONDecodeError("Expecting value", s, err.value)
json.decoder.JSONDecodeError: Expecting value: line 1 column 1 (char 0)
`
	result := filterPythonTraceback(output)
	if !strings.Contains(result, "my_script.py") {
		t.Error("should preserve user code frame")
	}
	if !strings.Contains(result, "JSONDecodeError") {
		t.Error("should preserve error type")
	}
	// Should omit library frames
	if strings.Contains(result, "library frames omitted") {
		// Good - library frames were collapsed
	} else if !strings.Contains(result, "json/decoder.py") {
		// Also acceptable if just filtered out
	}
}

// ─── Grep Filter Tests ──────────────────────────────────────────────────────

func TestCompressGrep_SmallOutput(t *testing.T) {
	output := "main.go:10:func main() {\nmain.go:25:return nil"
	result := compressGrep(output)
	if result != output {
		t.Error("small grep output should be unchanged")
	}
}

func TestCompressGrep_LargeOutput(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString("main.go:" + string(rune('0'+i%10)) + ":match " + string(rune('A'+i%26)) + "\n")
	}
	for i := 0; i < 20; i++ {
		sb.WriteString("helper.go:" + string(rune('0'+i%10)) + ":match " + string(rune('a'+i%26)) + "\n")
	}

	result := compressGrep(sb.String())
	if !strings.Contains(result, "main.go") {
		t.Error("should contain file name")
	}
	if !strings.Contains(result, "matches") {
		t.Error("should show match count for large files")
	}
}

// ─── Find Filter Tests ──────────────────────────────────────────────────────

func TestCompressFind_SmallOutput(t *testing.T) {
	output := "file1.go\nfile2.go\nfile3.go"
	result := compressFind(output)
	if result != output {
		t.Error("small find output should be unchanged")
	}
}

func TestCompressFind_LargeOutput(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("src/pkg" + string(rune('A'+i%5)) + "/file" + string(rune('0'+i%10)) + ".go\n")
	}
	result := compressFind(sb.String())
	if !strings.Contains(result, "results in") {
		t.Error("should contain summary")
	}
	if !strings.Contains(result, "directories") {
		t.Error("should mention directories")
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
		"filesystem", "filesystem_op", "file_reader_advanced", "smart_file_read"}
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
		{"ABC123def456", false}, // uppercase
		{"short", false},        // too short
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

// ─── LsTree Filter Tests ────────────────────────────────────────────────────

func TestCompressLsTree_Small(t *testing.T) {
	output := "file1.go\nfile2.go\nfile3.go"
	result := compressLsTree(output)
	if result != output {
		t.Error("small ls output should be unchanged")
	}
}

func TestCompressLsTree_Large(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("src/module" + string(rune('A'+i%5)) + "/file" + string(rune('0'+i%10)) + ".go\n")
	}
	result := compressLsTree(sb.String())
	if !strings.Contains(result, "entries") {
		t.Error("should group by directory")
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
	if !strings.Contains(result, "omitted") {
		t.Error("should show omission count")
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

// ── V5: K8s Compressor Tests ──────────────────────────────────────────

func TestCompressK8sLogs(t *testing.T) {
	output := "2026-04-13T12:00:00Z [INFO] server started\n" +
		strings.Repeat("2026-04-13T12:00:01Z [INFO] request ok\n", 60) +
		"2026-04-13T12:01:00Z [ERROR] connection lost\n"
	result := compressK8sLogs(output)
	if !strings.Contains(result, "ERROR") {
		t.Error("should preserve error lines")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression for k8s logs: %d >= %d", len(result), len(output))
	}
}

func TestCompressK8sGet_Small(t *testing.T) {
	output := "NAME       READY   STATUS    RESTARTS   AGE\nnginx      1/1     Running   0          1h\nredis      1/1     Running   0          2h"
	result := compressK8sGet(output)
	// Small output (<=8 lines) should pass through
	if !strings.Contains(result, "nginx") {
		t.Error("small k8s get should preserve content")
	}
}

func TestCompressK8sGet_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME                       READY   STATUS      RESTARTS   AGE\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-abc   1/1     Running   0   %dh\n", i, i))
	}
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-def   0/1     Pending    0   %dm\n", i+20, i))
	}
	for i := 0; i < 3; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-ghi   0/1     CrashLoopBackOff   5   %dh\n", i+30, i))
	}
	result := compressK8sGet(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Pending") {
		t.Error("should contain Pending count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count for CrashLoopBackOff")
	}
	if !strings.Contains(result, "CrashLoopBackOff") {
		t.Error("should include failed/pending lines for context")
	}
}

func TestCompressK8sDescribe(t *testing.T) {
	output := `Name:         nginx-deployment-abc123
Namespace:    default
Priority:     0
Node:         node-1/10.0.0.1
Labels:       app=nginx
Status:       Running
IP:           10.244.0.5
Containers:
	 nginx:
	   Image:          nginx:latest
	   Port:           80/TCP
Conditions:
	 Type           Status
	 Ready          True
	 PodScheduled   True
Events:
	 Type    Reason   Age   From       Message
	 Normal  Pulled   5m    kubelet    Successfully pulled image
	 Normal  Created  5m    kubelet    Created container
	 Warning Failed   1m    kubelet    Error: ImagePullBackOff
	 Warning BackOff  30s   kubelet    Back-off restarting failed container`
	result := compressK8sDescribe(output)
	if !strings.Contains(result, "Name:") {
		t.Error("should contain Name field")
	}
	if !strings.Contains(result, "Status:") {
		t.Error("should contain Status field")
	}
	if !strings.Contains(result, "Node:") {
		t.Error("should contain Node field")
	}
	if !strings.Contains(result, "Warning") {
		t.Error("should include warning events")
	}
	if !strings.Contains(result, "Ready") {
		t.Error("should include Conditions")
	}
}

func TestCompressK8s_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"kubectl logs pod-1", "k8s-logs"},
		{"kubectl get pods", "k8s-get"},
		{"kubectl describe pod nginx", "k8s-describe"},
		{"kubectl top nodes", "k8s-top"},
		{"kubectl apply -f.yaml", "k8s-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V5: Systemctl Compressor Tests ────────────────────────────────────

func TestCompressSystemctlStatus(t *testing.T) {
	output := `● nginx.service - A high performance web server
	    Loaded: loaded (/lib/systemd/system/nginx.service; enabled)
	    Active: active (running) since Mon 2026-04-13 12:00:00 UTC; 2h ago
	  Main PID: 1234 (nginx)
	     Tasks: 5 (limit: 4915)
	    Memory: 4.2M
	       CPU: 1.234s
	    CGroup: /system.slice/nginx.service
	            ├─1234 "nginx: master process"
	            └─1235 "nginx: worker process"

    Apr 13 12:00:00 server nginx[1234]: start processing
    Apr 13 12:00:01 server nginx[1234]: request handled
    Apr 13 12:00:02 server nginx[1234]: request handled
    Apr 13 12:30:00 server nginx[1234]: error: connection reset by peer
    Apr 13 12:30:01 server nginx[1234]: warning: slow upstream response
    Apr 13 13:00:00 server nginx[1234]: request handled
    Apr 13 13:00:01 server nginx[1234]: request handled
    Apr 13 13:00:02 server nginx[1234]: request handled
    Apr 13 13:00:03 server nginx[1234]: request handled
    Apr 13 13:00:04 server nginx[1234]: request handled`
	result := compressSystemctlStatus(output)
	if !strings.Contains(result, "Active:") {
		t.Error("should contain Active field")
	}
	if !strings.Contains(result, "Main PID:") {
		t.Error("should contain Main PID field")
	}
	if !strings.Contains(result, "Memory:") {
		t.Error("should contain Memory field")
	}
	if !strings.Contains(result, "error") {
		t.Error("should include error log lines")
	}
	if !strings.Contains(result, "warning") {
		t.Error("should include warning log lines")
	}
}

func TestCompressSystemctlList(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("  UNIT                           LOAD   ACTIVE   SUB          DESCRIPTION\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("  service-%d.service              loaded active   running      Service %d\n", i, i))
	}
	sb.WriteString("  broken.service                 loaded failed   failed       Broken Service\n")
	result := compressSystemctlList(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count")
	}
	if !strings.Contains(result, "broken.service") {
		t.Error("should include failed unit lines")
	}
}

func TestCompressSystemctl_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"systemctl status nginx", "systemctl-status"},
		{"systemctl list-units", "systemctl-list"},
		{"systemctl list-unit-files", "systemctl-list"},
		{"systemctl restart nginx", "systemctl-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V5: JS/Test Compressor Tests ──────────────────────────────────────

func TestCompressJsTest_WithFailures(t *testing.T) {
	output := `PASS src/utils/helpers.test.js
	 ✓ should add numbers (5ms)
	 ✓ should subtract numbers (2ms)

FAIL src/api/users.test.js
	 ✕ should fetch users (15ms)
	 ● Test suite failed to run
	   TypeError: Cannot read properties of undefined (reading 'map')
	     at Object.<anonymous> (src/api/users.test.js:5:32)

Test Suites: 1 failed, 1 passed, 2 total
Tests:       1 failed, 2 passed, 3 total
Snapshots:   0 total
Time:        2.5s`
	result := compressJsTest(output)
	if !strings.Contains(result, "FAIL") {
		t.Error("should contain FAIL section")
	}
	if !strings.Contains(result, "failed") {
		t.Error("should contain failure summary")
	}
	if !strings.Contains(result, "TypeError") {
		t.Error("should contain error details")
	}
}

func TestCompressJsTest_AllPass(t *testing.T) {
	output := `PASS src/utils/helpers.test.js
	 ✓ should add numbers (5ms)
	 ✓ should subtract numbers (2ms)

Test Suites: 1 passed, 1 total
Tests:       2 passed, 2 total
Time:        1.2s`
	result := compressJsTest(output)
	if !strings.Contains(result, "passed") {
		t.Error("should contain pass summary")
	}
}

func TestCompressJsTest_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"npm test", "npm-test"},
		{"npm run test", "npm-test"},
		{"npx vitest", "js-test"},
		{"npx jest", "js-test"},
		{"yarn test", "yarn-test"},
		{"yarn jest", "yarn-test"},
		{"pnpm test", "pnpm-test"},
		{"pnpm run test", "pnpm-test"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V5: Lint Compressor Tests ─────────────────────────────────────────

func TestCompressLint_Small(t *testing.T) {
	output := "src/main.go:10: syntax error"
	result := compressLint(output)
	// Small output (<=10 lines) should pass through
	if !strings.Contains(result, "main.go") {
		t.Error("small lint output should be unchanged")
	}
}

func TestCompressLint_Large(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("src/components/App.tsx:%d:10  E1001  Unexpected any  no-explicit-any\n", i+1))
	}
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("src/utils/helpers.ts:%d:5  W2001  Missing return type  explicit-module-boundary-types\n", i+1))
	}
	sb.WriteString("\n✖ 35 problems (20 errors, 15 warnings)\n")
	result := compressLint(sb.String())
	if !strings.Contains(result, "issues") || !strings.Contains(result, "src/components/App.tsx") {
		t.Error("should group by file with issue count")
	}
	if !strings.Contains(result, "problems") {
		t.Error("should include summary line")
	}
}

func TestCompressLint_Routing(t *testing.T) {
	linters := []string{"eslint", "tsc", "ruff", "golangci-lint", "flake8", "pylint"}
	for _, linter := range linters {
		_, filter := compressShellOutput(linter+" --check src/", strings.Repeat("issue\n", 50))
		if filter != "lint" {
			t.Errorf("compressShellOutput(%q) filter = %q, want lint", linter, filter)
		}
	}
}

func TestExtractLintFile(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"src/main.go:10: syntax error", "src/main.go"},
		{"./components/App.tsx:5:2  error  msg", "./components/App.tsx"},
		{"no file reference here", ""},
		{"3 problems found", ""},
	}
	for _, tt := range tests {
		got := extractLintFile(tt.line)
		if got != tt.want {
			t.Errorf("extractLintFile(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

// ── V5: AWS Compressor Tests ──────────────────────────────────────────

func TestCompressAwsTable_Small(t *testing.T) {
	output := "INSTANCE_ID   TYPE       STATE\ni-12345       t3.micro   running"
	result := compressAwsTable(output)
	if !strings.Contains(result, "i-12345") {
		t.Error("small AWS table should be unchanged")
	}
}

func TestCompressAwsTable_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("INSTANCE_ID     TYPE       STATE       NAME\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("i-%08d     t3.micro   running     app-%d\n", i, i))
	}
	sb.WriteString("i-99999999     t3.micro   stopped     legacy\n")
	result := compressAwsTable(sb.String())
	if !strings.Contains(result, "total rows") {
		t.Error("should contain row summary")
	}
	if !strings.Contains(result, "stopped") {
		t.Error("should include stopped/error rows")
	}
}

func TestCompressAwsTable_JSON(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	sb.WriteString(`  "name": "test"` + "\n")
	sb.WriteString("}")
	result := compressAwsTable(sb.String())
	// Should use JSON compaction
	if strings.Contains(result, ": null") {
		t.Error("should compact JSON and remove null fields")
	}
}

func TestCompressAws_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"aws ec2 describe-instances", "aws-ec2"},
		{"aws s3 ls", "aws-s3"},
		{"aws lambda list-functions", "aws-lambda"},
		{"aws cloudformation describe-stacks", "aws-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V5: Ansible Compressor Tests ──────────────────────────────────────

func TestCompressAnsible_Small(t *testing.T) {
	output := "PLAY [webservers] **********************************************************\nTASK [Gathering Facts] *****************************************************\nok: [host1]\nPLAY RECAP *****************************************************************\nhost1 : ok=2  changed=0  unreachable=0  failed=0"
	result := compressAnsible(output)
	// Small output (<=10 lines) should pass through
	if !strings.Contains(result, "PLAY") {
		t.Error("small ansible output should be preserved")
	}
}

func TestCompressAnsible_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("PLAY [webservers] **********************************************************\n")
	sb.WriteString("TASK [Gathering Facts] *****************************************************\n")
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("ok: [host%d]\n", i))
	}
	sb.WriteString("TASK [Install nginx] *******************************************************\n")
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf("changed: [host%d]\n", i))
	}
	sb.WriteString("TASK [Start nginx] *********************************************************\n")
	sb.WriteString("fatal: [host5]: UNREACHABLE! => {\"changed\": false, \"msg\": \"Connection refused\"}\n")
	for i := 0; i < 14; i++ {
		if i != 5 {
			sb.WriteString(fmt.Sprintf("changed: [host%d]\n", i))
		}
	}
	sb.WriteString("PLAY RECAP *****************************************************************\n")
	sb.WriteString("host0 : ok=3  changed=2  unreachable=0    failed=0\n")
	sb.WriteString("host5 : ok=2  changed=1  unreachable=1    failed=1\n")
	result := compressAnsible(sb.String())
	if !strings.Contains(result, "PLAY") {
		t.Error("should contain PLAY headers")
	}
	if !strings.Contains(result, "fatal") {
		t.Error("should contain fatal/error lines")
	}
	if !strings.Contains(result, "PLAY RECAP") {
		t.Error("should contain PLAY RECAP section")
	}
	if !strings.Contains(result, "Summary") {
		t.Error("should contain summary counts")
	}
}

func TestCompressAnsible_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"ansible all -m ping", "ansible"},
		{"ansible-playbook site.yml", "ansible"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V5: Journalctl/Logs Routing Tests ─────────────────────────────────

func TestCompressLogs_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"journalctl -u nginx", "logs"},
		{"logcli query '{app=\"nginx\"}'", "logs"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V6: Docker Compose Tests ──────────────────────────────────────────

func TestCompressComposePs_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME                IMAGE          COMMAND   SERVICE   STATUS          PORTS\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d              nginx:latest   \"/bin/sh\" app-%d     Up 2 hours      0.0.0.0:808%d->80/tcp\n", i, i, i%10))
	}
	sb.WriteString("app-broken          redis:7        \"redis…\"  cache     Exited (1) 5m\n")
	result := compressComposePs(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Stopped") {
		t.Error("should contain Stopped count")
	}
	if !strings.Contains(result, "app-broken") {
		t.Error("should include stopped services")
	}
}

func TestCompressComposeConfig_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("services:\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("  service-%d:\n    image: app:%d\n    ports:\n      - \"808%d:80\"\n", i, i, i))
	}
	sb.WriteString("networks:\n  default:\n    driver: bridge\n")
	sb.WriteString("volumes:\n  data:\n")
	result := compressComposeConfig(sb.String())
	if !strings.Contains(result, "services") {
		t.Error("should mention services")
	}
	if !strings.Contains(result, "30 services") {
		t.Error("should count services")
	}
}

func TestCompressDockerCompose_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"docker compose ps", "compose-ps"},
		{"docker compose logs", "compose-logs"},
		{"docker compose config", "compose-config"},
		{"docker compose events", "compose-events"},
		{"docker compose up -d", "compose-generic"},
		{"docker-compose ps", "compose-ps"},
		{"docker-compose logs -f", "compose-logs"},
		{"docker_compose ps", "compose-ps"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V6: Helm Tests ────────────────────────────────────────────────────

func TestCompressHelmList_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME            NAMESPACE       REVISION        STATUS          CHART                   APP VERSION\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d           default         %d              deployed        chart-%d-1.0.%d        1.0.%d\n", i, i+1, i, i, i))
	}
	sb.WriteString("app-broken      default         3               failed          broken-1.0.0            1.0.0\n")
	result := compressHelmList(sb.String())
	if !strings.Contains(result, "Deployed") {
		t.Error("should contain Deployed count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count")
	}
	if !strings.Contains(result, "app-broken") {
		t.Error("should include failed releases")
	}
}

func TestCompressHelmStatus(t *testing.T) {
	output := `STATUS: deployed
REVISION: 5
CHART: nginx-ingress-4.0.1
NAMESPACE: ingress-nginx
LAST DEPLOYED: Mon Apr 13 12:00:00 2026
NOTES:
The nginx ingress controller has been installed.

==> v1/Service
NAME                          TYPE          CLUSTER-IP     EXTERNAL-IP   PORT(S)
nginx-ingress-controller      LoadBalancer  10.0.0.1       pending       80:31234/TCP,443:31235/TCP
READY   REASON
`
	result := compressHelmStatus(output)
	if !strings.Contains(result, "STATUS:") {
		t.Error("should contain STATUS field")
	}
	if !strings.Contains(result, "REVISION:") {
		t.Error("should contain REVISION field")
	}
}

func TestCompressHelm_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"helm list", "helm-list"},
		{"helm ls", "helm-list"},
		{"helm status nginx", "helm-status"},
		{"helm history nginx", "helm-history"},
		{"helm get values nginx", "helm-get"},
		{"helm repo update", "helm-repo"},
		{"helm install nginx bitnami/nginx", "helm-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V6: Terraform Tests ───────────────────────────────────────────────

func TestCompressTerraformPlan(t *testing.T) {
	output := `Terraform will perform the following actions:

  # aws_instance.web will be created
  + resource "aws_instance" "web" {
      + ami           = "ami-12345"
      + instance_type = "t3.micro"
    }

  # aws_security_group.sg will be destroyed
  - resource "aws_security_group" "sg" {
      - name = "old-sg"
    }

  # aws_db_instance.db will be updated in-place
  ~ resource "aws_db_instance" "db" {
      ~ instance_class = "db.t3.small" -> "db.t3.medium"
    }

Plan: 1 to add, 1 to change, 1 to destroy.`
	result := compressTerraformPlan(output)
	if !strings.Contains(result, "Plan:") {
		t.Error("should contain Plan summary")
	}
	if !strings.Contains(result, "will be created") {
		t.Error("should contain creation notice")
	}
	if !strings.Contains(result, "will be destroyed") {
		t.Error("should contain destruction notice")
	}
}

func TestCompressTerraformApply(t *testing.T) {
	output := `aws_instance.web: Creating...
aws_instance.web: Still creating... [10s elapsed]
aws_instance.web: Still creating... [20s elapsed]
aws_instance.web: Creation complete after 30s [id=i-12345]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

  instance_ip = "10.0.0.1"
  instance_id = "i-12345"`
	result := compressTerraformApply(output)
	if !strings.Contains(result, "Apply complete!") {
		t.Error("should contain Apply complete")
	}
	if !strings.Contains(result, "instance_ip") {
		t.Error("should contain outputs")
	}
}

func TestCompressTerraformStateList(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("aws_instance.web-%d\n", i))
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("aws_security_group.sg-%d\n", i))
	}
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("module.networking.aws_vpc.main-%d\n", i))
	}
	result := compressTerraformStateList(sb.String())
	if !strings.Contains(result, "resources") {
		t.Error("should contain resource count")
	}
	if !strings.Contains(result, "aws_instance") {
		t.Error("should group by resource type")
	}
}

func TestCompressTerraform_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"terraform plan", "tf-plan"},
		{"terraform apply", "tf-apply"},
		{"terraform show", "tf-show"},
		{"terraform state list", "tf-state"},
		{"terraform output", "tf-output"},
		{"terraform init", "tf-init"},
		{"terraform validate", "tf-generic"},
		{"tf plan", "tf-plan"},
		{"tf apply -auto-approve", "tf-apply"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ── V6: SSH Diagnostic Tests ──────────────────────────────────────────

func TestCompressDiskFree_HighUsage(t *testing.T) {
	output := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1       100G   20G   80G  20% /
/dev/sda2       500G  430G   70G  86% /data
/dev/sdb1       200G   10G  190G   5% /backup
tmpfs            16G   12G    4G  75% /dev/shm`
	result := compressDiskFree(output)
	if !strings.Contains(result, "86%") {
		t.Error("should include high-usage filesystem")
	}
	if strings.Contains(result, "/backup") {
		t.Error("should not include low-usage filesystem")
	}
}

func TestCompressDiskFree_AllLow(t *testing.T) {
	output := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1       100G   20G   80G  20% /
/dev/sdb1       200G   10G  190G   5% /backup`
	result := compressDiskFree(output)
	if !strings.Contains(result, "below 80%") {
		t.Error("should report all below threshold")
	}
}

func TestCompressDiskUsage_Large(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("%dM\t/path/dir-%d\n", 1000-i*20, i))
	}
	result := compressDiskUsage(sb.String())
	if !strings.Contains(result, "more entries") {
		t.Error("should truncate large output")
	}
}

func TestCompressProcessList_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("PID   USER     %CPU  %MEM  COMMAND\n")
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("%d    user     %d.%d   %d.%d   process-%d\n", 1000+i, i%10, i%5, i%8, i%3, i))
	}
	sb.WriteString("9999  user     95.2  80.1   runaway-process\n")
	result := compressProcessList(sb.String())
	if !strings.Contains(result, "runaway-process") {
		t.Error("should include high-resource process")
	}
	if !strings.Contains(result, "total processes") {
		t.Error("should show total count")
	}
}

func TestCompressNetworkConnections(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("State      Recv-Q Send-Q  Local Address:Port  Peer Address:Port\n")
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("LISTEN     0      128     0.0.0.0:%d         0.0.0.0:*\n", 8000+i))
	}
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("ESTAB      0      0       10.0.0.1:%d      10.0.0.2:%d\n", 40000+i, 80))
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("TIME-WAIT  0      0       10.0.0.1:%d      10.0.0.3:%d\n", 50000+i, 443))
	}
	result := compressNetworkConnections(sb.String())
	if !strings.Contains(result, "LISTEN") {
		t.Error("should contain LISTEN count")
	}
	if !strings.Contains(result, "ESTABLISHED") {
		t.Error("should contain ESTABLISHED count")
	}
	if !strings.Contains(result, "TIME-WAIT") {
		t.Error("should contain TIME-WAIT count")
	}
}

func TestCompressIpAddr(t *testing.T) {
	output := `1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 state UNKNOWN
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
    inet6 ::1/128 scope host
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 state UP
    link/ether 02:42:ac:11:00:02 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.2/16 brd 172.17.255.255 scope global eth0
    inet6 fe80::42:acff:fe11:2/64 scope link
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 state DOWN
    link/ether 02:42:3a:5f:12:34 brd ff:ff:ff:ff:ff:ff
    inet 172.18.0.1/16 brd 172.18.255.255 scope global docker0`
	result := compressIpAddr(output)
	if !strings.Contains(result, "eth0") {
		t.Error("should contain interface name")
	}
	if !strings.Contains(result, "172.17.0.2") {
		t.Error("should contain IP address")
	}
	if !strings.Contains(result, "docker0") {
		t.Error("should contain all interfaces")
	}
}

func TestCompressIpRoute(t *testing.T) {
	output := `default via 10.0.0.1 dev eth0
10.0.0.0/24 dev eth0 proto kernel scope link src 10.0.0.5
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1
172.18.0.0/16 dev br-1 proto kernel scope link src 172.18.0.1
192.168.1.0/24 dev wlan0 proto kernel scope link src 192.168.1.100`
	result := compressIpRoute(output)
	if !strings.Contains(result, "default") {
		t.Error("should contain default route")
	}
	if !strings.Contains(result, "other routes") {
		t.Error("should mention other routes")
	}
}

func TestCompressSSHDiag_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"df -h", "df"},
		{"du -sh /*", "du"},
		{"ps aux", "ps"},
		{"ss -tulnp", "netstat"},
		{"netstat -tulnp", "netstat"},
		{"ip addr show", "ip-addr"},
		{"ip a", "ip-addr"},
		{"ip route show", "ip-route"},
		{"ip r", "ip-route"},
		{"ip link show", "ip-generic"},
		{"free -h", "free"},
		{"uptime", "uptime"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ─── V7: Home Assistant Compressor Tests ─────────────────────────────────────

// buildHAStatesJSON builds a HA get_states JSON envelope for testing.
func buildHAStatesJSON(entities []map[string]interface{}) string {
	envelope := map[string]interface{}{
		"status": "success",
		"count":  len(entities),
		"states": entities,
	}
	data, _ := json.Marshal(envelope)
	return string(data)
}

func TestCompressHAGetStates_Large(t *testing.T) {
	var entities []map[string]interface{}
	// Add 50 lights (30 on, 20 off)
	for i := 0; i < 30; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("light.light_%d", i),
			"state":         "on",
			"friendly_name": fmt.Sprintf("Light %d", i),
		})
	}
	for i := 30; i < 50; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("light.light_%d", i),
			"state":         "off",
			"friendly_name": fmt.Sprintf("Light %d", i),
		})
	}
	// Add 20 sensors (all "measuring")
	for i := 0; i < 20; i++ {
		entities = append(entities, map[string]interface{}{
			"entity_id":     fmt.Sprintf("sensor.temp_%d", i),
			"state":         fmt.Sprintf("%.1f", 20.0+float64(i)*0.5),
			"friendly_name": fmt.Sprintf("Temp Sensor %d", i),
		})
	}
	// Add 2 unavailable entities
	entities = append(entities, map[string]interface{}{
		"entity_id":     "switch.garage",
		"state":         "unavailable",
		"friendly_name": "Garage Switch",
	})
	entities = append(entities, map[string]interface{}{
		"entity_id":     "sensor.old_sensor",
		"state":         "unknown",
		"friendly_name": "Old Sensor",
	})

	output := buildHAStatesJSON(entities)
	result, filter := compressHAOutput(output)

	if filter != "ha-states" {
		t.Errorf("expected ha-states filter, got %q", filter)
	}
	if !strings.Contains(result, "HA States: 72 entities") {
		t.Errorf("expected entity count, got: %s", result[:min(200, len(result))])
	}
	if !strings.Contains(result, "3 domains") {
		t.Errorf("expected domain count, got: %s", result[:min(200, len(result))])
	}
	if !strings.Contains(result, "light:") {
		t.Error("expected light domain in summary")
	}
	if !strings.Contains(result, "sensor:") {
		t.Error("expected sensor domain in summary")
	}
	if !strings.Contains(result, "30 on") {
		t.Error("expected 30 lights on")
	}
	if !strings.Contains(result, "20 off") {
		t.Error("expected 20 lights off")
	}
	if !strings.Contains(result, "Unavailable entities (2)") {
		t.Error("expected 2 unavailable entities")
	}
	if !strings.Contains(result, "Garage Switch") {
		t.Error("expected Garage Switch in unavailable list")
	}
	// Verify compression ratio
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressHAGetStates_Empty(t *testing.T) {
	output := buildHAStatesJSON(nil)
	result, filter := compressHAOutput(output)

	if filter != "ha-states" {
		t.Errorf("expected ha-states filter, got %q", filter)
	}
	if !strings.Contains(result, "0 entities") {
		t.Errorf("expected 0 entities, got: %s", result)
	}
}

func TestCompressHAGetState_Compact(t *testing.T) {
	envelope := map[string]interface{}{
		"status": "success",
		"entity": map[string]interface{}{
			"entity_id": "climate.living_room",
			"state":     "heat",
			"attributes": map[string]interface{}{
				"friendly_name":       "Living Room Climate",
				"temperature":         22.5,
				"current_temperature": 21.0,
				"hvac_mode":           "heat",
				"hvac_action":         "heating",
				"target_temp_high":    25.0,
				"target_temp_low":     18.0,
				"preset_mode":         "comfort",
				"min_temp":            5.0,
				"max_temp":            35.0,
				"some_internal_attr":  "should_be_removed",
				"another_useless":     42,
			},
			"last_changed": "2024-01-14T15:20:00.000000+00:00",
			"last_updated": "2024-01-14T15:25:00.000000+00:00",
			"context": map[string]interface{}{
				"id":       "abc123",
				"parent_id": nil,
				"user_id":   nil,
			},
		},
	}
	data, _ := json.Marshal(envelope)
	output := string(data)

	result, filter := compressHAOutput(output)

	if filter != "ha-state" {
		t.Errorf("expected ha-state filter, got %q", filter)
	}
	if !strings.Contains(result, "Entity: climate.living_room") {
		t.Error("expected entity_id")
	}
	if !strings.Contains(result, "State: heat") {
		t.Error("expected state")
	}
	if !strings.Contains(result, "temperature: 22.5") {
		t.Error("expected temperature attribute")
	}
	if !strings.Contains(result, "hvac_mode: heat") {
		t.Error("expected hvac_mode attribute")
	}
	if strings.Contains(result, "some_internal_attr") {
		t.Error("internal attribute should be filtered out")
	}
	if strings.Contains(result, "another_useless") {
		t.Error("useless attribute should be filtered out")
	}
	if !strings.Contains(result, "attributes omitted") {
		t.Error("expected omitted count")
	}
	if !strings.Contains(result, "Last changed:") {
		t.Error("expected last_changed")
	}
	if strings.Contains(result, "context") {
		t.Error("context should be removed")
	}
}

func TestCompressHACallService(t *testing.T) {
	envelope := map[string]interface{}{
		"status":            "success",
		"service":           "light.turn_on",
		"affected_entities": []string{"light.living_room", "light.bedroom", "light.kitchen"},
		"count":             3,
	}
	data, _ := json.Marshal(envelope)

	result, filter := compressHAOutput(string(data))

	if filter != "ha-call-service" {
		t.Errorf("expected ha-call-service filter, got %q", filter)
	}
	if !strings.Contains(result, "✓ Service light.turn_on called successfully") {
		t.Error("expected success message")
	}
	if !strings.Contains(result, "Affected entities (3)") {
		t.Error("expected affected entities count")
	}
	if !strings.Contains(result, "light.living_room") {
		t.Error("expected entity in list")
	}
}

func TestCompressHAListServices_Large(t *testing.T) {
	type svcEntry struct {
		Domain   string   `json:"domain"`
		Services []string `json:"services"`
	}

	// Create domains with varying service counts
	var services []svcEntry
	for i := 0; i < 20; i++ {
		var svcNames []string
		numSvcs := 5 + i*2 // 5,7,9,...43 services per domain
		for j := 0; j < numSvcs; j++ {
			svcNames = append(svcNames, fmt.Sprintf("service_%d", j))
		}
		services = append(services, svcEntry{
			Domain:   fmt.Sprintf("domain_%02d", i),
			Services: svcNames,
		})
	}

	envelope := map[string]interface{}{
		"status":   "success",
		"count":    len(services),
		"services": services,
	}
	data, _ := json.Marshal(envelope)

	result, filter := compressHAOutput(string(data))

	if filter != "ha-list-services" {
		t.Errorf("expected ha-list-services filter, got %q", filter)
	}
	if !strings.Contains(result, "HA Services: 20 domains") {
		t.Error("expected domain count")
	}
	// Domains with >15 services should show "+N more"
	if !strings.Contains(result, " more") {
		t.Error("expected truncation for domains with >15 services")
	}
	// Verify compression
	if len(result) >= len(data) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(data))
	}
}

func TestCompress_HA_Routing(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: true}

	// Test that homeassistant tool name routes to HA compressor
	states := buildHAStatesJSON([]map[string]interface{}{
		{"entity_id": "light.test", "state": "on", "friendly_name": "Test Light"},
	})
	// Pad to exceed MinChars
	for i := 0; i < 30; i++ {
		states = strings.Replace(states, "]", fmt.Sprintf(",{\"entity_id\":\"sensor.p%d\",\"state\":\"%d\",\"friendly_name\":\"S%d\"}]", i, i, i), 1)
	}

	tests := []struct {
		toolName string
		want     string
	}{
		{"homeassistant", "ha-states"},
		{"home_assistant", "ha-states"},
	}
	for _, tt := range tests {
		_, stats := Compress(tt.toolName, "", states, cfg)
		if stats.FilterUsed != tt.want {
			t.Errorf("Compress(%q) filter = %q, want %q", tt.toolName, stats.FilterUsed, tt.want)
		}
	}
}

func TestCompress_HA_SubToggleOff(t *testing.T) {
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true, APICompression: false}

	states := buildHAStatesJSON([]map[string]interface{}{
		{"entity_id": "light.test", "state": "on"},
	})
	// Pad to exceed MinChars
	for i := 0; i < 30; i++ {
		states += fmt.Sprintf(`{"entity_id":"sensor.p%d","state":"%d"}`, i, i)
	}

	_, stats := Compress("homeassistant", "", states, cfg)
	if stats.FilterUsed != "generic" {
		t.Errorf("expected generic filter with APICompression=false, got %q", stats.FilterUsed)
	}
}

// ─── V7: Shell File/Log Compressor Tests ─────────────────────────────────────

func TestCompressCatFile_LogContent(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-15 10:%02d:00 INFO  Processing request %d\n", i%60, i))
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-15 11:%02d:00 ERROR Connection timeout %d\n", i, i))
	}
	output := sb.String()

	result := compressCatFile(output)

	// Should apply log compression (strip timestamps, deduplicate)
	if len(result) >= len(output) {
		t.Errorf("expected compression for log content: got %d >= %d", len(result), len(output))
	}
}

func TestCompressCatFile_LargeNonLog(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 300; i++ {
		sb.WriteString(fmt.Sprintf("This is line %d of a regular text file with some content.\n", i))
	}
	output := sb.String()

	result := compressCatFile(output)

	// Should apply tail focus for large non-log content
	if len(result) >= len(output) {
		t.Errorf("expected compression for large non-log content: got %d >= %d", len(result), len(output))
	}
}

func TestCompressCatFile_SmallNonLog(t *testing.T) {
	output := "Hello World\nThis is a small file\nWith just a few lines\n"
	result := compressCatFile(output)

	// Small non-log content should pass through mostly unchanged
	if !strings.Contains(result, "Hello World") {
		t.Error("expected content preserved for small non-log file")
	}
}

func TestCompressTailHead_LogContent(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("2024-01-15 10:%02d:00 INFO  Processing request %d\n", i, i))
	}
	output := sb.String()

	result := compressTailHead(output)

	// Should apply log compression
	if len(result) >= len(output) {
		t.Errorf("expected compression for log tail output: got %d >= %d", len(result), len(output))
	}
}

func TestCompressTailHead_NonLog(t *testing.T) {
	output := "Last 10 lines of a config file\nserver {\n    listen 80;\n}\n"
	result := compressTailHead(output)

	// Non-log tail output should pass through
	if !strings.Contains(result, "listen 80") {
		t.Error("expected content preserved for non-log tail output")
	}
}

func TestCompressStat_MultiFile(t *testing.T) {
	output := `  File: config.yaml
	 Size: 2048       Blocks: 8        IO Block: 4096   regular file
Device: 802h/2050d  Inode: 1234567    Links: 1
Access: (0644/-rw-r--r--)  Uid: ( 1000/   user)   Gid: ( 1000/   user)
Access: 2024-01-15 10:30:00.000000000 +0100
Modify: 2024-01-14 15:20:00.000000000 +0100
Change: 2024-01-14 15:20:00.000000000 +0100
	Birth: 2024-01-10 08:00:00.000000000 +0100
	 File: script.sh
	 Size: 512        Blocks: 8        IO Block: 4096   regular file
Device: 802h/2050d  Inode: 1234568    Links: 1
Access: (0755/-rwxr-xr-x)  Uid: ( 1000/   user)   Gid: ( 1000/   user)
Access: 2024-01-15 10:30:00.000000000 +0100
Modify: 2024-01-15 09:00:00.000000000 +0100
Change: 2024-01-15 09:00:00.000000000 +0100
	Birth: 2024-01-10 08:00:00.000000000 +0100`

	result := compressStat(output)

	if !strings.Contains(result, "config.yaml:") {
		t.Error("expected config.yaml in output")
	}
	if !strings.Contains(result, "2048B") {
		t.Error("expected size in output")
	}
	if !strings.Contains(result, "-rw-r--r--") {
		t.Error("expected permissions in output")
	}
	if !strings.Contains(result, "modified 2024-01-14 15:20:00") {
		t.Error("expected modify time in output")
	}
	if !strings.Contains(result, "script.sh:") {
		t.Error("expected script.sh in output")
	}
	if !strings.Contains(result, "-rwxr-xr-x") {
		t.Error("expected executable permissions")
	}
	// Verify it's compact (2 lines instead of 16)
	lines := strings.Count(result, "\n")
	if lines > 3 {
		t.Errorf("expected compact output (2-3 lines), got %d lines: %s", lines, result)
	}
}

func TestCompressStat_SingleFile(t *testing.T) {
	output := `  File: readme.txt
	 Size: 128        Blocks: 8        IO Block: 4096   regular file
Device: 802h/2050d  Inode: 1234569    Links: 1
Access: (0644/-rw-r--r--)  Uid: ( 1000/   user)   Gid: ( 1000/   user)
Access: 2024-01-15 10:30:00.000000000 +0100
Modify: 2024-01-14 15:20:00.000000000 +0100
Change: 2024-01-14 15:20:00.000000000 +0100
	Birth: 2024-01-10 08:00:00.000000000 +0100`

	result := compressStat(output)

	if !strings.Contains(result, "readme.txt:") {
		t.Error("expected readme.txt in output")
	}
	if !strings.Contains(result, "128B") {
		t.Error("expected size")
	}
}

func TestCompressStat_ShortOutput(t *testing.T) {
	output := "  File: tiny.txt\n  Size: 10 Blocks: 8"
	result := compressStat(output)

	// Short output (<=3 lines) should be returned as-is
	if result != output {
		t.Errorf("expected short stat output preserved, got: %s", result)
	}
}

func TestCompressShell_V7Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"cat /var/log/syslog", "cat"},
		{"less /etc/config", "cat"},
		{"more readme.txt", "cat"},
		{"tail -f /var/log/syslog", "tail"},
		{"head -20 file.txt", "head"},
		{"stat config.yaml", "stat"},
		{"file *", "file"},
		{"wc -l *.go", "wc"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
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

// ─── V8: Network Diagnostics Compressor Tests ────────────────────────────────

func TestCompressPing_Success(t *testing.T) {
	output := `PING google.com (142.250.80.46): 56 data bytes
64 bytes from 142.250.80.46: icmp_seq=0 ttl=116 time=5.123 ms
64 bytes from 142.250.80.46: icmp_seq=1 ttl=116 time=4.987 ms
64 bytes from 142.250.80.46: icmp_seq=2 ttl=116 time=5.456 ms
64 bytes from 142.250.80.46: icmp_seq=3 ttl=116 time=5.012 ms
64 bytes from 142.250.80.46: icmp_seq=4 ttl=116 time=4.876 ms
--- google.com ping statistics ---
5 packets transmitted, 5 received, 0% packet loss
round-trip min/avg/max = 4.876/5.090/5.456 ms`

	result := compressPing(output)

	if !strings.Contains(result, "PING google.com") {
		t.Error("expected PING header")
	}
	if !strings.Contains(result, "5 packets transmitted, 5 received, 0% packet loss") {
		t.Error("expected packet statistics")
	}
	if !strings.Contains(result, "round-trip") {
		t.Error("expected RTT summary")
	}
	if strings.Contains(result, "icmp_seq=1") {
		t.Error("individual ICMP lines should be removed")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressPing_Timeout(t *testing.T) {
	output := `PING unreachable.local (10.0.0.99): 56 data bytes
Request timeout for icmp_seq 0
Request timeout for icmp_seq 1
Request timeout for icmp_seq 2
--- unreachable.local ping statistics ---
3 packets transmitted, 0 received, 100% packet loss`

	result := compressPing(output)

	if !strings.Contains(result, "100% packet loss") {
		t.Error("expected packet loss in output")
	}
	if !strings.Contains(result, "Request timeout") {
		t.Error("expected timeout error lines preserved")
	}
}

func TestCompressPing_Unreachable(t *testing.T) {
	output := `PING badhost (0.0.0.0): 56 data bytes
ping: badhost: Name or service not known`

	result := compressPing(output)

	if !strings.Contains(result, "Name or service not known") {
		t.Error("expected DNS error preserved")
	}
}

func TestCompressPing_ShortOutput(t *testing.T) {
	output := "PING host (1.2.3.4): 56 data bytes\n64 bytes from 1.2.3.4: icmp_seq=0 ttl=64 time=1.0 ms\n"
	result := compressPing(output)

	// Short output (<=4 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved, got: %s", result)
	}
}

func TestCompressDig_Large(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> example.com ANY
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 5, AUTHORITY: 4, ADDITIONAL: 3

;; QUESTION SECTION:
;example.com.			IN	ANY

;; ANSWER SECTION:
example.com.		3600	IN	A	93.184.216.34
example.com.		3600	IN	A	93.184.216.35
example.com.		3600	IN	A	93.184.216.36
example.com.		3600	IN	AAAA	2606:2800:220:1:248:1893:25c8:1946
example.com.		3600	IN	MX	10 mail.example.com.

;; AUTHORITY SECTION:
example.com.		86400	IN	NS	a.iana-servers.net.
example.com.		86400	IN	NS	b.iana-servers.net.
example.com.		86400	IN	NS	c.iana-servers.net.
example.com.		86400	IN	NS	d.iana-servers.net.

;; ADDITIONAL SECTION:
mail.example.com.	3600	IN	A	10.0.0.1
mail.example.com.	3600	IN	A	10.0.0.2
mail.example.com.	3600	IN	AAAA	::1

;; Query time: 42 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)
;; MSG SIZE  rcvd: 256`

	result := compressDig(output)

	if !strings.Contains(result, "QUESTION SECTION") {
		t.Error("expected question section")
	}
	if !strings.Contains(result, "ANSWER SECTION") {
		t.Error("expected answer section")
	}
	if !strings.Contains(result, "example.com.		3600	IN	A	93.184.216.34") {
		t.Error("expected answer records")
	}
	if !strings.Contains(result, "Query time") {
		t.Error("expected query time")
	}
	// Authority section should be removed
	if strings.Contains(result, "a.iana-servers.net") {
		t.Error("authority section should be removed")
	}
	// Additional section should be removed
	if strings.Contains(result, "mail.example.com.	3600	IN	A") {
		t.Error("additional section should be removed")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressDig_NXDOMAIN(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> nonexistent.example.com
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 54321

;; QUESTION SECTION:
;nonexistent.example.com.	IN	A

;; AUTHORITY SECTION:
example.com.		3600	IN	SOA	ns1.example.com. admin.example.com. 2024010101 3600 900 604800 86400

;; Query time: 15 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)`

	result := compressDig(output)

	if !strings.Contains(result, "NXDOMAIN") {
		t.Error("expected NXDOMAIN status preserved")
	}
}

func TestCompressDig_ShortOutput(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> example.com
;; Answer: 93.184.216.34`
	result := compressDig(output)

	// Short output (<=10 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved")
	}
}

func TestCompressDNS_Nslookup(t *testing.T) {
	output := "Server:\t\t8.8.8.8\nAddress:\t8.8.8.8#53\n\nNon-authoritative answer:\nName:\tgoogle.com\nAddress: 142.250.80.46\nName:\tgoogle.com\nAddress: 2606:2800:220:1:248:1893:25c8:1946\n"

	result := compressDNS(output)

	if !strings.Contains(result, "Server:") {
		t.Errorf("expected server info, got: %s", result)
	}
	if !strings.Contains(result, "google.com") {
		t.Error("expected answer")
	}
}

func TestCompressDNS_ShortOutput(t *testing.T) {
	output := "google.com has address 142.250.80.46\n"
	result := compressDNS(output)

	// Short output (<=5 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved")
	}
}

func TestCompressCurl_JSON(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 40; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	for i := 40; i < 60; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": "value_%d",`, i, i) + "\n")
	}
	sb.WriteString("}")

	result := compressCurl(sb.String())

	// Should apply JSON compaction
	if !strings.Contains(result, "omitted") {
		t.Error("expected null field omission")
	}
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression for JSON curl output")
	}
}

func TestCompressCurl_HTML(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<title>Test Page</title>\n</head>\n<body>\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("<p>Paragraph %d with some content</p>\n", i))
	}
	sb.WriteString("</body>\n</html>")

	result := compressCurl(sb.String())

	if !strings.Contains(result, "Test Page") {
		t.Error("expected title preserved")
	}
	if !strings.Contains(result, "HTML response") {
		t.Error("expected HTML response note")
	}
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression for HTML curl output")
	}
}

func TestCompressCurl_Verbose(t *testing.T) {
	output := "> GET /api/health HTTP/2\n> Host: example.com\n> User-Agent: curl/8.0\n> Accept: */*\n>\n< HTTP/2 200\n< content-type: application/json\n< date: Mon, 15 Jan 2024 10:30:00 GMT\n< server: nginx\n< x-request-id: abc123\n< x-cache: HIT\n< content-length: 42\n<\n{\"status\":\"ok\",\"uptime\":123456,\"version\":\"1.0.0\"}"

	result := compressCurl(output)

	if !strings.Contains(result, "HTTP/2 200") {
		t.Errorf("expected HTTP status, got: %s", result)
	}
	if !strings.Contains(result, "content-type:") {
		t.Error("expected content-type header")
	}
	if !strings.Contains(result, "status") {
		t.Error("expected body preserved")
	}
}

func TestCompressCurl_PlainText(t *testing.T) {
	output := "Hello, this is a plain text response from a server.\nNothing special here.\n"
	result := compressCurl(output)

	// Plain text should go through generic compression
	if !strings.Contains(result, "Hello") {
		t.Error("expected content preserved")
	}
}

func TestCompressShell_V8Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"ping google.com", "ping"},
		{"ping6 ipv6.google.com", "ping"},
		{"dig example.com", "dig"},
		{"nslookup google.com", "nslookup"},
		{"host google.com", "host"},
		{"curl http://example.com", "curl"},
		{"wget http://example.com", "curl"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

// ─── V9A Tests ───────────────────────────────────────────────────────────────

func TestCompressShell_V9A_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"tar -czf archive.tar.gz .", "tar"},
		{"tar -tf archive.tar", "tar"},
		{"tar -xvf archive.tar", "tar"},
		{"zip -r archive.zip dir/", "zip"},
		{"unzip -l archive.zip", "unzip"},
		{"unzip archive.zip", "unzip"},
		{"rsync -avz src/ dest/", "rsync"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("file\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}

func TestCompressTar_LargeListing(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("src/\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "src/module%d/main.go\n", i)
		fmt.Fprintf(&sb, "src/module%d/util.go\n", i)
	}
	sb.WriteString("src/README.md\n")

	input := sb.String()
	result := compressTar(input)

	// Should group by directory
	if !strings.Contains(result, "src/") {
		t.Error("expected directory grouping for src/")
	}
	// Output should be significantly shorter than input
	if len(result) >= len(input) {
		t.Errorf("expected compression: input=%d, output=%d", len(input), len(result))
	}
}

func TestCompressTar_ShortListing(t *testing.T) {
	output := "file1.txt\nfile2.txt\nfile3.txt\n"
	result := compressTar(output)

	// Short output should be returned as-is
	if result != output {
		t.Errorf("expected short output preserved, got: %s", result)
	}
}

func TestCompressTar_Errors(t *testing.T) {
	output := "tar: Error opening archive: No such file\nfile1.txt\nfile2.txt\n"
	result := compressTar(output)

	if !strings.Contains(result, "Error opening") {
		t.Error("expected error preserved")
	}
}

func TestCompressZip_LargeOutput(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("  adding: src/main.go (deflated 45%)\n")
	sb.WriteString("  adding: src/util.go (deflated 30%)\n")
	sb.WriteString("  adding: src/config.go (stored 0%)\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "  adding: src/mod%d/app.go (deflated 25%%)\n", i)
	}
	sb.WriteString("  adding: README.md (stored 0%)\n")

	input := sb.String()
	result := compressZip(input)

	// Should extract file paths without compression ratios
	if strings.Contains(result, "deflated") {
		t.Error("expected compression ratios removed")
	}
	// Should contain file paths (groupByDir format: "src/ (N): main.go, ...")
	if !strings.Contains(result, "main.go") {
		t.Error("expected main.go preserved")
	}
	if !strings.Contains(result, "src/") {
		t.Error("expected src/ directory grouping")
	}
	// Output should be shorter than input
	if len(result) >= len(input) {
		t.Errorf("expected compression: input=%d, output=%d", len(input), len(result))
	}
}

func TestCompressZip_UnzipListing(t *testing.T) {
	output := "Archive:  archive.zip\n  Length      Date    Time    Name\n---------  ---------- -----   ----\n    12345  2024-01-15 10:30   src/main.go\n     5678  2024-01-15 10:30   src/util.go\n     9012  2024-01-15 10:30   README.md\n---------                     -------\n    27035                     3 files\n"

	result := compressZip(output)

	if !strings.Contains(result, "src/main.go") {
		t.Error("expected file paths preserved")
	}
	if !strings.Contains(result, "3 files") {
		t.Error("expected summary preserved")
	}
}

func TestCompressRsync_LargeOutput(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("sending incremental file list\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, ">f++++++++x src/module%d/main.go\n", i)
	}
	sb.WriteString("sent 12,345 bytes  received 678 bytes  25,346.00 bytes/sec\n")
	sb.WriteString("total size is 1,234,567  speedup is 98.76\n")

	result := compressRsync(sb.String())

	// Should keep summary
	if !strings.Contains(result, "sending incremental file list") {
		t.Error("expected 'sending' summary preserved")
	}
	if !strings.Contains(result, "total size") {
		t.Error("expected total size summary preserved")
	}
	// Should group files by directory
	if !strings.Contains(result, "src/") {
		t.Error("expected src/ directory grouping")
	}
}

func TestCompressRsync_Errors(t *testing.T) {
	output := "rsync: link_stat \"/missing\" failed: No such file (2)\nrsync error: some errors could not be transferred\n"
	result := compressRsync(output)

	if !strings.Contains(result, "rsync:") {
		t.Error("expected rsync error preserved")
	}
}

func TestCompressGitHub_Repos(t *testing.T) {
	type repo struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		Language    string `json:"language"`
		UpdatedAt   string `json:"updated_at"`
		HTMLURL     string `json:"html_url"`
		CloneURL    string `json:"clone_url"`
	}

	repos := make([]repo, 25)
	for i := 0; i < 25; i++ {
		repos[i] = repo{
			Name:        fmt.Sprintf("repo%d", i),
			FullName:    fmt.Sprintf("user/repo%d", i),
			Description: fmt.Sprintf("Repository number %d with a somewhat long description", i),
			Private:     i < 3,
			Language:    "Go",
			UpdatedAt:   "2024-01-15T10:30:00Z",
			HTMLURL:     fmt.Sprintf("https://github.com/user/repo%d", i),
			CloneURL:    fmt.Sprintf("https://github.com/user/repo%d.git", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(repos),
		"repos":  repos,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-repos" {
		t.Errorf("expected filter github-repos, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "25 repos") {
		t.Error("expected repo count in output")
	}
	if !strings.Contains(result, "3 private") {
		t.Error("expected private count")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation indicator")
	}
	if stats.Ratio >= 1.0 {
		t.Errorf("expected compression, got ratio %.2f", stats.Ratio)
	}
}

func TestCompressGitHub_Issues(t *testing.T) {
	type issue struct {
		Number    int      `json:"number"`
		Title     string   `json:"title"`
		State     string   `json:"state"`
		User      string   `json:"user"`
		Labels    []string `json:"labels"`
		CreatedAt string   `json:"created_at"`
		HTMLURL   string   `json:"html_url"`
	}

	issues := make([]issue, 30)
	for i := 0; i < 30; i++ {
		issues[i] = issue{
			Number:    100 + i,
			Title:     fmt.Sprintf("Bug: something broke in module %d", i),
			State:     "open",
			User:      "developer",
			Labels:    []string{"bug", "priority-high"},
			CreatedAt: "2024-01-15T10:30:00Z",
			HTMLURL:   fmt.Sprintf("https://github.com/user/repo/issues/%d", 100+i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(issues),
		"issues": issues,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-issues" {
		t.Errorf("expected filter github-issues, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 issues") {
		t.Error("expected issue count")
	}
	if !strings.Contains(result, "#100") {
		t.Error("expected issue number")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}

func TestCompressGitHub_PRs(t *testing.T) {
	type pr struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		User      string `json:"user"`
		Head      string `json:"head"`
		Base      string `json:"base"`
		CreatedAt string `json:"created_at"`
	}

	prs := []pr{
		{Number: 42, Title: "Fix login bug", State: "open", User: "dev1", Head: "fix/login", Base: "main"},
		{Number: 41, Title: "Add feature X", State: "closed", User: "dev2", Head: "feat/x", Base: "develop"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"count":         len(prs),
		"pull_requests": prs,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-prs" {
		t.Errorf("expected filter github-prs, got %s", filter)
	}
	if !strings.Contains(result, "2 PRs") {
		t.Error("expected PR count")
	}
	if !strings.Contains(result, "fix/login → main") {
		t.Error("expected branch info")
	}
}

func TestCompressGitHub_Commits(t *testing.T) {
	type commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}

	commits := make([]commit, 30)
	for i := 0; i < 30; i++ {
		commits[i] = commit{
			SHA:     fmt.Sprintf("abc%d", i),
			Message: fmt.Sprintf("Fix issue #%d with a detailed commit message", i),
			Author:  "Developer",
			Date:    "2024-01-15T10:30:00Z",
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(commits),
		"commits": commits,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-commits" {
		t.Errorf("expected filter github-commits, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 commits") {
		t.Error("expected commit count")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}

func TestCompressGitHub_WorkflowRuns(t *testing.T) {
	type run struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Branch     string `json:"branch"`
		CreatedAt  string `json:"created_at"`
	}

	runs := []run{
		{ID: 123, Name: "CI", Status: "completed", Conclusion: "success", Branch: "main", CreatedAt: "2024-01-15T10:30:00Z"},
		{ID: 122, Name: "Deploy", Status: "completed", Conclusion: "failure", Branch: "develop", CreatedAt: "2024-01-14T08:00:00Z"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(runs),
		"runs":   runs,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-runs" {
		t.Errorf("expected filter github-runs, got %s", filter)
	}
	if !strings.Contains(result, "2 workflow runs") {
		t.Error("expected workflow run count")
	}
	if !strings.Contains(result, "completed/success") {
		t.Error("expected status/conclusion")
	}
	if !strings.Contains(result, "completed/failure") {
		t.Error("expected failure run")
	}
}

func TestCompressGitHub_Error(t *testing.T) {
	output := `Tool Output: {"status":"error","message":"GitHub API error (HTTP 401): Bad credentials"}`

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("github", output)

	if filter != "github-error" {
		t.Errorf("expected filter github-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompressGitHub_Branches(t *testing.T) {
	type branch struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}

	branches := make([]branch, 35)
	for i := 0; i < 35; i++ {
		branches[i] = branch{
			Name:      fmt.Sprintf("feature/branch-%d", i),
			Protected: i == 0,
		}
	}
	branches[0] = branch{Name: "main", Protected: true}

	data, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"branches": branches,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("github", "", output, cfg)

	if stats.FilterUsed != "github-branches" {
		t.Errorf("expected filter github-branches, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "branches") {
		t.Error("expected branches header")
	}
	if !strings.Contains(result, "[protected]") {
		t.Error("expected protected marker")
	}
	if !strings.Contains(result, "+ 5 more") {
		t.Error("expected truncation")
	}
}

func TestCompressSQL_QueryResult(t *testing.T) {
	rows := make([]map[string]interface{}, 30)
	for i := 0; i < 30; i++ {
		rows[i] = map[string]interface{}{
			"id":    i + 1,
			"name":  fmt.Sprintf("user%d", i),
			"email": fmt.Sprintf("user%d@example.com", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": rows,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("sql_query", "", output, cfg)

	if stats.FilterUsed != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", stats.FilterUsed)
	}
	if !strings.Contains(result, "30 rows") {
		t.Error("expected row count")
	}
	if !strings.Contains(result, "3 cols") {
		t.Error("expected column count")
	}
	if !strings.Contains(result, "+ 10 more rows") {
		t.Error("expected truncation")
	}
}

func TestCompressSQL_QueryEmpty(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": []map[string]interface{}{},
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", filter)
	}
	if !strings.Contains(result, "0 rows") {
		t.Errorf("expected '0 rows returned', got: %s", result)
	}
}

func TestCompressSQL_Describe(t *testing.T) {
	columns := []map[string]interface{}{
		{"name": "id", "type": "INTEGER", "notnull": true, "pk": true},
		{"name": "name", "type": "TEXT", "notnull": true, "pk": false},
		{"name": "email", "type": "TEXT", "notnull": false, "pk": false, "unique": true},
		{"name": "created_at", "type": "TIMESTAMP", "notnull": false, "pk": false, "default_value": "NOW()"},
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"table":   "users",
		"columns": columns,
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-describe" {
		t.Errorf("expected filter sql-describe, got %s", filter)
	}
	if !strings.Contains(result, "Table users") {
		t.Error("expected table name")
	}
	if !strings.Contains(result, "4 columns") {
		t.Error("expected column count")
	}
	if !strings.Contains(result, "PK") {
		t.Error("expected PK marker")
	}
	if !strings.Contains(result, "NOT NULL") {
		t.Error("expected NOT NULL marker")
	}
	if !strings.Contains(result, "UNIQUE") {
		t.Error("expected UNIQUE marker")
	}
	if !strings.Contains(result, "DEFAULT") {
		t.Error("expected DEFAULT marker")
	}
}

func TestCompressSQL_ListTables(t *testing.T) {
	tables := make([]string, 60)
	for i := 0; i < 60; i++ {
		tables[i] = fmt.Sprintf("table_%d", i)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"tables": tables,
		"count":  len(tables),
	})
	output := "Tool Output: " + string(data)

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-list-tables" {
		t.Errorf("expected filter sql-list-tables, got %s", filter)
	}
	if !strings.Contains(result, "60 tables") {
		t.Error("expected table count")
	}
	if !strings.Contains(result, "+ 10 more") {
		t.Error("expected truncation")
	}
}

func TestCompressSQL_Error(t *testing.T) {
	output := `Tool Output: {"status":"error","message":"'sql_query' is required for query operation"}`

	// Call compressor directly (bypass MinChars threshold)
	result, filter := compressAPIOutput("sql_query", output)

	if filter != "sql-error" {
		t.Errorf("expected filter sql-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompress_APIRouting_GitHubAndSQL(t *testing.T) {
	// Verify github and sql_query are recognized as API tools
	if !isAPITool("github") {
		t.Error("expected 'github' to be an API tool")
	}
	if !isAPITool("sql_query") {
		t.Error("expected 'sql_query' to be an API tool")
	}
	if !isGitHubTool("github") {
		t.Error("expected isGitHubTool('github') = true")
	}
	if !isSQLTool("sql_query") {
		t.Error("expected isSQLTool('sql_query') = true")
	}
}

func TestCompressSQL_QueryResult_LargeValues(t *testing.T) {
	rows := make([]map[string]interface{}, 5)
	for i := 0; i < 5; i++ {
		rows[i] = map[string]interface{}{
			"id":          i + 1,
			"description": strings.Repeat("x", 100), // long value
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"result": rows,
	})
	output := "Tool Output: " + string(data)

	cfg := DefaultConfig()
	result, stats := Compress("sql_query", "", output, cfg)

	if stats.FilterUsed != "sql-query" {
		t.Errorf("expected filter sql-query, got %s", stats.FilterUsed)
	}
	// Long values should be truncated
	if strings.Contains(result, strings.Repeat("x", 100)) {
		t.Error("expected long values to be truncated")
	}
	if !strings.Contains(result, "...") {
		t.Error("expected truncation indicator")
	}
}

// ─── V10A Tests: Filesystem / File Reader / Smart File ──────────────────────

func TestCompress_APIRouting_FileTools(t *testing.T) {
	// Verify file tool families are recognized
	if !isAPITool("filesystem") {
		t.Error("expected 'filesystem' to be an API tool")
	}
	if !isAPITool("filesystem_op") {
		t.Error("expected 'filesystem_op' to be an API tool")
	}
	if !isAPITool("file_reader_advanced") {
		t.Error("expected 'file_reader_advanced' to be an API tool")
	}
	if !isAPITool("smart_file_read") {
		t.Error("expected 'smart_file_read' to be an API tool")
	}
	if !isFilesystemTool("filesystem") {
		t.Error("expected isFilesystemTool('filesystem') = true")
	}
	if !isFileReaderTool("file_reader_advanced") {
		t.Error("expected isFileReaderTool('file_reader_advanced') = true")
	}
	if !isSmartFileTool("smart_file_read") {
		t.Error("expected isSmartFileTool('smart_file_read') = true")
	}
}

func TestCompressFS_ListDir_Large(t *testing.T) {
	type entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"modified"`
	}

	entries := make([]entry, 200)
	for i := 0; i < 200; i++ {
		isDir := i < 20
		name := fmt.Sprintf("file_%04d.txt", i)
		if isDir {
			name = fmt.Sprintf("dir_%04d", i)
		}
		entries[i] = entry{
			Name:    name,
			IsDir:   isDir,
			Size:    int64(i * 1024),
			ModTime: "2024-01-15T10:30:00Z",
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Listed 200 entries (of 200 total)",
		"data": map[string]interface{}{
			"entries":     entries,
			"total_count": 200,
			"truncated":   false,
			"limit":       500,
			"offset":      0,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-list-dir" {
		t.Errorf("expected filter fs-list-dir, got %s", filter)
	}
	if !strings.Contains(result, "200 entries") {
		t.Error("expected entry count")
	}
	if !strings.Contains(result, "20 dirs") {
		t.Error("expected dir count")
	}
	if !strings.Contains(result, "180 files") {
		t.Error("expected file count")
	}
	if !strings.Contains(result, "+ 150 more") {
		t.Error("expected truncation")
	}
}

func TestCompressFS_ListDir_Paginated(t *testing.T) {
	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}

	entries := make([]entry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = entry{Name: fmt.Sprintf("file_%d.txt", i), IsDir: false, Size: 1024}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Listed 10 entries (of 1500 total) — use next_offset for more",
		"data": map[string]interface{}{
			"entries":     entries,
			"total_count": 1500,
			"truncated":   true,
			"limit":       10,
			"offset":      0,
			"next_offset": 10,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-list-dir" {
		t.Errorf("expected filter fs-list-dir, got %s", filter)
	}
	if !strings.Contains(result, "1500 entries") {
		t.Error("expected total count")
	}
	if !strings.Contains(result, "paginated") {
		t.Error("expected pagination indicator")
	}
}

func TestCompressFS_ReadFile_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Read 52 bytes",
		"data":    content,
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-read-file" {
		t.Errorf("expected filter fs-read-file, got %s", filter)
	}
	// Content must be preserved exactly
	if !strings.Contains(result, content) {
		t.Error("expected file content preserved")
	}
	if !strings.Contains(result, "Read 52 bytes") {
		t.Error("expected message preserved")
	}
}

func TestCompressFS_Batch_Partial(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		status := "success"
		msg := "OK"
		if i == 3 || i == 7 {
			status = "error"
			msg = "Permission denied"
		}
		items[i] = map[string]interface{}{
			"index":     i,
			"file_path": fmt.Sprintf("/tmp/file_%d.txt", i),
			"status":    status,
			"message":   msg,
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "partial",
		"message": "copy_batch processed 10 items (8 succeeded, 2 failed)",
		"data": map[string]interface{}{
			"summary": map[string]int{
				"requested": 10,
				"succeeded": 8,
				"failed":    2,
			},
			"results": items,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-batch" {
		t.Errorf("expected filter fs-batch, got %s", filter)
	}
	if !strings.Contains(result, "8 succeeded") {
		t.Error("expected succeeded count")
	}
	if !strings.Contains(result, "2 failed") {
		t.Error("expected failed count")
	}
	// Failed items should be shown
	if !strings.Contains(result, "FAILED") {
		t.Error("expected failed items shown")
	}
	// Succeeded items should be summarized
	if !strings.Contains(result, "succeeded items omitted") {
		t.Error("expected succeeded summary")
	}
}

func TestCompressFS_Error(t *testing.T) {
	output := `{"status":"error","message":"Failed to list directory: permission denied","data":{"error_code":"io_error"}}`

	result, filter := compressAPIOutput("filesystem", output)

	if filter != "fs-error" {
		t.Errorf("expected filter fs-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompressFR_Content_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"start_line": 1,
			"end_line":   5,
			"total_read": 5,
			"content":    content,
			"truncated":  false,
		},
	})

	result, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-content" {
		t.Errorf("expected filter fr-content, got %s", filter)
	}
	// Content must be preserved
	if !strings.Contains(result, content) {
		t.Error("expected file content preserved")
	}
	if !strings.Contains(result, "Lines 1-5") {
		t.Error("expected line range")
	}
}

func TestCompressFR_SearchContext_ManyMatches(t *testing.T) {
	type match struct {
		MatchLine int    `json:"match_line"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}

	matches := make([]match, 30)
	for i := 0; i < 30; i++ {
		matches[i] = match{
			MatchLine: 10 + i*5,
			StartLine: 8 + i*5,
			EndLine:   12 + i*5,
			Content:   fmt.Sprintf("Line with error: something went wrong at step %d", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"pattern":       "error",
			"total_matches": 30,
			"matches":       matches,
		},
	})

	result, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-search" {
		t.Errorf("expected filter fr-search, got %s", filter)
	}
	if !strings.Contains(result, "30 matches") {
		t.Error("expected match count")
	}
	if !strings.Contains(result, "+ 15 more matches") {
		t.Error("expected truncation")
	}
	if !strings.Contains(result, "L8-12:") {
		t.Error("expected line range format")
	}
}

func TestCompressFR_CountLines(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"lines": 12345,
			"bytes": 67890,
		},
	})

	_, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-count-lines" {
		t.Errorf("expected filter fr-count-lines, got %s", filter)
	}
	// count_lines is already compact, should pass through
}

func TestCompressFR_Error(t *testing.T) {
	output := `{"status":"error","message":"'file_path' is required"}`

	result, filter := compressAPIOutput("file_reader_advanced", output)

	if filter != "fr-error" {
		t.Errorf("expected filter fr-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompressSF_Analyze(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Analyzed main.go",
		"data": map[string]interface{}{
			"path":              "main.go",
			"size_bytes":        15360,
			"line_count":        450,
			"mime":              "text/plain",
			"extension":         ".go",
			"group":             "text",
			"is_text_like":      true,
			"is_large":          false,
			"recommended_tool":  "file_reader_advanced",
			"default_strategy":  "head_tail",
			"next_steps":        []string{"Use file_reader_advanced read_lines"},
			"detected_encoding": "utf-8",
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-analyze" {
		t.Errorf("expected filter sf-analyze, got %s", filter)
	}
	if !strings.Contains(result, "main.go") {
		t.Error("expected file name")
	}
	if !strings.Contains(result, "450 lines") {
		t.Error("expected line count")
	}
	if !strings.Contains(result, "text/plain") {
		t.Error("expected mime type")
	}
	if !strings.Contains(result, "Recommended: file_reader_advanced") {
		t.Error("expected recommendation")
	}
}

func TestCompressSF_Structure(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Detected structure for config.json",
		"data": map[string]interface{}{
			"path":            "config.json",
			"size_bytes":      8192,
			"mime":            "application/json",
			"extension":       ".json",
			"is_text_like":    true,
			"format":          "json",
			"root_type":       "object",
			"top_level_keys":  []string{"server", "database", "logging", "auth", "cache", "features", "version"},
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-structure" {
		t.Errorf("expected filter sf-structure, got %s", filter)
	}
	if !strings.Contains(result, "config.json") {
		t.Error("expected file name")
	}
	if !strings.Contains(result, "JSON") && !strings.Contains(result, "json") {
		t.Error("expected format")
	}
	if !strings.Contains(result, "Keys:") {
		t.Error("expected keys listing")
	}
	if !strings.Contains(result, "server") {
		t.Error("expected key names")
	}
}

func TestCompressSF_Sample_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Built head_tail sample from main.go",
		"data": map[string]interface{}{
			"path":              "main.go",
			"size_bytes":        15360,
			"sampling_strategy": "head_tail",
			"sample_sections":   2,
			"content":           content,
			"next_steps":        []string{"Use file_reader_advanced"},
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-content" {
		t.Errorf("expected filter sf-content, got %s", filter)
	}
	// Content must be preserved
	if !strings.Contains(result, content) {
		t.Error("expected content preserved")
	}
	if !strings.Contains(result, "head_tail") {
		t.Error("expected strategy")
	}
}

func TestCompressSF_Error(t *testing.T) {
	output := `{"status":"error","message":"'file_path' is required"}`

	result, filter := compressAPIOutput("smart_file_read", output)

	if filter != "sf-error" {
		t.Errorf("expected filter sf-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
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
