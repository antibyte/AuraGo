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
