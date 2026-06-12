package agent

import (
	"log/slog"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func refreshActivatedNativeToolSchemas(s *agentLoopState) bool {
	if s == nil || !s.useNativeFunctions || s.nativeSchemaSnapshot == nil {
		return false
	}
	activated := ConsumeActivatedTools(s.runCfg.SessionID)
	if len(activated) == 0 {
		return false
	}

	allSchemas := s.nativeSchemaSnapshot.FullSchemas()
	schemaByName := make(map[string]openai.Tool, len(allSchemas))
	for _, schema := range allSchemas {
		if schema.Function != nil && schema.Function.Name != "" {
			schemaByName[schema.Function.Name] = schema
		}
	}

	activeSet := make(map[string]bool, len(s.req.Tools)+len(activated))
	for _, schema := range s.req.Tools {
		if schema.Function != nil && schema.Function.Name != "" {
			activeSet[schema.Function.Name] = true
		}
	}
	activeTools := append([]openai.Tool(nil), s.req.Tools...)
	activatedSet := make(map[string]bool, len(activated))
	for _, name := range activated {
		activatedSet[name] = true
		if activeSet[name] {
			continue
		}
		if schema, ok := schemaByName[name]; ok {
			activeTools = append(activeTools, schema)
			activeSet[name] = true
		}
	}

	activeTools = trimActivatedToolSchemasToTotalCap(activeTools, activatedSet, s.toolingPolicy.EffectiveMaxTotalTools, s.runCfg.Config, s.currentLogger)
	activeNames := toolSchemaNames(activeTools)
	if s.toolingPolicy.StructuredOutputsEnabled {
		activeTools = strictSchemasForActiveTools(s.nativeSchemaSnapshot, activeTools)
	}

	s.req.Tools = activeTools
	s.flags.ActiveNativeTools = activeNames
	s.adaptiveFilteredTools = removeNamesFromList(s.adaptiveFilteredTools, activatedSet)
	SetDiscoverToolsState(s.runCfg.SessionID, allSchemas, activeSchemasFromNames(allSchemas, activeNames), s.toolGuidesDir)
	return true
}

func trimActivatedToolSchemasToTotalCap(tools []openai.Tool, activated map[string]bool, maxTotal int, cfg *config.Config, logger *slog.Logger) []openai.Tool {
	if maxTotal <= 0 || len(tools) <= maxTotal {
		return tools
	}
	hardSet := stringSet(adaptiveHardAlwaysInclude(cfg))
	out := append([]openai.Tool(nil), tools...)
	for len(out) > maxTotal {
		dropped := false
		for i := len(out) - 1; i >= 0; i-- {
			name := nativeToolSortName(out[i])
			if name == "" || hardSet[name] || activated[name] {
				continue
			}
			out = append(out[:i], out[i+1:]...)
			dropped = true
			break
		}
		if !dropped {
			if logger != nil {
				logger.Warn("[NativeTools] Activated tools plus hard tools exceed total schema cap",
					"tool_count", len(out),
					"max_total", maxTotal)
			}
			return out
		}
	}
	return out
}

func activeSchemasFromNames(allSchemas []openai.Tool, names []string) []openai.Tool {
	nameSet := stringSet(names)
	active := make([]openai.Tool, 0, len(names))
	for _, schema := range allSchemas {
		if schema.Function != nil && nameSet[schema.Function.Name] {
			active = append(active, schema)
		}
	}
	return active
}

func removeNamesFromList(names []string, remove map[string]bool) []string {
	if len(names) == 0 || len(remove) == 0 {
		return names
	}
	out := names[:0]
	for _, name := range names {
		if !remove[name] {
			out = append(out, name)
		}
	}
	return out
}
