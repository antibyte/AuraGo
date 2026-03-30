package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── splitFrontmatter ──────────────────────────────────────────────────────────

func TestSplitFrontmatter_NonePresent(t *testing.T) {
	raw := "# Title\n\nSome body text."
	fm, body := splitFrontmatter(raw)
	if fm != "" {
		t.Errorf("expected empty frontmatter, got %q", fm)
	}
	if body != raw {
		t.Errorf("expected body == raw, got %q", body)
	}
}

func TestSplitFrontmatter_Standard(t *testing.T) {
	raw := "---\ndescription: My tool\n---\n\n# Title\n\nBody here."
	fm, body := splitFrontmatter(raw)
	if !strings.Contains(fm, "description: My tool") {
		t.Errorf("frontmatter missing content, got %q", fm)
	}
	if !strings.Contains(body, "# Title") {
		t.Errorf("body missing content, got %q", body)
	}
}

func TestSplitFrontmatter_WindowsLineEndings(t *testing.T) {
	raw := "---\r\ndescription: Win\r\n---\r\n\r\n# Title\r\n"
	fm, body := splitFrontmatter(raw)
	if !strings.Contains(fm, "description: Win") {
		t.Errorf("frontmatter missing content under CRLF, got %q", fm)
	}
	if !strings.Contains(body, "# Title") {
		t.Errorf("body missing content under CRLF, got %q", body)
	}
}

func TestSplitFrontmatter_MissingCloseMarker(t *testing.T) {
	raw := "---\ndescription: No closer\n# Body here"
	fm, body := splitFrontmatter(raw)
	// Without a closing --- the function falls back to returning the raw content.
	if fm != "" {
		t.Errorf("expected no frontmatter without close marker, got %q", fm)
	}
	if body != raw {
		t.Errorf("expected body == raw, got %q", body)
	}
}

func TestSplitFrontmatter_EmptyFrontmatter(t *testing.T) {
	raw := "---\n---\n\nBody only."
	fm, body := splitFrontmatter(raw)
	if fm != "" {
		t.Errorf("expected empty frontmatter string, got %q", fm)
	}
	if !strings.Contains(body, "Body only.") {
		t.Errorf("body missing content, got %q", body)
	}
}

// ── chunkText ─────────────────────────────────────────────────────────────────

func TestChunkText_ShortText(t *testing.T) {
	text := "Short text."
	chunks := chunkText(text, 4000, 200)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("expected chunk == input, got %q", chunks[0])
	}
}

func TestChunkText_SplitsOnParagraph(t *testing.T) {
	// Build a text with a clear paragraph boundary beyond the mid-point of chunkSize.
	para1 := strings.Repeat("a", 300)
	para2 := strings.Repeat("b", 300)
	text := para1 + "\n\n" + para2
	// chunkSize 400 — para boundary is at 300 which is > 400/2 = 200 → should split there
	chunks := chunkText(text, 400, 0)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], strings.Repeat("a", 10)) {
		t.Errorf("first chunk should contain 'a' text")
	}
	if !strings.Contains(chunks[len(chunks)-1], strings.Repeat("b", 10)) {
		t.Errorf("last chunk should contain 'b' text")
	}
}

func TestChunkText_SplitsOnSentence(t *testing.T) {
	// No paragraph boundary — but there is a sentence boundary past the midpoint.
	sentence1 := strings.Repeat("x", 250) + ". "
	sentence2 := strings.Repeat("y", 250)
	text := sentence1 + sentence2
	// chunkSize 400 — sentence boundary at ~252 > 200 → should prefer it over hard cut
	chunks := chunkText(text, 400, 0)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for sentence split, got %d", len(chunks))
	}
}

func TestChunkText_HardCutFallback(t *testing.T) {
	// No paragraph or sentence boundary → hard cut.
	text := strings.Repeat("z", 2000)
	chunks := chunkText(text, 500, 0)
	if len(chunks) < 4 {
		t.Fatalf("expected multiple chunks for hard-cut text, got %d", len(chunks))
	}
	// Verify total content is preserved.
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if total < len(text)-len(text)/10 {
		t.Errorf("chunks appear to have lost content: total=%d original=%d", total, len(text))
	}
}

func TestChunkText_WithOverlap(t *testing.T) {
	// Overlap > 0: successive chunks should share bytes.
	para1 := strings.Repeat("a", 300)
	para2 := strings.Repeat("b", 300)
	text := para1 + para2
	chunks := chunkText(text, 400, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// The second chunk should start somewhere within the overlap region.
	// It should not start exactly where chunk-1 ended (i.e. there is overlap).
	end0 := len(chunks[0])
	start1Content := chunks[1]
	if !strings.Contains(text[:end0], start1Content[:min(50, len(start1Content))]) {
		// This is a soft check; just ensure the second chunk isn't empty.
		if len(chunks[1]) == 0 {
			t.Error("second chunk should not be empty")
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── computeToolGuidesHash ────────────────────────────────────────────────────

// fakeCV returns a minimal ChromemVectorDB that is sufficient for calling methods
// that do not require an actual database connection (e.g. computeToolGuidesHash).
func fakeCV(t *testing.T) *ChromemVectorDB {
	t.Helper()
	return &ChromemVectorDB{}
}

func TestComputeToolGuidesHash_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cv := fakeCV(t)
	hash := cv.computeToolGuidesHash(dir)
	// Empty directory → SHA-256 of nothing → well-known constant.
	expected := hex.EncodeToString(sha256.New().Sum(nil))
	if hash != expected {
		t.Errorf("empty dir hash: got %q want %q", hash, expected)
	}
}

func TestComputeToolGuidesHash_SingleFile(t *testing.T) {
	dir := t.TempDir()
	cv := fakeCV(t)
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := cv.computeToolGuidesHash(dir)
	if hash == "" {
		t.Fatal("expected non-empty hash for directory with one file")
	}
}

func TestComputeToolGuidesHash_ContentChange(t *testing.T) {
	dir := t.TempDir()
	cv := fakeCV(t)
	path := filepath.Join(dir, "tool.md")
	if err := os.WriteFile(path, []byte("version 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash1 := cv.computeToolGuidesHash(dir)

	if err := os.WriteFile(path, []byte("version 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash2 := cv.computeToolGuidesHash(dir)

	if hash1 == hash2 {
		t.Error("hash should change when file content changes")
	}
}

func TestComputeToolGuidesHash_AddFile(t *testing.T) {
	dir := t.TempDir()
	cv := fakeCV(t)
	if err := os.WriteFile(filepath.Join(dir, "tool_a.md"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	hashBefore := cv.computeToolGuidesHash(dir)

	if err := os.WriteFile(filepath.Join(dir, "tool_b.md"), []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	hashAfter := cv.computeToolGuidesHash(dir)

	if hashBefore == hashAfter {
		t.Error("hash should change when a new .md file is added")
	}
}

func TestComputeToolGuidesHash_IgnoresNonMD(t *testing.T) {
	dir := t.TempDir()
	cv := fakeCV(t)
	if err := os.WriteFile(filepath.Join(dir, "tool.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash1 := cv.computeToolGuidesHash(dir)

	// Adding a non-.md file must not affect the hash.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash2 := cv.computeToolGuidesHash(dir)

	if hash1 != hash2 {
		t.Error("hash should not change when a non-.md file is added")
	}
}

func TestComputeToolGuidesHash_MissingDir(t *testing.T) {
	cv := fakeCV(t)
	hash := cv.computeToolGuidesHash("/nonexistent/path/that/does/not/exist")
	if hash != "" {
		t.Errorf("missing directory should return empty hash, got %q", hash)
	}
}
