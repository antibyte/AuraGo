package agent

import (
	"log/slog"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/tools"
)

type nativeToolSchemaSnapshot struct {
	full   []openai.Tool
	strict []openai.Tool
}

func newNativeToolSchemaSnapshot(schemas []openai.Tool) *nativeToolSchemaSnapshot {
	full := cloneToolSchemasForSnapshot(schemas)
	strict := cloneToolSchemasForSnapshot(schemas)
	for i := range strict {
		if strict[i].Function == nil {
			continue
		}
		if params, ok := strict[i].Function.Parameters.(map[string]interface{}); ok {
			normalizeStrictSchemaRequiredRec(params)
		}
		strict[i].Function.Strict = true
	}
	return &nativeToolSchemaSnapshot{full: full, strict: strict}
}

func (s *nativeToolSchemaSnapshot) FullSchemas() []openai.Tool {
	if s == nil || len(s.full) == 0 {
		return nil
	}
	out := make([]openai.Tool, len(s.full))
	copy(out, s.full)
	return out
}

func (s *nativeToolSchemaSnapshot) StrictSchemas() []openai.Tool {
	if s == nil || len(s.strict) == 0 {
		return nil
	}
	out := make([]openai.Tool, len(s.strict))
	copy(out, s.strict)
	return out
}

func cloneToolSchemasForSnapshot(schemas []openai.Tool) []openai.Tool {
	if len(schemas) == 0 {
		return nil
	}
	out := make([]openai.Tool, len(schemas))
	for i, schema := range schemas {
		out[i] = cloneToolSchemaForSnapshot(schema)
	}
	return out
}

func cloneToolSchemaForSnapshot(schema openai.Tool) openai.Tool {
	out := schema
	if schema.Function != nil {
		fn := *schema.Function
		fn.Parameters = cloneJSONSchemaValue(fn.Parameters)
		out.Function = &fn
	}
	return out
}

func cloneJSONSchemaValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			out[k] = cloneJSONSchemaValue(v)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, v := range typed {
			out[i] = cloneJSONSchemaValue(v)
		}
		return out
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	case []map[string]interface{}:
		out := make([]map[string]interface{}, len(typed))
		for i, v := range typed {
			if cloned, ok := cloneJSONSchemaValue(v).(map[string]interface{}); ok {
				out[i] = cloned
			}
		}
		return out
	default:
		return value
	}
}

func strictSchemasForActiveTools(snapshot *nativeToolSchemaSnapshot, active []openai.Tool) []openai.Tool {
	if snapshot == nil || len(active) == 0 {
		return nil
	}
	strictByName := make(map[string]openai.Tool, len(snapshot.strict))
	for _, schema := range snapshot.strict {
		if schema.Function != nil {
			strictByName[schema.Function.Name] = schema
		}
	}
	out := make([]openai.Tool, 0, len(active))
	for _, schema := range active {
		if schema.Function == nil {
			out = append(out, schema)
			continue
		}
		if strictSchema, ok := strictByName[schema.Function.Name]; ok {
			out = append(out, strictSchema)
			continue
		}
		out = append(out, schema)
	}
	return out
}

func BuildNativeToolSchemaSnapshot(skillsDir string, manifest *tools.Manifest, ff ToolFeatureFlags, logger *slog.Logger) *nativeToolSchemaSnapshot {
	cacheKey := dynamicToolSchemaCacheKey{
		Flags:               ff,
		SkillsFingerprint:   nativeSkillsFingerprint(skillsDir),
		ManifestFingerprint: nativeManifestFingerprint(manifest),
	}
	if cached, ok := dynamicToolSchemaCache.Load(cacheKey); ok {
		if snapshot, ok := cached.(*nativeToolSchemaSnapshot); ok {
			return snapshot
		}
	}

	allTools := buildNativeToolSchemasUncached(skillsDir, manifest, ff, logger)
	snapshot := newNativeToolSchemaSnapshot(allTools)
	dynamicToolSchemaCache.Store(cacheKey, snapshot)
	return snapshot
}
