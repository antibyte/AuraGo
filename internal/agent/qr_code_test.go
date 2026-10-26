package agent

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestQRCodeCatalogGatesCacheAndActionClassification(t *testing.T) {
	cfg := &config.Config{}
	if slices.Contains(ToolNamesFromConfig(cfg), "qr_code") {
		t.Fatal("disabled QR tool exposed")
	}
	before := buildToolFlagsFromConfig(cfg).Key()
	cfg.Tools.QRCode.Enabled = true
	if !slices.Contains(ToolNamesFromConfig(cfg), "qr_code") || before == buildToolFlagsFromConfig(cfg).Key() {
		t.Fatal("QR catalog/cache did not update")
	}
	for _, test := range []struct {
		op     string
		mutate bool
	}{{"presets", false}, {"scan", false}, {"generate", true}, {"batch_generate", true}} {
		a := DescribeAction(ToolCall{Action: "qr_code", Operation: test.op, Params: map[string]interface{}{"output_root": "desktop", "output_path": "Documents/qr.png"}}, nil)
		if a.Mutating != test.mutate || a.HighRisk || !strings.Contains(a.Target, "desktop_file:Documents/qr.png") {
			t.Fatalf("incorrect QR action: %+v", a)
		}
	}
	if a := DescribeAction(ToolCall{Action: "office_document", Operation: "insert_image"}, nil); !a.Mutating {
		t.Fatal("Office image insertion marked readonly")
	}
}

func TestQRCodeNativeAndInvokeRespectScopeAndReadOnly(t *testing.T) {
	resetToolCatalogForTest(t)
	cfg := &config.Config{}
	cfg.Tools.QRCode.Enabled = true
	cfg.Tools.QRCode.ReadOnly = true
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionID: "qr-native"}
	call := ToolCall{Action: "qr_code", Operation: "presets", Params: map[string]interface{}{"operation": "presets"}}
	out := dispatchInner(context.Background(), call, dc)
	if !strings.Contains(out, `"content_types"`) || !strings.Contains(out, `"epc"`) {
		t.Fatalf("QR native not routed: %s", out)
	}
	wrapped := ToolCall{Params: map[string]interface{}{"tool_name": "qr_code", "arguments": map[string]interface{}{"operation": "generate", "output_path": "blocked.png", "spec": map[string]interface{}{"content": map[string]interface{}{"type": "text", "text": "x"}}}}}
	out = dispatchInvokeTool(context.Background(), wrapped, dc)
	if !strings.Contains(out, "read-only") {
		t.Fatalf("wrapped readonly denied incorrectly: %s", out)
	}
	dc.ToolScopeRestricted = true
	dc.AllowedTools = normalizedAllowedToolSet([]string{"invoke_tool"})
	out = dispatchInvokeTool(context.Background(), wrapped, dc)
	if !strings.Contains(out, "tool_scope_denied") {
		t.Fatalf("wrapper scope bypass: %s", out)
	}
	cfg.Tools.QRCode.Enabled = false
	dc.ToolScopeRestricted = false
	out = dispatchInner(context.Background(), call, dc)
	if strings.Contains(out, `"content_types"`) {
		t.Fatal("disabled native QR executed")
	}
}
