package agent

import (
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func imageGenerationToolResultPayload(cfg *config.Config, result *tools.ImageGenResult, prompt, enhancedPrompt string) map[string]interface{} {
	payload := map[string]interface{}{
		"status":          "success",
		"web_path":        result.WebPath,
		"markdown":        result.Markdown,
		"prompt":          prompt,
		"enhanced_prompt": enhancedPrompt,
		"model":           result.Model,
		"provider":        result.Provider,
		"size":            result.Size,
		"duration_ms":     result.DurationMs,
	}
	if cfg != nil && result.Filename != "" {
		payload["local_path"] = filepath.Join(cfg.Directories.DataDir, "generated_images", result.Filename)
	}
	return payload
}
