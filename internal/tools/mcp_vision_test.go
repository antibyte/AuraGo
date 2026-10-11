package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMCPVisionImageArgumentsMatchRuntime(t *testing.T) {
	raw := []byte("\x89PNG\r\n\x1a\nfixture")
	image := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(image, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	text := map[string]interface{}{"type": "string"}
	for _, tc := range []struct {
		name, runtime, key string
		props              map[string]interface{}
		denied             bool
	}{
		{"base64 wins", "streamable_http", "image_base64", map[string]interface{}{"image_base64": text, "image_url": text, "path": text}, false},
		{"data URL", "websocket", "imageUrl", map[string]interface{}{"imageUrl": map[string]interface{}{"type": "string", "format": "uri"}}, false},
		{"HTTP only URL", "sse", "", map[string]interface{}{"url": map[string]interface{}{"type": "string", "pattern": "^https://"}}, true},
		{"remote path", "streamable_http", "", map[string]interface{}{"image_path": text}, true},
		{"local path", "local", "image_path", map[string]interface{}{"image_path": text}, false},
		{"Docker path", "docker", "image_path", map[string]interface{}{"image_path": text}, false},
		{"no declared input", "local", "", nil, true},
		{"external reference", "streamable_http", "", map[string]interface{}{"image_url": map[string]interface{}{"$ref": "http://127.0.0.1:1/schema"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mount := t.TempDir()
			tool := &MCPToolInfo{Name: "vision", InputSchema: map[string]interface{}{"properties": tc.props}, connection: &mcpConn{runtime: tc.runtime, hostDir: mount, contDir: "/workspace"}}
			args, cleanup, err := buildPreferredMCPVisionArgs(context.Background(), tool, image, "describe")
			if tc.denied {
				if err == nil {
					cleanup()
					t.Fatal("unsupported input accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			got := args[tc.key].(string)
			switch tc.key {
			case "image_base64":
				if got != base64.StdEncoding.EncodeToString(raw) {
					t.Fatal("base64 changed")
				}
			case "imageUrl":
				if got != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw) {
					t.Fatal("incorrect data URL")
				}
			default:
				host := got
				if tc.runtime == "docker" {
					if !strings.HasPrefix(got, "/workspace/aurago-mcp-vision-") {
						t.Fatalf("container path=%s", got)
					}
					host = filepath.Join(mount, filepath.FromSlash(strings.TrimPrefix(got, "/workspace/")))
				}
				data, err := os.ReadFile(host)
				if err != nil || string(data) != string(raw) {
					t.Fatalf("staging: %v", err)
				}
				cleanup()
				cleanup = func() {}
				if _, err := os.Stat(filepath.Dir(host)); !os.IsNotExist(err) {
					t.Fatal("staging directory leaked")
				}
			}
		})
	}
}

func TestMCPVisionRejectsInvalidAndOversizedImages(t *testing.T) {
	file := filepath.Join(t.TempDir(), "image.png")
	tool := &MCPToolInfo{connection: &mcpConn{runtime: "local"}}
	if err := os.WriteFile(file, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildPreferredMCPVisionArgs(context.Background(), tool, file, ""); err == nil || !strings.Contains(err.Error(), "image type") {
		t.Fatalf("MIME: %v", err)
	}
	f, err := os.OpenFile(file, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(maxGeneratedImageBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, _, err := buildPreferredMCPVisionArgs(context.Background(), tool, file, ""); err == nil || !strings.Contains(err.Error(), "50 MiB") {
		t.Fatalf("size: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := buildPreferredMCPVisionArgs(ctx, tool, file, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestMCPVisionCleanupAfterCall(t *testing.T) {
	oldList, oldCall := listPreferredMCPTools, callPreferredMCPTool
	defer func() { listPreferredMCPTools, callPreferredMCPTool = oldList, oldCall }()
	for _, outcome := range []string{"success", "error", "cancel"} {
		t.Run(outcome, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.AllowMCP = true
			cfg.MCP.Enabled = true
			cfg.MCP.PreferredCapabilities.Vision = config.MCPPreferredToolSelection{Server: "s", Tool: "vision"}
			cfg.Directories.WorkspaceDir = t.TempDir()
			file := filepath.Join(cfg.Directories.WorkspaceDir, "image.png")
			if err := os.WriteFile(file, []byte("\x89PNG\r\n\x1a\nfixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			mount := t.TempDir()
			listPreferredMCPTools = func(context.Context, string, *slog.Logger) ([]MCPToolInfo, error) {
				return []MCPToolInfo{{Name: "vision", connection: &mcpConn{runtime: "docker", hostDir: mount, contDir: "/workspace"}, InputSchema: map[string]interface{}{"properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}}}}}, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := false
			callPreferredMCPTool = func(ctx context.Context, _, _ string, args map[string]interface{}, _ *slog.Logger) (string, error) {
				called = true
				staged := filepath.Join(mount, filepath.FromSlash(strings.TrimPrefix(args["path"].(string), "/workspace/")))
				if _, err := os.Stat(staged); err != nil {
					t.Fatal(err)
				}
				if outcome == "error" {
					return "", errors.New("fixture failed")
				}
				if outcome == "cancel" {
					cancel()
					return "", ctx.Err()
				}
				return "ok", nil
			}
			_, used, err := CallPreferredMCPVision(ctx, cfg, file, "describe", testLogger())
			if !called || !used || (err != nil) != (outcome != "success") {
				t.Fatalf("call: used=%v err=%v", used, err)
			}
			entries, err := os.ReadDir(mount)
			if err != nil || len(entries) != 0 {
				t.Fatalf("staging leaked: %v", err)
			}
		})
	}
}

func TestMCPPreferredDiscoveryUsesCallerContext(t *testing.T) {
	old := listPreferredMCPTools
	defer func() { listPreferredMCPTools = old }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	listPreferredMCPTools = func(got context.Context, _ string, _ *slog.Logger) ([]MCPToolInfo, error) {
		if got != ctx {
			t.Fatal("context replaced")
		}
		return nil, got.Err()
	}
	if _, err := findPreferredMCPTool(ctx, config.MCPPreferredToolSelection{Server: "s", Tool: "vision"}, testLogger()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestMCPPreferredCallRejectsReplacedConnection(t *testing.T) {
	oldInvoke := invokeMCPConnTool
	defer func() { invokeMCPConnTool = oldInvoke }()
	invokeMCPConnTool = func(context.Context, *mcpConn, string, map[string]interface{}) (string, error) {
		t.Fatal("dispatched after connection changed")
		return "", nil
	}
	previous, current := &mcpConn{ready: true}, &mcpConn{ready: true}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{"s": current}, configs: map[string]MCPServerConfig{"s": {Name: "s", Enabled: true, Command: "fixture"}}}
	defer mgr.Close()
	if _, err := mgr.callTool(context.Background(), "s", "vision", nil, previous); err == nil {
		t.Fatal("changed connection accepted")
	}
}

func TestMCPDiscoveryPreservesExplicitDeadline(t *testing.T) {
	oldStart := startManagedMCPConn
	defer func() { startManagedMCPConn = oldStart; ShutdownMCPManager() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	deadline, _ := ctx.Deadline()
	mgr := InitMCPManager(nil, testLogger())
	mgr.configs["s"] = MCPServerConfig{Name: "s", Enabled: true, Command: "fixture"}
	startManagedMCPConn = func(got context.Context, _ MCPServerConfig, _ *slog.Logger) (*mcpConn, error) {
		if actual, _ := got.Deadline(); !actual.Equal(deadline) {
			t.Fatal("discovery shortened caller deadline")
		}
		return &mcpConn{ready: true}, nil
	}
	if _, err := MCPListToolsContext(ctx, "s", testLogger()); err != nil {
		t.Fatal(err)
	}
}
