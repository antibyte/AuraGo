package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func mcpArgumentTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.MCPServer.Enabled = true
	cfg.MCPServer.AllowedTools = []string{"invoke_tool", "text_diff"}
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestMCPCallToolInvokeArguments(t *testing.T) {
	args := map[string]interface{}{
		"operation": "diff_strings",
		"text1":     "alpha\nbeta\n",
		"text2":     "alpha\nBETA\n",
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]interface{}{"object": args, "json_string": string(encoded)} {
		t.Run(name, func(t *testing.T) {
			s := mcpArgumentTestServer(t)
			params, err := json.Marshal(mcpCallToolParams{Name: "invoke_tool", Arguments: map[string]interface{}{
				"tool_name": "text_diff", "arguments": value,
			}})
			if err != nil {
				t.Fatal(err)
			}
			got, rpcErr := mcpCallTool(context.Background(), s, params)
			if rpcErr != nil || got.IsError || len(got.Content) != 1 {
				t.Fatalf("invoke failed: result=%+v protocol_error=%+v", got, rpcErr)
			}
			for _, want := range []string{"-beta", "+BETA", "alpha"} {
				if !strings.Contains(got.Content[0].Text, want) {
					t.Fatalf("diff missing %q: %s", want, got.Content[0].Text)
				}
			}
		})
	}
}

func TestMCPCallToolInvokeJSONStringCannotWidenScope(t *testing.T) {
	s := mcpArgumentTestServer(t)
	s.Cfg.Agent.AllowFilesystemWrite = true
	path := filepath.Join(s.Cfg.Directories.WorkspaceDir, "ungranted.txt")
	args, err := json.Marshal(map[string]interface{}{"operation": "write_file", "file_path": path, "content": "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(mcpCallToolParams{Name: "invoke_tool", Arguments: map[string]interface{}{
		"tool_name": "filesystem", "arguments": string(args),
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, rpcErr := mcpCallTool(context.Background(), s, params)
	if rpcErr != nil || !got.IsError {
		t.Fatalf("unexported target was not denied: result=%+v protocol_error=%+v", got, rpcErr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("denied target produced a file: %v", err)
	}
}

func TestMCPCallToolRejectsInvalidInvokeArgumentObjects(t *testing.T) {
	for _, value := range []interface{}{nil, false, 42, []interface{}{}, "", "null", "[]", "false", "{broken", "{} {}"} {
		t.Run(strings.ReplaceAll(string(mustMCPJSON(t, value)), "/", "_"), func(t *testing.T) {
			s := mcpArgumentTestServer(t)
			params := mustMCPJSON(t, mcpCallToolParams{Name: "invoke_tool", Arguments: map[string]interface{}{
				"tool_name": "text_diff", "arguments": value,
			}})
			_, rpcErr := mcpCallTool(context.Background(), s, params)
			if rpcErr == nil || rpcErr.Code != -32602 {
				t.Fatalf("invalid argument object was accepted: %#v", rpcErr)
			}
		})
	}
}

func mustMCPJSON(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
