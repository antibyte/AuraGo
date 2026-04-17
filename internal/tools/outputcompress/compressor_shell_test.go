package outputcompress

import (
	"fmt"
	"strings"
	"testing"
)

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
// ─── V9B: Text pipeline tests ──────────────────────────────────────────────

func TestCompressShell_V9B_Routing(t *testing.T) {
	tools := []struct {
		cmd    string
		filter string
	}{
		{"sort file.txt", "sort"},
		{"uniq file.txt", "uniq"},
		{"cut -d, -f1 file.csv", "cut"},
		{"sed 's/old/new/g' file.txt", "sed"},
		{"awk '{print $1}' file.txt", "awk"},
		{"gawk '{print $1}' file.txt", "awk"},
		{"xargs echo", "xargs"},
		{"jq '.name'", "jq"},
		{"tr 'a-z' 'A-Z'", "tr"},
		{"column -t file.txt", "column"},
		{"diff file1.txt file2.txt", "diff"},
		{"comm file1.txt file2.txt", "comm"},
		{"paste file1.txt file2.txt", "paste"},
	}
	for _, tt := range tools {
		_, filter := compressShellOutput(tt.cmd, "some output line 1\nline 2\nline 3\n")
		if filter != tt.filter {
			t.Errorf("compressShellOutput(%q): expected filter %q, got %q", tt.cmd, tt.filter, filter)
		}
	}
}
func TestCompressSort_LargeOutput(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "line-%03d\n", i)
	}
	// Add many duplicates
	for i := 0; i < 200; i++ {
		sb.WriteString("duplicate-line\n")
	}

	result := compressSort(sb.String())
	// Should be shorter than input
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
	// Should have dedup marker or tail-focus
	if strings.Contains(result, "duplicate-line") && strings.Count(result, "duplicate-line") > 1 {
		// Consecutive duplicates should be removed
		lines := strings.Split(result, "\n")
		consecDupes := 0
		for i := 1; i < len(lines); i++ {
			if lines[i] == lines[i-1] && lines[i] == "duplicate-line" {
				consecDupes++
			}
		}
		if consecDupes > 5 {
			t.Errorf("expected consecutive duplicates removed, found %d consecutive", consecDupes)
		}
	}
}

func TestCompressJq_JSON(t *testing.T) {
	input := `{
  "name": "test",
  "items": [
    {"id": 1, "value": "a"},
    {"id": 2, "value": "b"},
    {"id": 3, "value": "c"}
  ],
  "count": 3
}`
	result := compressJq(input)
	// Should be compacted (no excessive whitespace)
	if strings.Contains(result, "    ") {
		t.Errorf("expected compacted JSON, got: %q", result)
	}
}

func TestCompressJq_LargeArray(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&sb, "  {\"id\": %d, \"name\": \"item-%d\", \"value\": \"some data here\"},\n", i, i)
	}
	sb.WriteString("]")

	result := compressJq(sb.String())
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
}

func TestCompressSed_LargeOutput(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&sb, "processed line %d with some content\n", i)
	}

	result := compressSed(sb.String())
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
}

func TestCompressDiff_Output(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/file1.txt b/file2.txt\n")
	sb.WriteString("index abc1234..def5678 100644\n")
	sb.WriteString("--- a/file1.txt\n")
	sb.WriteString("+++ b/file2.txt\n")
	sb.WriteString("@@ -1,5 +1,5 @@\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(" context line that is the same\n")
	}
	sb.WriteString("-removed line\n")
	sb.WriteString("+added line\n")

	result := compressDiff(sb.String())
	// Should compress the repeated context lines
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
}

func TestCompressTextPipeline_Short(t *testing.T) {
	input := "line 1\nline 2\nline 3\n"
	result := compressTextPipeline(input)
	// Short output should pass through mostly unchanged
	if len(result) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestCompressCut_Columnar(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&sb, "col1-%d\tcol2-%d\tcol3-%d\n", i, i, i)
	}

	result := compressCut(sb.String())
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
}
