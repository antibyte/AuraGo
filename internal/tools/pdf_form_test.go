package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/form"
)

func TestPdfFormFields_MissingFile(t *testing.T) {
	result := ExecutePDFOperations("form_fields", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfFormFields_NonExistentFile(t *testing.T) {
	result := ExecutePDFOperations("form_fields", "/nonexistent/file.pdf", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfFillForm_MissingFile(t *testing.T) {
	result := ExecutePDFOperations("fill_form", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfFillForm_MissingFormData(t *testing.T) {
	result := ExecutePDFOperations("fill_form", "/some/file.pdf", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
	if !strings.Contains(r.Message, "source_files is required") {
		t.Errorf("expected source_files required message, got %s", r.Message)
	}
}

func TestPdfFillForm_InvalidJSON(t *testing.T) {
	// Note: file existence is checked before JSON parsing, so non-existent file
	// returns "file not found" rather than "invalid form data"
	result := ExecutePDFOperations("fill_form", "/some/file.pdf", "", "", "", "", "not-json")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfExportForm_MissingFile(t *testing.T) {
	result := ExecutePDFOperations("export_form", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfResetForm_MissingFile(t *testing.T) {
	result := ExecutePDFOperations("reset_form", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfLockForm_MissingFile(t *testing.T) {
	result := ExecutePDFOperations("lock_form", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestPdfOperations_NewOperationsInDefaultError(t *testing.T) {
	result := ExecutePDFOperations("totally_unknown", "", "", "", "", "", "")
	var r pdfOpsResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
	// Verify new operations are listed in the error message
	for _, op := range []string{"form_fields", "fill_form", "export_form", "reset_form", "lock_form"} {
		if !strings.Contains(r.Message, op) {
			t.Errorf("error message should list %s, got: %s", op, r.Message)
		}
	}
}

func TestFieldTypeName(t *testing.T) {
	tests := []struct {
		input form.FieldType
		want  string
	}{
		{form.FTText, "text"},
		{form.FTDate, "date"},
		{form.FTCheckBox, "checkbox"},
		{form.FTComboBox, "combobox"},
		{form.FTListBox, "listbox"},
		{form.FTRadioButtonGroup, "radio"},
		{form.FieldType(99), "unknown"},
	}
	for _, tc := range tests {
		got := fieldTypeName(tc.input)
		if got != tc.want {
			t.Errorf("fieldTypeName(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
