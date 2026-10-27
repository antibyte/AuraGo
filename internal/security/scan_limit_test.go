package security

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// The regex tier reads tens of KiB per second, so text beyond maxScanOutputBytes
// is never scanned: external output is withheld as incomplete (no body, no
// warning line), source output takes the escaped form, and the advisory
// ScanExternalContent skips the scan. All of that must return at once; a scan of
// this much text takes many seconds.
func TestSanitizeToolOutputBoundsScanWork(t *testing.T) {
	var logs bytes.Buffer
	g := NewGuardian(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	line := "a quiet line of ordinary log output\n"
	big := strings.Repeat(line, maxScanOutputBytes/len(line)+1)
	if len(big) <= maxScanOutputBytes {
		t.Fatal("test body must exceed the scan limit")
	}
	envelope := `{"status":"success","content":"` + strings.ReplaceAll(big, "\n", `\n`) + `"}`

	start := time.Now()
	for _, tc := range []struct{ tool, output string }{
		{"execute_shell", big},
		{"web_scraper", envelope},
		{"unknown_tool", big},
	} {
		got := g.SanitizeToolOutput(tc.tool, tc.output)
		if !strings.HasPrefix(got, "[QUARANTINE NOTICE]\n") || !strings.Contains(got, "incomplete or exceeded its size limit") {
			t.Fatalf("%s: oversized external output was not withheld as incomplete: %.200q", tc.tool, got)
		}
		if strings.Contains(got, "quiet line") || strings.Contains(got, "SECURITY WARNING") || len(got) > 1000 {
			t.Fatalf("%s: withheld output kept a body or warning: %.200q", tc.tool, got)
		}
	}

	source := `{"status":"ok","content":"const s = 'it\'s';\n` + strings.Repeat("if (a < b) { go(); }\n", maxScanOutputBytes/20) + `"}`
	if got := g.SanitizeToolOutput("game_maker_file", source); got != IsolateExternalData(source) {
		t.Fatalf("oversized source kept the copyable form: %.200q", got)
	}

	if got := g.ScanExternalContent("https://example.test/", big); got != IsolateExternalData(big) {
		t.Fatal("oversized content was not isolated")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("bounded paths took %v: something scanned the oversized text", elapsed)
	}
	for _, want := range []string{"beyond the scan limit", "not scanned"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log lacks %q: %s", want, logs.String())
		}
	}

	// A body within the limit keeps its isolated form and no warning.
	small := strings.Repeat(line, 4096/len(line))
	if got := g.SanitizeToolOutput("execute_shell", small); got != IsolateExternalData(small) {
		t.Fatalf("a small body changed: %.200q", got)
	}
}
