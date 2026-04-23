package agent

import (
	"strings"

	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

var directBuiltinReasoningActions = map[string]struct{}{
	"wikipedia_search": {},
	"ddg_search":       {},
	"brave_search":     {},
	"pdf_extractor":    {},
	"virustotal_scan":  {},
}

func knownReasoningExtractedActionSet(currentTools []openai.Tool, manifest *tools.Manifest) map[string]struct{} {
	knownActions := make(map[string]struct{})

	for name := range allBuiltinToolNameSet() {
		knownActions[name] = struct{}{}
	}
	for name := range directBuiltinReasoningActions {
		knownActions[name] = struct{}{}
	}

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
		if strings.HasPrefix(name, "tool__") {
			normalized := strings.TrimSpace(strings.TrimPrefix(name, "tool__"))
			if normalized != "" {
				knownActions[normalized] = struct{}{}
			}
		}
	}

	if manifest != nil {
		if entries, err := manifest.Load(); err == nil {
			for name := range entries {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				knownActions[trimmed] = struct{}{}
				knownActions["tool__"+trimmed] = struct{}{}
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

func shouldAcceptParsedTextToolCallsInNativeMode(currentTools []openai.Tool, parseSource ToolCallParseSource, primary ToolCall, pending []ToolCall) bool {
	if parseSource != ToolCallParseSourceContentJSON {
		return false
	}
	if !primary.IsTool || strings.TrimSpace(primary.Action) == "" {
		return false
	}

	knownActions := knownReasoningExtractedActionSet(currentTools, nil)
	if _, ok := knownActions[strings.TrimSpace(primary.Action)]; !ok {
		return false
	}

	for _, extra := range pending {
		if !extra.IsTool || strings.TrimSpace(extra.Action) == "" {
			return false
		}
		if _, ok := knownActions[strings.TrimSpace(extra.Action)]; !ok {
			return false
		}
	}

	return true
}
