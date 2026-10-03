package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestPDFOperationsStagesInputsAndPublishesOutput(t *testing.T) {
	workspace := t.TempDir()
	created := ExecuteDocumentCreator(context.Background(), &config.DocumentCreatorConfig{
		Enabled: true, Backend: "maroto", OutputDir: workspace,
	}, "create_pdf", "Audit", "PDF fixture", "", "input.pdf", "A4", false, "", "")
	if !strings.Contains(created, `"status":"success"`) {
		t.Fatalf("create fixture: %s", created)
	}
	result := ExecutePDFOperations(workspace, "page_count", "input.pdf", "", "", "", "", "")
	var counted struct {
		Status string `json:"status"`
		Pages  int    `json:"pages"`
	}
	if err := json.Unmarshal([]byte(result), &counted); err != nil || counted.Status != "success" || counted.Pages < 1 {
		t.Fatalf("page count: %s, %v", result, err)
	}
	result = ExecutePDFOperations(workspace, "compress", "input.pdf", "output.pdf", "", "", "", "")
	var converted pdfOpsResult
	if err := json.Unmarshal([]byte(result), &converted); err != nil || converted.Status != "success" {
		t.Fatalf("compress: %s, %v", result, err)
	}
	if len(converted.Files) != 1 || converted.Files[0] != filepath.Join(workspace, "output.pdf") {
		t.Fatalf("published path: %#v", converted.Files)
	}
	if strings.Contains(converted.Message, "aurago-pdf-") || strings.Contains(converted.Message, "result.pdf") {
		t.Fatalf("published message should not expose staging paths: %q", converted.Message)
	}
	if info, err := os.Stat(converted.Files[0]); err != nil || info.Size() == 0 {
		t.Fatalf("published PDF: %v, %v", info, err)
	}
	result = ExecutePDFOperations(workspace, "split", "input.pdf", "pages", "", "", "", "")
	if err := json.Unmarshal([]byte(result), &converted); err != nil || converted.Status != "success" || !strings.Contains(converted.Message, filepath.Join(workspace, "pages")) || strings.Contains(converted.Message, "aurago-pdf-") {
		t.Fatalf("split should report the published directory: %s, %v", result, err)
	}
}
