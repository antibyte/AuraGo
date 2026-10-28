package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/qrcode"
)

func TestQRCodeScopedPermissionsAndLiveReadRevocation(t *testing.T) {
	cfg := qrTestConfig(t)
	ctx := WithRuntimePermissions(context.Background(), RuntimePermissions{})
	if got := qrTestPayload(t, ExecuteQRCode(ctx, cfg, qrTestArgs("denied.png"))); got["status"] != "error" {
		t.Fatal("run-scoped filesystem denial was ignored")
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.WorkspaceDir, "denied.png")); !os.IsNotExist(err) {
		t.Fatal("denied output exists")
	}
	if _, _, err := qrPublish(ctx, cfg, qrFileRef{Root: "workspace", Path: "denied.png"}, []byte("bytes"), ""); err == nil {
		t.Fatal("publication bypassed run-scoped filesystem denial")
	}
	cfg.Agent.AllowNetworkRequests = true
	_, err := qrResolveSpec(ctx, cfg, qrToolArgs{Spec: &qrcode.QRSpec{Content: qrcode.Content{Type: "text", Text: "test"}}, LogoURL: "https://example.invalid/logo.png"})
	if err == nil || !strings.Contains(err.Error(), "network requests is disabled by runtime permissions") {
		t.Fatalf("run-scoped network denial was ignored: %v", err)
	}
	args := qrTestArgs("Documents/test.png")
	args["output_root"] = "desktop"
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "ok" {
		t.Fatal(got)
	}
	current := *cfg
	current.VirtualDesktop.AllowAgentControl = false
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &current }
	scan := map[string]interface{}{"operation": "scan", "input_ref": map[string]interface{}{"root": "desktop", "path": "Documents/test.png"}}
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, scan)); got["status"] != "error" {
		t.Fatal("revoked Desktop read returned QR content")
	}
	if _, err := qrReadReference(context.Background(), cfg, &qrFileRef{Root: "desktop", Path: "Documents/test.png"}, qrcode.MaxResourceBytes); err == nil {
		t.Fatal("Office image/config/logo reference bypassed live read revocation")
	}
}

func TestQRCodeExplicitConfigurationExportAndSVGReference(t *testing.T) {
	cfg := qrTestConfig(t)
	args := qrTestArgs("saved.qr.json")
	args["export_config"] = true
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "ok" {
		t.Fatal(got)
	}
	data, err := os.ReadFile(filepath.Join(cfg.Directories.WorkspaceDir, "saved.qr.json"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := qrcode.ParseConfig(context.Background(), data)
	if err != nil || spec.Content.Text != "Grüße QR — exact content" {
		t.Fatalf("config=%+v err=%v", spec, err)
	}
	reload := map[string]interface{}{"operation": "generate", "config_ref": map[string]interface{}{"root": "workspace", "path": "saved.qr.json"}, "output_path": "reloaded.png"}
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, reload)); got["status"] != "ok" {
		t.Fatal(got)
	}
	cfg.Tools.QRCode.ReadOnly = true
	args["output_path"] = "blocked.qr.json"
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "error" {
		t.Fatal("readonly configuration export succeeded")
	}
	if err := os.WriteFile(filepath.Join(cfg.Directories.WorkspaceDir, "logo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="#aa2222"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, err := qrResolveSpec(context.Background(), cfg, qrToolArgs{Spec: &spec, LogoRef: &qrFileRef{Root: "workspace", Path: "logo.svg"}})
	if err != nil || resolved.Logo == nil || resolved.Logo.Image.MIME != "image/png" {
		t.Fatalf("static SVG reference normalization: %v", err)
	}
}

func TestQRCodePresetRetainsEPCAndExplicitCorrection(t *testing.T) {
	cfg := qrTestConfig(t)
	args := map[string]interface{}{
		"operation": "generate", "preset": "soft", "output_path": "payment.png",
		"spec": map[string]interface{}{"content": map[string]interface{}{"type": "epc", "epc": map[string]interface{}{"version": "002", "name": "Example", "iban": "DE89370400440532013000", "amount": "1.00"}}},
	}
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "ok" {
		t.Fatal("presentation preset overrode EPC correction M: ", got)
	}
	spec, err := qrResolveSpec(context.Background(), cfg, qrToolArgs{Preset: "label", Spec: &qrcode.QRSpec{Content: qrcode.Content{Type: "text", Text: "payload"}, Style: qrcode.Style{ErrorCorrection: "Q"}, Frame: &qrcode.Frame{Text: "Retain caption"}}})
	if err != nil || spec.Style.ErrorCorrection != "Q" || spec.Frame == nil || spec.Frame.Text != "Retain caption" {
		t.Fatalf("preset lost explicit correction/caption: %+v %v", spec, err)
	}
}
