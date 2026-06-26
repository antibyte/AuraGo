package chunking

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRecursiveChunkerKeepsMarkdownCodeFencesAndTablesTogether(t *testing.T) {
	text := strings.Join([]string{
		"# Overview",
		"",
		"Short intro paragraph that explains the document.",
		"",
		"```go",
		"func main() {",
		`	println("hello")`,
		"}",
		"```",
		"",
		"| Name | Purpose |",
		"| --- | --- |",
		"| AuraGo | Home lab agent |",
		"",
		"## Next",
		"",
		strings.Repeat("Follow up sentence. ", 20),
	}, "\n")

	chunks, err := ChunkText(text, Options{
		Strategy:     StrategyRecursive,
		MaxChars:     160,
		OverlapChars: 0,
		MaxChunks:    20,
	})
	if err != nil {
		t.Fatalf("ChunkText error: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("len(chunks) = %d, want at least 3", len(chunks))
	}

	var sawFence, sawTable bool
	for _, chunk := range chunks {
		if strings.Count(chunk.Text, "```") == 1 {
			t.Fatalf("chunk contains an unbalanced code fence:\n%s", chunk.Text)
		}
		if strings.Contains(chunk.Text, "func main()") {
			sawFence = true
			if !strings.Contains(chunk.Text, "```go") || !strings.Contains(chunk.Text, "```") {
				t.Fatalf("code chunk missing fence markers:\n%s", chunk.Text)
			}
		}
		if strings.Contains(chunk.Text, "| AuraGo | Home lab agent |") {
			sawTable = true
			if !strings.Contains(chunk.Text, "| Name | Purpose |") {
				t.Fatalf("table row split away from header:\n%s", chunk.Text)
			}
		}
	}
	if !sawFence {
		t.Fatal("expected a chunk containing the fenced code block")
	}
	if !sawTable {
		t.Fatal("expected a chunk containing the markdown table")
	}
}

func TestRecursiveChunkerIsUTF8SafeAndCapsChunks(t *testing.T) {
	text := strings.Repeat("Ein schöner Satz mit Umlauten äöü und Kontext. ", 20)
	chunks, err := ChunkText(text, Options{
		Strategy:     StrategyRecursive,
		MaxChars:     80,
		OverlapChars: 12,
		MaxChunks:    3,
	})
	if err != nil {
		t.Fatalf("ChunkText error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("len(chunks) = %d, want capped 3", len(chunks))
	}
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Fatalf("chunk[%d].Index = %d, want %d", i, chunk.Index, i)
		}
		if chunk.Total != 3 {
			t.Fatalf("chunk[%d].Total = %d, want 3", i, chunk.Total)
		}
		if !utf8.ValidString(chunk.Text) {
			t.Fatalf("chunk[%d] is not valid UTF-8: %q", i, chunk.Text)
		}
	}
}

func TestRecursiveChunkerOverlapDoesNotDropContent(t *testing.T) {
	text := strings.Repeat("a", 100) + strings.Repeat("b", 100) + strings.Repeat("c", 100)
	chunks, err := ChunkText(text, Options{
		Strategy:     StrategyRecursive,
		MaxChars:     100,
		OverlapChars: 10,
		MaxChunks:    10,
	})
	if err != nil {
		t.Fatalf("ChunkText error: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("len(chunks) = %d, want at least 3", len(chunks))
	}

	var reconstructed strings.Builder
	for i, chunk := range chunks {
		if got := runeLen(chunk.Text); got > 100 {
			t.Fatalf("chunk[%d] length = %d, want <= 100", i, got)
		}

		textPart := chunk.Text
		if i > 0 {
			overlap := tailRunes(chunks[i-1].Text, 10)
			if strings.HasPrefix(textPart, overlap) {
				textPart = strings.TrimPrefix(textPart, overlap)
				textPart = strings.TrimPrefix(textPart, "\n")
			}
		}
		reconstructed.WriteString(textPart)
	}
	if got := reconstructed.String(); got != text {
		t.Fatalf("reconstructed text lost content: got len=%d want len=%d", len(got), len(text))
	}
}

func TestLegacyChunkerPreservesExistingParagraphSplitBehavior(t *testing.T) {
	text := strings.Repeat("a", 300) + "\n\n" + strings.Repeat("b", 300)
	chunks, err := ChunkText(text, Options{
		Strategy:     StrategyLegacy,
		MaxChars:     400,
		OverlapChars: 0,
	})
	if err != nil {
		t.Fatalf("ChunkText error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want at least 2", len(chunks))
	}
	if strings.Contains(chunks[0].Text, strings.Repeat("b", 20)) {
		t.Fatalf("first chunk crossed paragraph boundary unexpectedly: %q", chunks[0].Text)
	}
}
