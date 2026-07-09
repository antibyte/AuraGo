package agent

import "testing"

func TestHuggingFaceNativeToolIsFeatureGated(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "huggingface") {
		t.Fatal("huggingface tool must be hidden when the integration is disabled")
	}
	if !containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{HuggingFaceEnabled: true})), "huggingface") {
		t.Fatal("huggingface tool must be exposed when the integration is enabled")
	}
}
