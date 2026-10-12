package agent

import (
	"aurago/internal/config"
	"context"
	"strings"
	"testing"
)

func TestGogCatalogAndDispatchGates(t *testing.T) {
	if len(appendGogSchemas(nil, ToolFeatureFlags{})) != 0 {
		t.Fatal("disabled schemas exposed")
	}
	schemas := appendGogSchemas(nil, ToolFeatureFlags{GogEnabled: true})
	if len(schemas) != 9 {
		t.Fatal(len(schemas))
	}
	for _, schema := range schemas {
		if !strings.HasPrefix(schema.Function.Name, "gog_") {
			t.Fatal(schema.Function.Name)
		}
	}
	out, handled := dispatchGog(context.Background(), ToolCall{Action: "gog_drive"}, &DispatchContext{Cfg: &config.Config{}})
	if !handled || !strings.Contains(out, `"disabled"`) {
		t.Fatal(out)
	}
}

func TestGogLiveAccountChangeDeniesCapturedRun(t *testing.T) {
	original := &config.Config{}
	original.Gog.Accounts = []config.GogAccount{{ID: "a", Enabled: true, AgentAccess: true}}
	current := *original
	current.Gog.Accounts = []config.GogAccount{{ID: "a", Enabled: true, AgentAccess: false}}
	run := *original
	run.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return original, &current }
	if _, ok := dispatchAuthorization(&run); ok {
		t.Fatal("account revocation allowed an old run")
	}
}
