package agent

import "testing"

func TestHardAlwaysToolMetadataExists(t *testing.T) {
	for _, name := range []string{"discover_tools", "invoke_tool", "execute_skill", "run_tool"} {
		meta, ok := lookupToolMetadata(name)
		if !ok {
			t.Fatalf("missing metadata for %s", name)
		}
		if meta.VisibilityClass != ToolVisibilityHardAlways {
			t.Fatalf("%s visibility = %s, want hard_always", name, meta.VisibilityClass)
		}
	}
	if _, ok := lookupToolMetadata("activate_tools"); ok {
		t.Fatal("activate_tools must not have model-visible registry metadata")
	}
}
