package agent

import (
	"aurago/internal/config"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestRuViewDiscoveryRequiresBothGrants(t *testing.T) {
	c := &config.Config{}
	for _, tc := range []struct{ enabled, agent, want bool }{{false, false, false}, {true, false, false}, {false, true, false}, {true, true, true}} {
		c.RuView.Enabled, c.RuView.AgentAccess = tc.enabled, tc.agent
		if got := containsName(ToolNamesFromConfig(c), "ruview"); got != tc.want {
			t.Fatalf("discovery: enabled=%v agent=%v got=%v", tc.enabled, tc.agent, got)
		}
	}
}

func TestRuViewSchemaHasOnlyFixedReadOperations(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{RuViewEnabled: true}, slog.Default())
	for _, tool := range schemas {
		if tool.Function != nil && tool.Function.Name == "ruview" {
			data, err := json.Marshal(tool.Function.Parameters)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			for _, read := range []string{"status", "nodes", "sensing", "model_info", "recordings"} {
				if !strings.Contains(text, `"`+read+`"`) {
					t.Fatal("missing read operation", read)
				}
			}
			for _, write := range []string{"flash", "start", "delete", "calibrate", "url", "token"} {
				if strings.Contains(text, `"`+write+`"`) {
					t.Fatal("unsafe agent parameter", write)
				}
			}
			return
		}
	}
	t.Fatal("RuView schema missing")
}
