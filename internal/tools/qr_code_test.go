package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/office"
	"aurago/internal/qrcode"
)

func qrTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.QRCode.Enabled = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = t.TempDir()
	return cfg
}

func qrTestArgs(output string) map[string]interface{} {
	return map[string]interface{}{"operation": "generate", "output_path": output, "spec": map[string]interface{}{"schema_version": 1, "content": map[string]interface{}{"type": "text", "text": "Grüße QR — exact content"}, "output": map[string]interface{}{"format": "png"}}}
}

func qrTestPayload(t *testing.T, exec VirtualDesktopExecution) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(exec.Output), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestQRCodeWorkspaceRoundtripAndConditionalPublication(t *testing.T) {
	cfg := qrTestConfig(t)
	args := qrTestArgs("codes/test.png")
	first := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args))
	if first["status"] != "ok" {
		t.Fatal(first)
	}
	data, err := os.ReadFile(filepath.Join(cfg.Directories.WorkspaceDir, "codes/test.png"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := qrcode.Decode(context.Background(), data)
	if err != nil || decoded.Text != "Grüße QR — exact content" {
		t.Fatalf("decode=%+v err=%v", decoded, err)
	}
	if strings.Contains(first["data"].(map[string]interface{})["artifact"].(map[string]interface{})["path"].(string), cfg.Directories.WorkspaceDir) {
		t.Fatal("host path exposed")
	}
	second := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args))
	if second["status"] != "error" {
		t.Fatal("create-only overwrite succeeded")
	}
	args["expected_version"] = strings.Repeat("0", 64)
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "error" {
		t.Fatal("stale replacement succeeded")
	}
	after, _ := os.ReadFile(filepath.Join(cfg.Directories.WorkspaceDir, "codes/test.png"))
	if !bytes.Equal(data, after) {
		t.Fatal("failed operation changed existing file")
	}
	args["expected_version"] = first["data"].(map[string]interface{})["version"]
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "ok" {
		t.Fatal(got)
	}
	cfg.Tools.QRCode.ReadOnly = true
	scan := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, map[string]interface{}{"operation": "scan", "input_ref": map[string]interface{}{"root": "workspace", "path": "codes/test.png"}}))
	if scan["status"] != "ok" || scan["data"].(map[string]interface{})["text"] != decoded.Text {
		t.Fatal(scan)
	}
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, args)); got["status"] != "error" {
		t.Fatal("readonly generation succeeded")
	}
}

func TestQRCodeBoundariesAndRevocation(t *testing.T) {
	cfg := qrTestConfig(t)
	for _, p := range []string{"../escaped.png", filepath.Join(t.TempDir(), "absolute.png")} {
		if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, qrTestArgs(p))); got["status"] != "error" {
			t.Fatalf("escaped %q", p)
		}
	}
	cfg.Tools.QRCode.Enabled = false
	if got := qrTestPayload(t, ExecuteQRCode(context.Background(), cfg, map[string]interface{}{"operation": "presets"})); got["status"] != "error" {
		t.Fatal("disabled tool visible")
	}
	cfg.Tools.QRCode.Enabled = true
	current := *cfg
	current.Tools.QRCode.ReadOnly = true
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &current }
	if _, _, err := qrPublish(context.Background(), cfg, qrFileRef{Root: "workspace", Path: "revoked.png"}, []byte("complete"), ""); err == nil {
		t.Fatal("revoked publication succeeded")
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.WorkspaceDir, "revoked.png")); !os.IsNotExist(err) {
		t.Fatal("revoked output exists")
	}
	cfg.AuthorizationSnapshots = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := qrPublish(ctx, cfg, qrFileRef{Root: "workspace", Path: "cancelled.png"}, []byte("complete"), ""); err == nil {
		t.Fatal("cancelled publication succeeded")
	}
	for _, url := range []string{"file:///etc/passwd", "http://127.0.0.1/private", "http://[::1]/", "http://169.254.169.254/latest/meta-data/"} {
		if _, err := ImportQRImage(context.Background(), url); err == nil {
			t.Fatalf("unsafe import %s", url)
		}
	}
}

func TestQRCodeAgentOfficeInsertPreservesVersion(t *testing.T) {
	cfg := qrTestConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	cfg.Tools.OfficeWorkbook.Enabled = true
	ctx := context.Background()
	qrArgs := qrTestArgs("Documents/code.png")
	qrArgs["output_root"] = "desktop"
	if got := qrTestPayload(t, ExecuteQRCode(ctx, cfg, qrArgs)); got["status"] != "ok" {
		t.Fatal(got)
	}
	svc, cleanup, err := getToolDesktopService(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	doc, _, err := office.EncodeDocument("report.docx", office.Document{Title: "Original", Text: "Retain this paragraph"})
	if err != nil {
		t.Fatal(err)
	}
	book, err := office.EncodeWorkbook(office.Workbook{Sheets: []office.Sheet{{Name: "Sheet1", Rows: [][]office.Cell{{{Value: "retain"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		data  []byte
		run   func(context.Context, *config.Config, map[string]interface{}) VirtualDesktopExecution
		extra map[string]interface{}
	}{
		{"Documents/report.docx", doc, ExecuteOfficeDocument, map[string]interface{}{"anchor": "end"}},
		{"Documents/REPORT.DOCX", doc, ExecuteOfficeDocument, map[string]interface{}{"anchor": "end"}},
		{"Documents/table.xlsx", book, ExecuteOfficeWorkbook, map[string]interface{}{"anchor": "C4", "sheet": "Sheet1"}},
	} {
		if err = svc.WriteFileBytes(ctx, test.name, test.data, desktop.SourceAgent); err != nil {
			t.Fatal(err)
		}
		_, entry, err := svc.ReadFileBytes(ctx, test.name)
		if err != nil {
			t.Fatal(err)
		}
		args := map[string]interface{}{"operation": "insert_image", "path": test.name, "image_ref": map[string]interface{}{"root": "desktop", "path": "Documents/code.png"}, "expected_version": officeToolVersionForEntry(entry, test.data).ETag, "width": 256, "height": 256, "alt": "QR code"}
		for key, value := range test.extra {
			args[key] = value
		}
		result := qrTestPayload(t, test.run(ctx, cfg, args))
		if result["status"] != "ok" {
			t.Fatal(result)
		}
		inserted, _, err := svc.ReadFileBytes(ctx, test.name)
		if err != nil {
			t.Fatal(err)
		}
		z, err := zip.NewReader(bytes.NewReader(inserted), int64(len(inserted)))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, part := range z.File {
			if strings.Contains(part.Name, "/media/") {
				r, _ := part.Open()
				b, _ := io.ReadAll(r)
				r.Close()
				if qr, err := qrcode.Decode(ctx, b); err == nil && qr.Text == "Grüße QR — exact content" {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("Office output lost QR pixels")
		}
		if got := qrTestPayload(t, test.run(ctx, cfg, args)); got["status"] != "error" {
			t.Fatal("stale Office overwrite succeeded")
		}
		after, _, _ := svc.ReadFileBytes(ctx, test.name)
		if !bytes.Equal(after, inserted) {
			t.Fatal("conflict altered Office file")
		}
	}
}
