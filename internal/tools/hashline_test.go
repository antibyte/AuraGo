package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashLineContentIsDeterministicContentOnlyAndEightHexChars(t *testing.T) {
	first := hashLineContent("func main() {")
	second := hashLineContent("func main() {")
	different := hashLineContent("func main() {}")

	if first != second {
		t.Fatalf("hashLineContent not deterministic: %q vs %q", first, second)
	}
	if len(first) != 8 {
		t.Fatalf("hashLineContent length = %d, want 8: %q", len(first), first)
	}
	if first == different {
		t.Fatalf("different content produced the same hash: %q", first)
	}

	entries := buildHashlineEntries([]byte("inserted\nfunc main() {"))
	if entries[1].Hash != first {
		t.Fatalf("hash changed when the same content moved lines: got %q want %q", entries[1].Hash, first)
	}
}

func TestBuildAndFormatHashlineEntriesPreservesLineDetails(t *testing.T) {
	entries := buildHashlineEntries([]byte("alpha: beta\nomega\n"))
	if len(entries) != 3 {
		t.Fatalf("entries length = %d, want 3", len(entries))
	}
	if entries[0].LineNum != 1 || entries[0].Content != "alpha: beta" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[2].LineNum != 3 || entries[2].Content != "" {
		t.Fatalf("trailing newline entry = %#v, want empty third line", entries[2])
	}

	output := formatHashlineOutput(entries)
	if !strings.Contains(output, "1#"+entries[0].Hash+":alpha: beta\n") {
		t.Fatalf("formatted output missing first line with colon content: %q", output)
	}
	if !strings.HasSuffix(output, "3#"+entries[2].Hash+":\n") {
		t.Fatalf("formatted output missing trailing empty line: %q", output)
	}
}

func TestValidateHashlineAnchorRejectsMissingAndStaleAnchors(t *testing.T) {
	entries := buildHashlineEntries([]byte("line1\nline2"))
	if err := validateHashlineAnchor(entries, 2, entries[1].Hash); err != nil {
		t.Fatalf("valid anchor rejected: %v", err)
	}

	if err := validateHashlineAnchor(entries, 0, entries[0].Hash); err == nil || !strings.Contains(err.Error(), "anchor_line") {
		t.Fatalf("missing anchor error = %v, want anchor_line guidance", err)
	}
	if err := validateHashlineAnchor(entries, 2, "00000000"); err == nil || !strings.Contains(err.Error(), "STALE CONTEXT") {
		t.Fatalf("stale anchor error = %v, want STALE CONTEXT", err)
	}
}

func TestExecuteFilesystemReadFileWithHashesReturnsStructuredHashlineData(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "notes.txt"), []byte("alpha\nbeta: gamma\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	raw := ExecuteFilesystemWithOptions("read_file", "notes.txt", "", "", nil, workdir, 0, 0, FilesystemOptions{IncludeHashes: true})
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v -- raw: %s", err, raw)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success: %s", result.Status, result.Message)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map", result.Data)
	}
	if data["format"] != "hashline" {
		t.Fatalf("format = %v, want hashline", data["format"])
	}
	content, _ := data["content"].(string)
	if !strings.Contains(content, "1#") || !strings.Contains(content, ":alpha\n") || !strings.Contains(content, ":beta: gamma\n") {
		t.Fatalf("hashline content malformed: %q", content)
	}
	if data["truncated"] != false {
		t.Fatalf("truncated = %v, want false", data["truncated"])
	}
	if data["lines_returned"].(float64) != 3 {
		t.Fatalf("lines_returned = %v, want 3", data["lines_returned"])
	}
}

func TestExecuteFilesystemReadFileWithHashesRejectsBinaryContent(t *testing.T) {
	workdir := t.TempDir()
	data := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 32)...)
	if err := os.WriteFile(filepath.Join(workdir, "image.png"), data, 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	raw := ExecuteFilesystemWithOptions("read_file", "image.png", "", "", nil, workdir, 0, 0, FilesystemOptions{IncludeHashes: true})
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v -- raw: %s", err, raw)
	}
	if result.Status != "error" || !strings.Contains(result.Message, "binary file") {
		t.Fatalf("result = %#v, want binary error", result)
	}
}

func TestExecuteFilesystemReadFileWithHashesDoesNotCutOutputLineMidway(t *testing.T) {
	workdir := t.TempDir()
	content := strings.Repeat("0123456789abcdef", 3000)
	if err := os.WriteFile(filepath.Join(workdir, "large.log"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	raw := ExecuteFilesystemWithOptions("read_file", "large.log", "", "", nil, workdir, 0, 0, FilesystemOptions{IncludeHashes: true})
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v -- raw: %s", err, raw)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success: %s", result.Status, result.Message)
	}
	data := result.Data.(map[string]interface{})
	if data["truncated"] != true {
		t.Fatalf("truncated = %v, want true", data["truncated"])
	}
	hashlineContent := data["content"].(string)
	if hashlineContent != "" && !strings.HasSuffix(hashlineContent, "\n") {
		t.Fatalf("hashline content should end at a line boundary, got suffix %q", hashlineContent[len(hashlineContent)-16:])
	}
}

func TestExecuteFilesystemReadFileWithHashesDoesNotEmitSyntheticTrailingLineWhenByteTruncated(t *testing.T) {
	workdir := t.TempDir()
	content := "complete\n" + strings.Repeat("x", 48*1024)
	if err := os.WriteFile(filepath.Join(workdir, "large.log"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	raw := ExecuteFilesystemWithOptions("read_file", "large.log", "", "", nil, workdir, 0, 0, FilesystemOptions{IncludeHashes: true})
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v -- raw: %s", err, raw)
	}
	data := result.Data.(map[string]interface{})
	if data["lines_returned"].(float64) != 1 {
		t.Fatalf("lines_returned = %v, want only the one complete line before truncation; content=%q", data["lines_returned"], data["content"])
	}
}

func TestExecuteHashlineEditorReplaceTargetsValidatedAnchorLine(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "target\nother\ntarget\n")
	entries := buildHashlineEntries([]byte("target\nother\ntarget\n"))

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_replace",
		FilePath:   fname,
		Old:        "target",
		New:        "changed",
		AnchorLine: 3,
		AnchorHash: entries[2].Hash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if string(data) != "target\nother\nchanged\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

func TestExecuteHashlineEditorReplaceRejectsStaleMissingAndMisplacedOld(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "alpha\nbeta\ngamma\n")
	entries := buildHashlineEntries([]byte("alpha\nbeta\ngamma\n"))

	cases := []struct {
		name string
		req  HashlineEditorRequest
		want string
	}{
		{
			name: "missing anchor",
			req: HashlineEditorRequest{
				Operation: "hashline_replace",
				FilePath:  fname,
				Old:       "beta",
				New:       "changed",
			},
			want: "anchor_line",
		},
		{
			name: "stale anchor",
			req: HashlineEditorRequest{
				Operation:  "hashline_replace",
				FilePath:   fname,
				Old:        "beta",
				New:        "changed",
				AnchorLine: 2,
				AnchorHash: "00000000",
			},
			want: "STALE CONTEXT",
		},
		{
			name: "old not at anchor",
			req: HashlineEditorRequest{
				Operation:  "hashline_replace",
				FilePath:   fname,
				Old:        "gamma",
				New:        "changed",
				AnchorLine: 2,
				AnchorHash: entries[1].Hash,
			},
			want: "anchor line",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := decodeEditorResult(t, ExecuteHashlineEditor(tc.req, wsDir))
			if res.Status != "error" || !strings.Contains(res.Message, tc.want) {
				t.Fatalf("result = %#v, want error containing %q", res, tc.want)
			}
		})
	}
}

func TestExecuteHashlineEditorReplaceSupportsMultilineOldAtAnchor(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "before\nstart\nmiddle\nend\nafter\n")
	entries := buildHashlineEntries([]byte("before\nstart\nmiddle\nend\nafter\n"))

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_replace",
		FilePath:   fname,
		Old:        "start\nmiddle\nend",
		New:        "replacement",
		AnchorLine: 2,
		AnchorHash: entries[1].Hash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if string(data) != "before\nreplacement\nafter\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

func TestExecuteHashlineEditorReplaceRejectsMultipleMatchesOnAnchorLine(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "dup dup\nother\n")
	entries := buildHashlineEntries([]byte("dup dup\nother\n"))

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_replace",
		FilePath:   fname,
		Old:        "dup",
		New:        "changed",
		AnchorLine: 1,
		AnchorHash: entries[0].Hash,
	}, wsDir))
	if res.Status != "error" || !strings.Contains(res.Message, "anchor line") {
		t.Fatalf("result = %#v, want anchor-line ambiguity error", res)
	}
}

func TestExecuteHashlineEditorInsertTargetsValidatedAnchorLine(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "marker\nother marker\n")
	entries := buildHashlineEntries([]byte("marker\nother marker\n"))

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_insert_after",
		FilePath:   fname,
		Marker:     "marker",
		Content:    "inserted",
		AnchorLine: 1,
		AnchorHash: entries[0].Hash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if string(data) != "marker\ninserted\nother marker\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

func TestExecuteHashlineEditorInsertRejectsStaleAndMarkerMismatch(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "marker\nother\n")
	entries := buildHashlineEntries([]byte("marker\nother\n"))

	for _, tc := range []struct {
		name string
		req  HashlineEditorRequest
		want string
	}{
		{
			name: "stale",
			req: HashlineEditorRequest{
				Operation:  "hashline_insert_before",
				FilePath:   fname,
				Marker:     "marker",
				Content:    "inserted",
				AnchorLine: 1,
				AnchorHash: "00000000",
			},
			want: "STALE CONTEXT",
		},
		{
			name: "marker mismatch",
			req: HashlineEditorRequest{
				Operation:  "hashline_insert_before",
				FilePath:   fname,
				Marker:     "marker",
				Content:    "inserted",
				AnchorLine: 2,
				AnchorHash: entries[1].Hash,
			},
			want: "Marker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := decodeEditorResult(t, ExecuteHashlineEditor(tc.req, wsDir))
			if res.Status != "error" || !strings.Contains(res.Message, tc.want) {
				t.Fatalf("result = %#v, want error containing %q", res, tc.want)
			}
		})
	}
}

func TestExecuteHashlineEditorDeleteRequiresAnchorInsideRange(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "one\ntwo\nthree\nfour\n")
	entries := buildHashlineEntries([]byte("one\ntwo\nthree\nfour\n"))

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_delete",
		FilePath:   fname,
		StartLine:  2,
		EndLine:    3,
		AnchorLine: 2,
		AnchorHash: entries[1].Hash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if string(data) != "one\nfour\n" {
		t.Fatalf("file content = %q", string(data))
	}

	entries = buildHashlineEntries(data)
	res = decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_delete",
		FilePath:   fname,
		StartLine:  1,
		EndLine:    1,
		AnchorLine: 2,
		AnchorHash: entries[1].Hash,
	}, wsDir))
	if res.Status != "error" || !strings.Contains(res.Message, "within the delete range") {
		t.Fatalf("result = %#v, want anchor range error", res)
	}
}

func TestExecuteHashlineEditorMultiEditAllowsAdjustedLineWithOriginalContentHash(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "one\ntwo\nthree\n")
	entries := buildHashlineEntries([]byte("one\ntwo\nthree\n"))
	lineThreeHash := entries[2].Hash

	res := decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_insert_after",
		FilePath:   fname,
		Marker:     "one",
		Content:    "inserted",
		AnchorLine: 1,
		AnchorHash: entries[0].Hash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("insert failed: %s: %s", res.Status, res.Message)
	}

	res = decodeEditorResult(t, ExecuteHashlineEditor(HashlineEditorRequest{
		Operation:  "hashline_replace",
		FilePath:   fname,
		Old:        "three",
		New:        "changed",
		AnchorLine: 4,
		AnchorHash: lineThreeHash,
	}, wsDir))
	if res.Status != "success" {
		t.Fatalf("replace failed after adjusted line number: %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if string(data) != "one\ninserted\ntwo\nchanged\n" {
		t.Fatalf("file content = %q", string(data))
	}
}
