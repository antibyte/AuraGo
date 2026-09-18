package tools

import (
	"bytes"
	"context"
	"fmt"

	"aurago/internal/config"
)

// RenderReportHTMLPDF uses an explicitly configured renderer without starting
// services or writing files. Callers supply escaped, resource-free report HTML.
func RenderReportHTMLPDF(ctx context.Context, cfg *config.GotenbergConfig, html string) ([]byte, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("report renderer is not configured")
	}
	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/convert/html", map[string]string{"printBackground": "true", "preferCssPageSize": "true"}, map[string][]byte{"index.html": []byte(html)})
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, fmt.Errorf("report renderer returned a non-PDF response")
	}
	return data, nil
}
