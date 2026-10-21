package agent

import (
	"aurago/internal/config"
	"encoding/json"
	"strings"
	"testing"
)

func TestOCRSchemaDiscoveryAndGates(t *testing.T) {
	cfg := &config.Config{OCR: config.DefaultOCRConfig()}
	if buildToolFlagsFromConfig(cfg).OCREnabled {
		t.Fatal("OCR enabled by default")
	}
	cfg.OCR.Enabled = true
	if !buildToolFlagsFromConfig(cfg).OCREnabled {
		t.Fatal("OCR unavailable")
	}
	if ToolCategoryForName("ocr") != "files" {
		t.Fatalf("category %q", ToolCategoryForName("ocr"))
	}
	data, _ := json.Marshal(ocrSchema())
	for _, operation := range []string{"extract", "status", "result", "cancel", "job_id", "file_id"} {
		if !strings.Contains(string(data), operation) {
			t.Fatal(operation, string(data))
		}
	}
	cfg.OCR.AgentAccess = false
	if buildToolFlagsFromConfig(cfg).OCREnabled {
		t.Fatal("agent revocation ignored")
	}
}
