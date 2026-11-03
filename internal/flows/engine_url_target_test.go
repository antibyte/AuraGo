package flows

import "testing"

func TestRunRefusesUntrustedURLAuthority(t *testing.T) {
	for _, typ := range []string{TypeHTTPRequest, TypeWebRead, "tool.fixture_fetch"} {
		for _, mode := range []RunMode{ModeLive, ModeTest} {
			for _, url := range []string{"{{trigger.data.link}}", "https://{{trigger.data.host}}/x", "https://api.example.org:{{trigger.data.host}}/x", "https://u{{trigger.data.host}}@api.example.org/x"} {
				t.Run(typ+"/"+string(mode)+"/"+url, func(t *testing.T) {
					r := newSecretRun(t)
					RefreshGenericTools(r.eng.reg, []GenericTool{{Name: "fixture_fetch", Schema: genericSchemaOf(map[string]any{"url": genericProp("string", "")})}}, fullEnv())
					f, call := secretCallFlow(map[string]any{"url": url})
					f.NodeByID(call).Type = typ
					f.NodeByID(call).Settings.Retry = RetryPolicy{Count: 3}
					res := r.run(f, f.Nodes[0].ID, mode)
					step := stepOf(res, call)
					if res.Status != RunError || step == nil || step.ErrorCode != "FLOW_PARAM_INVALID" || r.tools.count() != 0 || r.secrets.count() != 0 {
						t.Fatalf("unchecked authority ran: status=%s error=%s step=%+v tool calls=%d vault reads=%d", res.Status, res.ErrorCode, step, r.tools.count(), r.secrets.count())
					}
				})
			}
		}
	}
}
