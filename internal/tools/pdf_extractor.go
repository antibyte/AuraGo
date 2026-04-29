package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/ledongthuc/pdf"
)

// ExecutePDFExtract extracts all text content from a PDF file.
// workspaceDir is used to resolve relative paths and enforce path-traversal
// protection — the resolved file must stay within the project tree.
func ExecutePDFExtract(workspaceDir, filePath string) string {
	if filePath == "" {
		return pdfError("filepath is required")
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir
	resolved, err := resolveToolInputPath(filePath, cfg)
	if err != nil {
		return pdfError(err.Error())
	}

	f, r, err := pdf.Open(resolved)
	if err != nil {
		return pdfError(fmt.Sprintf("Failed to open PDF: %v", err))
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	content := strings.TrimSpace(sb.String())
	if content == "" {
		return pdfError("PDF contains no extractable text (may be image-based)")
	}

	result := map[string]interface{}{
		"status":  "success",
		"content": security.IsolateExternalData(content),
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func pdfError(msg string) string {
	result := map[string]interface{}{
		"status":  "error",
		"message": msg,
	}
	b, _ := json.Marshal(result)
	return string(b)
}
