package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
)

// AnalyzeGalleryImage accepts bytes already read through the gallery's rooted file boundary.
func AnalyzeGalleryImage(ctx context.Context, data []byte, prompt string, cfg *config.Config) (string, int, int, error) {
	if cfg == nil {
		return "", 0, 0, fmt.Errorf("vision configuration unavailable")
	}
	if VisionRequiresPublicImageURL(cfg) {
		return "", 0, 0, ErrVisionPublicURLRequired
	}
	security.RegisterSensitive(strings.TrimSpace(cfg.Vision.APIKey))
	security.RegisterSensitive(strings.TrimSpace(cfg.LLM.APIKey))
	if len(data) == 0 || len(data) > 20<<20 {
		return "", 0, 0, fmt.Errorf("image must be between 1 byte and 20 MB")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return "", 0, 0, fmt.Errorf("unsupported image format")
	}
	return analyzeImageReferenceWithPromptContext(ctx, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data), prompt, cfg)
}
