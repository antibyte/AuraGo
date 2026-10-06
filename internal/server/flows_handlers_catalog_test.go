package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFlowsAPICatalog(t *testing.T) {
	s, token := newFlowsTestServer(t)
	body := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types?lang=de", token, ""))
	found := false
	for _, raw := range body["node_types"].([]any) {
		if m, _ := raw.(map[string]any); m["type"] == "trigger.manual" {
			found = true
		}
	}
	if !found {
		t.Fatal("trigger.manual is missing from the catalog")
	}
	if cats, _ := body["categories"].([]any); len(cats) < 8 {
		t.Fatalf("categories = %+v", cats)
	}
	opts := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/notify.push/options/channel", token, ""))["options"].([]any)
	if first, _ := opts[0].(map[string]any); first["value"] != "all" {
		t.Fatalf("channel options = %+v", opts)
	}
	acc := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/notify.email/options/account", token, ""))["options"].([]any)
	if first, _ := acc[0].(map[string]any); first["value"] != "" {
		t.Fatalf("account options = %+v", acc)
	}
	ai := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/ai.step/options/model", token, ""))["options"].([]any)
	if len(ai) != 1 {
		t.Fatalf("ai model options without providers = %+v", ai)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/notify.push/options/title", token, ""); w.Code != http.StatusNotFound {
		t.Fatalf("param without options = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/node-types/bogus/options/x", token, ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown type = %d", w.Code)
	}
	if tpls := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/templates", token, ""))["templates"].([]any); len(tpls) != 6 {
		t.Fatalf("templates = %+v", tpls)
	}
	val := flowsBody(t, flowsCall(t, s, http.MethodPost, "/api/desktop/flows/validate", token,
		`{"doc":{"schema":1,"name":"","nodes":[],"edges":[]},"mode":"publish"}`))
	if val["valid"] != false || !strings.Contains(fmt.Sprint(val["issues"]), "FLOW_NAME_REQUIRED") {
		t.Fatalf("validate = %+v", val)
	}
}

func TestFlowOptionsFromConfiguration(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	s.Cfg.Telegram.BotToken, s.Cfg.Telegram.UserID = "1:x", 7
	opts, err := s.flowOptions(context.Background(), "notification_channels", "en")
	if err != nil || len(opts) != 3 || opts[2].Value != "telegram" {
		t.Fatalf("channels = %+v, %v", opts, err)
	}
	if _, err := s.flowOptions(context.Background(), "bogus", "en"); err == nil {
		t.Fatal("unknown sources must fail")
	}
	if ha, err := s.flowOptions(context.Background(), "ha_entities", "en"); err != nil || len(ha) != 0 {
		t.Fatalf("HA without configuration = %+v, %v", ha, err)
	}
}
