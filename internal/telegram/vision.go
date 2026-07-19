package telegram

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// AnalyzeImage sends a downloaded channel image through the shared Vision path.
// This keeps provider-specific input restrictions consistent across channels.
func AnalyzeImage(filePath string, cfg *config.Config) (string, error) {
	const prompt = "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions."
	visionCfg := cfg
	if cfg != nil && strings.TrimSpace(cfg.Vision.Model) == "" {
		cfgCopy := *cfg
		cfgCopy.Vision.Model = "google/gemini-2.5-flash-lite-preview-09-2025"
		visionCfg = &cfgCopy
	}
	analysis, _, _, err := tools.AnalyzeTrustedImageFileWithPrompt(filePath, prompt, visionCfg)
	return analysis, err
}
