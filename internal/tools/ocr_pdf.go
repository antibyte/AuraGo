package tools

import (
	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/security"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ExecutePDFExtractContext preserves the legacy envelope and pure-Go fallback.
func ExecutePDFExtractContext(ctx context.Context, cfg *config.Config, path, session string) string {
	path = normalizeOCRInputPath(path)
	if cfg == nil {
		return pdfError("PDF configuration unavailable")
	}
	manager := OCRManager()
	if cfg.OCR.Enabled && cfg.OCR.AgentAccess && manager != nil && manager.Status().Ready {
		text, jobID, err := ExtractOCRText(ctx, cfg, path, "agent:"+session)
		if err == nil {
			if text == "" {
				return pdfError("PDF contains no recognized text")
			}
			data, _ := json.Marshal(map[string]string{"status": "success", "content": security.IsolateExternalData(text)})
			return string(data)
		}
		if !errors.Is(err, ocr.ErrNotReady) {
			// An incomplete OCR result must not be presented as a full text-layer extraction.
			return pdfError(fmt.Sprintf("OCR extraction incomplete (job_id: %s); use ocr status/result for details", jobID))
		}
	}
	return ExecutePDFExtract(cfg.Directories.WorkspaceDir, path)
}
