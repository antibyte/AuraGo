package agent

import (
	"strings"

	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func knownReasoningExtractedActionSet(currentTools []openai.Tool, manifest *tools.Manifest) map[string]struct{} {
	_ = manifest
	knownActions := make(map[string]struct{})

	for _, t := range currentTools {
		if t.Function == nil {
			continue
		}
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		knownActions[name] = struct{}{}
		if strings.HasPrefix(name, "skill__") {
			if normalized, err := tools.ValidateSkillShortcutName(strings.TrimPrefix(name, "skill__")); err == nil {
				knownActions[normalized] = struct{}{}
			}
		}
	}

	return knownActions
}

func normalizeParsedToolShortcut(tc ToolCall) ToolCall {
	action := strings.TrimSpace(tc.Action)
	if !strings.HasPrefix(action, "skill__") {
		return tc
	}

	skillName, err := tools.ValidateSkillShortcutName(strings.TrimPrefix(action, "skill__"))
	if err != nil {
		return tc
	}

	tc.Action = "execute_skill"
	if tc.Skill == "" {
		tc.Skill = skillName
	}
	if tc.SkillArgs == nil && len(tc.Params) > 0 {
		tc.SkillArgs = tc.Params
	}
	if tc.Params == nil && len(tc.SkillArgs) > 0 {
		tc.Params = tc.SkillArgs
	}
	return tc
}
