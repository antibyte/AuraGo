package prompts

import (
	"fmt"
	"strings"

	"aurago/internal/memory"
	"aurago/internal/promptsource"
	"gopkg.in/yaml.v3"
)

// ValidatePersonalitySource rejects typos rather than silently changing a persona.
// Plain Markdown remains a supported profile format.
func ValidatePersonalitySource(content string) error {
	frontmatter, _, present, err := promptsource.Split(content)
	if err != nil || !present {
		return err
	}
	metadata := PromptMetadata{Meta: memory.DefaultPersonalityMeta()}
	decoder := yaml.NewDecoder(strings.NewReader(frontmatter))
	decoder.KnownFields(true)
	if err = decoder.Decode(&metadata); err != nil {
		return fmt.Errorf("invalid personality metadata: %w", err)
	}
	if err = metadata.Meta.Validate(); err != nil {
		return fmt.Errorf("invalid personality metadata: %w", err)
	}
	return nil
}
