package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestBuildNativeToolSchemasGatesManus(t *testing.T) {
	t.Parallel()

	disabled := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{}, nil))
	if containsName(disabled, "manus") {
		t.Fatal("manus schema present while disabled")
	}
	enabled := toolNames(BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{ManusEnabled: true}, nil))
	if !containsName(enabled, "manus") {
		t.Fatal("manus schema missing while enabled")
	}
}

func TestBuildNativeToolSchemasPublishesStructuredOutputAsObject(t *testing.T) {
	t.Parallel()

	tools := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{ManusEnabled: true}, nil)
	for _, candidate := range tools {
		if candidate.Function == nil || candidate.Function.Name != "manus" {
			continue
		}
		params, ok := candidate.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("manus parameters type = %T, want map[string]interface{}", candidate.Function.Parameters)
		}
		properties, ok := params["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("manus properties = %#v, want object", params["properties"])
		}
		structured, ok := properties["structured_output_schema"].(map[string]interface{})
		if !ok {
			t.Fatalf("structured_output_schema = %#v, want object schema", properties["structured_output_schema"])
		}
		if got := structured["type"]; got != "object" {
			t.Fatalf("structured_output_schema type = %#v, want object", got)
		}
		if got := structured["additionalProperties"]; got != true {
			t.Fatalf("structured_output_schema additionalProperties = %#v, want true", got)
		}
		return
	}
	t.Fatal("manus schema missing while enabled")
}

func TestDecodeManusCallArgsAcceptsStructuredOutputObjectAndLegacyString(t *testing.T) {
	t.Parallel()

	want := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"summary": map[string]interface{}{"type": "string"},
		},
	}
	for _, tc := range []ToolCall{
		{Params: map[string]interface{}{"structured_output_schema": want}},
		{Params: map[string]interface{}{"structured_output_schema": `{"type":"object","properties":{"summary":{"type":"string"}}}`}},
	} {
		got := decodeManusCallArgs(tc)
		if got.SchemaError != "" {
			t.Fatalf("decode schema error = %q", got.SchemaError)
		}
		if got.StructuredOutputSchema["type"] != "object" {
			t.Fatalf("decoded schema = %#v, want object type", got.StructuredOutputSchema)
		}
		properties, ok := got.StructuredOutputSchema["properties"].(map[string]interface{})
		if !ok || properties["summary"] == nil {
			t.Fatalf("decoded properties = %#v, want summary", got.StructuredOutputSchema["properties"])
		}
	}
}

func TestManusStrictToolSchemaUsesLegacyStringFallback(t *testing.T) {
	t.Parallel()

	tools := BuildNativeToolSchemaSnapshot(t.TempDir(), nil, ToolFeatureFlags{ManusEnabled: true}, nil).StrictSchemas()
	for _, candidate := range tools {
		if candidate.Function == nil || candidate.Function.Name != "manus" {
			continue
		}
		params := candidate.Function.Parameters.(map[string]interface{})
		properties := params["properties"].(map[string]interface{})
		structured := properties["structured_output_schema"].(map[string]interface{})
		if got := structured["type"]; got != "string" {
			t.Fatalf("strict structured_output_schema type = %#v, want string fallback", got)
		}
		return
	}
	t.Fatal("manus strict schema missing while enabled")
}

func TestToolFeatureFlagsKeyIncludesManusEnabled(t *testing.T) {
	t.Parallel()

	without := ToolFeatureFlags{}.Key()
	with := ToolFeatureFlags{ManusEnabled: true}.Key()
	if without == with {
		t.Fatal("ManusEnabled did not change schema cache key")
	}
}

func TestBuildToolFlagsRequiresEnabledManusAndVaultKey(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Manus: config.ManusConfig{Enabled: true}}
	if buildToolFlagsFromConfig(cfg).ManusEnabled {
		t.Fatal("Manus tool enabled without API key")
	}
	cfg.Manus.APIKey = "secret"
	if !buildToolFlagsFromConfig(cfg).ManusEnabled {
		t.Fatal("Manus tool disabled with enabled config and API key")
	}
}

func TestManusIsDiscoverableAndSeededFromIntent(t *testing.T) {
	t.Parallel()

	if got := resolveDiscoverToolName("manus ai"); got != "manus" {
		t.Fatalf("resolveDiscoverToolName(manus ai) = %q", got)
	}
	if seeds := adaptiveFamilySeedsForQuery("Delegate this research task to Manus"); !containsName(seeds, "manus") {
		t.Fatalf("adaptive seeds = %#v, want manus", seeds)
	}
}
