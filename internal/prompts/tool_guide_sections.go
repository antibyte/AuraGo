package prompts

import (
	"path/filepath"
	"strings"
)

type ToolGuideVariant struct{ Text, Version string }
type ToolGuideExposure struct{ Manual, Version, SourceRevision string }

// Selection happens before fitting; only sections surviving the fit are exposures.
var SelectToolGuideVariant = func(manual, canonical, runKey string) ToolGuideVariant { return ToolGuideVariant{canonical, "v1"} }

// ToolGuideSections keeps a manual atomic even when it contains top-level
// headings or fenced code. Markdown contents cannot change its ledger policy.
func ToolGuideSections(flags *ContextFlags) []PromptSection {
	if flags == nil {
		return nil
	}
	var sections []PromptSection
	for _, guide := range flags.PredictedGuides {
		var exposure *ToolGuideExposure
		if path := flags.GuideSources[PromptRevision(guide)]; path != "" {
			if canonical, ok := canonicalToolGuide(path); ok && guideConditionsAllow(canonical.conditions, flags) {
				manual := strings.TrimSuffix(filepath.Base(path), ".md")
				variant := ToolGuideVariant{Text: guide, Version: "v1"}
				if flags.OptimizerEnabled {
					variant = SelectToolGuideVariant(manual, canonical.content, flags.OptimizerRunID)
				}
				if variant.Version != "v1" {
					if body, valid := SanitizeToolGuideOverride(variant.Text); valid {
						guide = truncateGuide(body, 2048)
					} else {
						variant.Version = "v1"
					}
				}
				exposure = &ToolGuideExposure{Manual: manual, Version: variant.Version, SourceRevision: PromptRevision(canonical.content)}
			}
		}
		if flags.NativeToolsEnabled {
			guide = sanitizeDynamicToolGuideForNative(guide)
		}
		if strings.TrimSpace(guide) == "" {
			continue
		}
		id := "tool_guide:" + PromptRevision(guide)
		text := "\n\n# TOOL GUIDES\n"
		if flags.NativeToolsEnabled {
			text += "Translate legacy call examples into native calls; follow the advertised call_method.\n\n"
		}
		sections = append(sections, PromptSection{ID: id, GroupID: id, Text: text + guide + "\n", Priority: 20, GuideExposure: exposure})
	}
	return sections
}

func exposedToolGuides(text string, sections []PromptSection) []ToolGuideExposure {
	var result []ToolGuideExposure
	for _, section := range sections {
		if section.GuideExposure != nil && strings.Contains(text, section.Text) {
			result = append(result, *section.GuideExposure)
		}
	}
	return result
}

func exposedOptionalSections(text string, sections []PromptSection) []string {
	var exposed []string
	for _, section := range sections {
		if section.Text != "" && strings.Contains(text, section.Text) {
			exposed = append(exposed, section.ID)
		}
	}
	return exposed
}

// ReadToolGuideFull supports explicit, paginated retrieval without losing late
// manual sections. It uses the same validation, fallback and digest-bound cache.
func ReadToolGuideFull(path string) (string, bool) {
	if content, ok := activeToolGuideOverride(path); ok {
		return content, true
	}
	entry, ok := canonicalToolGuide(path)
	return entry.content, ok
}
