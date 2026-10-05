package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
)

// c102DecodeResult decodes a document_creator result and fails the test when it is not valid JSON.
func c102DecodeResult(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return out
}

func c102MarotoConfig(outputDir string) *config.DocumentCreatorConfig {
	return &config.DocumentCreatorConfig{Enabled: true, Backend: "maroto", OutputDir: outputDir}
}

func TestDocumentCreatorErrorKeepsQuotesAndBackslashesValid(t *testing.T) {
	const message = `C:\data\x "y"`
	out := c102DecodeResult(t, documentCreatorError(message))
	if out["status"] != "error" || out["message"] != message {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDocumentCreatorUnknownOperationIsValidAndBounded(t *testing.T) {
	operation := `C:\data\x "y"` + strings.Repeat("ä", 500)
	out := c102DecodeResult(t, ExecuteDocumentCreator(context.Background(), c102MarotoConfig(t.TempDir()), operation, "", "", "", "", "", false, "", ""))
	message, _ := out["message"].(string)
	if out["status"] != "error" || !strings.HasPrefix(message, `unknown operation: C:\data\x "y"`) {
		t.Fatalf("decoded = %+v", out)
	}
	echoed := strings.TrimPrefix(message, "unknown operation: ")
	echoed = echoed[:strings.Index(echoed, ". Valid:")]
	if got := utf8.RuneCountInString(echoed); got > maxEchoedDocumentOperationRunes+1 {
		t.Fatalf("echoed operation has %d runes, want at most %d plus the ellipsis", got, maxEchoedDocumentOperationRunes)
	}
}

func TestDocumentCreatorCreateOutputDirErrorIsValidJSON(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(blocker, `sub "x"\dir`)
	out := c102DecodeResult(t, ExecuteDocumentCreator(context.Background(), c102MarotoConfig(outputDir), "create_pdf", "T", "Body", "", "", "", false, "", ""))
	if out["status"] != "error" || !strings.Contains(out["message"].(string), "create output dir") {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDocumentCreatorMarotoErrorsAreValidJSON(t *testing.T) {
	t.Run("invalid sections", func(t *testing.T) {
		out := c102DecodeResult(t, createPDFMaroto(t.TempDir(), "T", "", "x", "A4", false, `{"type":"te\x"}`))
		if out["status"] != "error" || !strings.Contains(out["message"].(string), "invalid sections JSON") {
			t.Fatalf("decoded = %+v", out)
		}
	})
	t.Run("save fails", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), `missing "dir"`, `sub\dir`)
		out := c102DecodeResult(t, createPDFMaroto(missing, "T", "Body", "x", "A4", false, ""))
		if out["status"] != "error" || !strings.Contains(out["message"].(string), "save PDF") {
			t.Fatalf("decoded = %+v", out)
		}
	})
}

func TestDocumentCreatorInvalidSourcePathIsValidJSON(t *testing.T) {
	cfg := &config.DocumentCreatorConfig{
		Enabled:   true,
		Backend:   "gotenberg",
		OutputDir: t.TempDir(),
		Gotenberg: config.GotenbergConfig{URL: "http://127.0.0.1:1"},
	}
	sources, _ := json.Marshal([]string{`..\..\..\C:\data\x "y".docx`})
	out := c102DecodeResult(t, ExecuteDocumentCreatorInWorkspace(context.Background(), cfg, t.TempDir(), "convert_document", "", "", "", "", "", false, "", string(sources)))
	if out["status"] != "error" || !strings.Contains(out["message"].(string), "invalid source path") {
		t.Fatalf("decoded = %+v", out)
	}
}

var c102DefaultNamePattern = regexp.MustCompile(`^doc_\d+_[0-9a-f]{6}$`)

func TestDefaultDocumentNamesAreUnique(t *testing.T) {
	first, second := defaultDocumentName(), defaultDocumentName()
	if first == second {
		t.Fatalf("two default names are equal: %q", first)
	}
	for _, name := range []string{first, second} {
		if !c102DefaultNamePattern.MatchString(name) {
			t.Fatalf("default name %q does not match %s", name, c102DefaultNamePattern)
		}
	}
}

func TestDefaultDocumentNamesDoNotOverwriteEachOther(t *testing.T) {
	t.Run("maroto", func(t *testing.T) {
		dir := t.TempDir()
		first := c102DecodeResult(t, createPDFMaroto(dir, "One", "Body", "", "A4", false, ""))
		second := c102DecodeResult(t, createPDFMaroto(dir, "Two", "Body", "", "A4", false, ""))
		if first["status"] != "success" || second["status"] != "success" || first["file_path"] == second["file_path"] {
			t.Fatalf("first = %+v, second = %+v", first, second)
		}
	})
	t.Run("gotenberg", func(t *testing.T) {
		dir := t.TempDir()
		firstPath, _, err := saveGotenbergOutput([]byte("one"), dir, "", ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		secondPath, _, err := saveGotenbergOutput([]byte("two"), dir, "", ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		if firstPath == secondPath {
			t.Fatalf("both outputs went to %s", firstPath)
		}
		if data, err := os.ReadFile(firstPath); err != nil || string(data) != "one" {
			t.Fatalf("first output = %q, %v", data, err)
		}
		if !c102DefaultNamePattern.MatchString(strings.TrimSuffix(filepath.Base(firstPath), ".pdf")) {
			t.Fatalf("output name %q does not match %s", filepath.Base(firstPath), c102DefaultNamePattern)
		}
	})
}
