package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	input, err := rootedToolOpen(resolved)
	if err != nil {
		return pdfError(fmt.Sprintf("Failed to open PDF: %v", err))
	}
	defer input.Close()
	stagingDir, err := os.MkdirTemp("", "aurago-pdf-extract-*")
	if err != nil {
		return pdfError(fmt.Sprintf("Failed to stage PDF: %v", err))
	}
	defer os.RemoveAll(stagingDir)
	stagedPath := filepath.Join(stagingDir, "source.pdf")
	staged, err := os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return pdfError(fmt.Sprintf("Failed to stage PDF: %v", err))
	}
	_, copyErr := io.Copy(staged, input)
	closeErr := staged.Close()
	if copyErr != nil || closeErr != nil {
		return pdfError(fmt.Sprintf("Failed to stage PDF: %v %v", copyErr, closeErr))
	}
	f, r, err := pdf.Open(stagedPath)
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
