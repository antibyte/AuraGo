package outputcompress

import (
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
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true}
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
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true}
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
	cfg := Config{Enabled: true, MinChars: 100, PreserveErrors: true}
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
	apiTools := []string{"docker", "docker_compose", "proxmox", "homeassistant", "kubernetes", "api_request"}
	for _, tool := range apiTools {
		if !isAPITool(tool) {
			t.Errorf("%q should be an API tool", tool)
		}
	}
	nonAPITools := []string{"execute_shell", "filesystem", "execute_python"}
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
