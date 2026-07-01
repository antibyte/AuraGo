package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/fritzbox"
)

func TestDecodeFritzBoxArgsAcceptsTemplateIDAlias(t *testing.T) {
	tc := ToolCall{
		Action: "fritzbox_smarthome",
		Params: map[string]interface{}{
			"operation":   "apply_template",
			"template_id": "tpl-1",
		},
	}

	req := decodeFritzBoxArgs(tc)
	if req.TemplateID != "tpl-1" {
		t.Fatalf("TemplateID = %q, want tpl-1", req.TemplateID)
	}
}

func TestSmartHomeTemplateSubFeatureBlocksBeforeClientCall(t *testing.T) {
	cfg := config.Config{}
	cfg.FritzBox.SmartHome.Enabled = true
	cfg.FritzBox.SmartHome.SubFeatures.Templates = false
	client := &fritzbox.Client{Cfg: cfg}

	out := fbSmartHomeOp(client, fritzBoxArgs{}, "get_templates", fritzBoxTestLogger())
	if !strings.Contains(out, "templates sub-feature is not enabled") {
		t.Fatalf("output = %s", out)
	}
}

func TestNetworkWakeOnLANSubFeatureBlocksBeforeClientCall(t *testing.T) {
	cfg := config.Config{}
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.SubFeatures.WakeOnLAN = false
	client := &fritzbox.Client{Cfg: cfg}

	out := fbNetworkOp(client, fritzBoxArgs{MACAddress: "AA:BB:CC:DD:EE:FF"}, "wake_on_lan", fritzBoxTestLogger())
	if !strings.Contains(out, "wake_on_lan sub-feature is not enabled") {
		t.Fatalf("output = %s", out)
	}
}

func fritzBoxTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
