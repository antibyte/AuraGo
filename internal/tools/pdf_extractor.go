package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	resolved, err := securePDFPath(workspaceDir, filePath)
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

// securePDFPath resolves a file path relative to the workspace directory and
// ensures the result stays within the project tree (2 levels up from workspace).
func securePDFPath(workspaceDir, userPath string) (string, error) {
	absWorkdir, err := filepath.EvalSymlinks(workspaceDir)
	if err != nil {
		absWorkdir, err = filepath.Abs(workspaceDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve workspace path: %w", err)
		}
	}

	projectRoot := filepath.Dir(filepath.Dir(absWorkdir))

	var resolved string
	if filepath.IsAbs(userPath) {
		resolved = filepath.Clean(userPath)
	} else {
		resolved = filepath.Clean(filepath.Join(absWorkdir, userPath))
	}

	absResolved, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		// File may not exist yet — use the cleaned path for the check.
		absResolved = resolved
	}

	if !strings.HasPrefix(absResolved, projectRoot+string(os.PathSeparator)) && absResolved != projectRoot {
		return "", fmt.Errorf("path traversal not allowed")
	}

	if _, err := os.Stat(absResolved); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", userPath)
	}

	return absResolved, nil
}

func pdfError(msg string) string {
	result := map[string]interface{}{
		"status":  "error",
		"message": msg,
	}
	b, _ := json.Marshal(result)
	return string(b)
}
