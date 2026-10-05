package tools

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDocumentSuccessJSONKeepsWindowsPathsValid(t *testing.T) {
	raw := documentSuccessJSON(`C:\data\documents\bericht.pdf`, "/files/documents/bericht.pdf", "bericht.pdf", "maroto")
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	if out["status"] != "success" || out["file_path"] != `C:\data\documents\bericht.pdf` ||
		out["web_path"] != "/files/documents/bericht.pdf" || out["filename"] != "bericht.pdf" || out["backend"] != "maroto" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestCreatePDFMarotoReturnsParsableResult(t *testing.T) {
	dir := t.TempDir()
	raw := createPDFMaroto(dir, "Titel", "Hallo Welt", "bericht", "A4", false, "")
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	if out["status"] != "success" || !strings.HasSuffix(out["file_path"], ".pdf") {
		t.Fatalf("result = %+v", out)
	}
	if _, err := os.Stat(out["file_path"]); err != nil {
		t.Fatalf("pdf missing: %v", err)
	}
}
