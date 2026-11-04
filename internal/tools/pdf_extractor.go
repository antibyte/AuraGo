package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"aurago/internal/config"
	"aurago/internal/pdftext"
	"aurago/internal/security"
)

// ExecutePDFExtract extracts all text content from a PDF file.
// workspaceDir is used to resolve relative paths and enforce path-traversal
// protection — the resolved file must stay within the project tree.
func ExecutePDFExtract(workspaceDir, filePath string) string {
	return ExecutePDFExtractContext(context.Background(), workspaceDir, filePath)
}

// ExecutePDFExtractContext parses the rooted input handle with shared limits.
func ExecutePDFExtractContext(ctx context.Context, workspaceDir, filePath string) string {
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
	info, err := input.Stat()
	if err != nil {
		return pdfError(fmt.Sprintf("Failed to stat PDF: %v", err))
	}
	content, err := pdftext.Extract(ctx, input, info.Size())
	if err != nil {
		return pdfError(err.Error())
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
