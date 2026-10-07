package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestManageMissionsRefusesToChangeFlowMissions(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	mm := tools.NewMissionManagerV2(t.TempDir(), nil)
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	dc := &DispatchContext{Cfg: cfg, Logger: slog.Default(), MissionManagerV2: mm}
	for _, op := range []string{"update", "delete"} {
		out, handled := dispatchComm(context.Background(), ToolCall{Action: "manage_missions", Params: map[string]interface{}{
			"operation": op, "id": id, "title": "Neu",
		}}, dc)
		if !handled || !strings.Contains(out, "EasyDrag") || !strings.Contains(out, `"status":"error"`) {
			t.Fatalf("%s: %q handled=%v", op, out, handled)
		}
	}
	if m, ok := mm.Get(id); !ok || m.Name != "Bericht" {
		t.Fatalf("the flow mission changed: %+v", m)
	}
	out, _ := dispatchComm(context.Background(), ToolCall{Action: "manage_missions", Params: map[string]interface{}{"operation": "list"}}, dc)
	if !strings.Contains(out, id) {
		t.Fatalf("list must include flow missions: %q", out)
	}
}
