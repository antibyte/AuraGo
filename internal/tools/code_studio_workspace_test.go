package tools

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestCodeStudioWorkspacePathMapping(t *testing.T) {
	ok := []struct {
		in        string
		allowRoot bool
		want      string
	}{
		{"src/main.go", false, "/workspace/src/main.go"},
		{"/workspace/src/main.go", false, "/workspace/src/main.go"},
		{"./src//main.go", false, "/workspace/src/main.go"},
		{`src\win.go`, false, "/workspace/src/win.go"},
		{"", true, "/workspace"},
		{"/workspace", true, "/workspace"},
		{".", true, "/workspace"},
	}
	for _, tc := range ok {
		got, err := codeStudioWorkspacePath(tc.in, tc.allowRoot)
		if err != nil || got != tc.want {
			t.Errorf("codeStudioWorkspacePath(%q, %v) = %q, %v; want %q", tc.in, tc.allowRoot, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "/etc/passwd", "/workspacex/a.go", "src/../../x", "../x", "~/x", "C:/x"} {
		if got, err := codeStudioWorkspacePath(bad, false); err == nil {
			t.Errorf("codeStudioWorkspacePath(%q) = %q, want an error", bad, got)
		}
	}
	if got := codeStudioWorkspaceRel("/workspace/src/a.go"); got != "src/a.go" {
		t.Fatalf("rel = %q", got)
	}
	if got := codeStudioWorkspaceRel("/workspace"); got != "." {
		t.Fatalf("rel root = %q", got)
	}
}

type fakeCodeStudioWorkspace struct {
	files       map[string]string // virtual path -> content
	search      []CodeStudioWorkspaceMatch
	execOut     CodeStudioWorkspaceExecResult
	execCmd     string
	execCwd     string
	execTimeout time.Duration
}

func (w *fakeCodeStudioWorkspace) Info() CodeStudioWorkspaceInfo {
	return CodeStudioWorkspaceInfo{ID: "ws_test", Label: "api", Host: "dev@build-box", Root: "/home/dev/api"}
}
func (w *fakeCodeStudioWorkspace) List(_ context.Context, vdir string) ([]CodeStudioWorkspaceEntry, error) {
	var out []CodeStudioWorkspaceEntry
	for p := range w.files {
		if path.Dir(p) == vdir {
			out = append(out, CodeStudioWorkspaceEntry{Name: path.Base(p), Path: p, Type: "file"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func (w *fakeCodeStudioWorkspace) Read(_ context.Context, vpath string, max int64) ([]byte, error) {
	content, ok := w.files[vpath]
	if !ok {
		return nil, errors.New("file not found")
	}
	if int64(len(content)) > max {
		return nil, errors.New("file too large")
	}
	return []byte(content), nil
}
func (w *fakeCodeStudioWorkspace) Write(_ context.Context, vpath string, content []byte) error {
	w.files[vpath] = string(content)
	return nil
}
func (w *fakeCodeStudioWorkspace) Search(context.Context, CodeStudioWorkspaceSearch) ([]CodeStudioWorkspaceMatch, bool, error) {
	return w.search, false, nil
}
func (w *fakeCodeStudioWorkspace) Exec(_ context.Context, command, vcwd string, timeout time.Duration) (CodeStudioWorkspaceExecResult, error) {
	w.execCmd, w.execCwd, w.execTimeout = command, vcwd, timeout
	return w.execOut, nil
}

type fakeCodeStudioProvider struct {
	ws       *fakeCodeStudioWorkspace
	readOnly bool
	changed  [][]string
}

func (p *fakeCodeStudioProvider) Workspace(_ context.Context, id string) (CodeStudioWorkspace, error) {
	switch id {
	case "local":
		return nil, ErrCodeStudioWorkspaceLocal
	case "ws_test":
		return p.ws, nil
	}
	return nil, ErrCodeStudioWorkspaceNotFound
}
func (p *fakeCodeStudioProvider) Changed(id string, vpaths ...string) {
	p.changed = append(p.changed, append([]string{id}, vpaths...))
}
func (p *fakeCodeStudioProvider) Limits() CodeStudioWorkspaceLimits {
	return CodeStudioWorkspaceLimits{MaxFileBytes: 1 << 20, ReadOnly: p.readOnly}
}

func useFakeCodeStudioProvider(t *testing.T) *fakeCodeStudioProvider {
	t.Helper()
	p := &fakeCodeStudioProvider{ws: &fakeCodeStudioWorkspace{files: map[string]string{
		"/workspace/src/main.go": "package main\n\nfunc main() {}\n",
	}}}
	previous := currentCodeStudioWorkspaceProvider()
	SetCodeStudioWorkspaceProvider(p)
	t.Cleanup(func() { SetCodeStudioWorkspaceProvider(previous) })
	return p
}

func codeStudioWorkspaceTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

type codeStudioWorkspaceTestResult struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func runCodeStudioWorkspaceTool(t *testing.T, cfg *config.Config, args map[string]interface{}) (codeStudioWorkspaceTestResult, VirtualDesktopExecution) {
	t.Helper()
	exec := ExecuteCodeStudioWorkspace(context.Background(), cfg, args)
	var res codeStudioWorkspaceTestResult
	if err := json.Unmarshal([]byte(exec.Output), &res); err != nil {
		t.Fatalf("output %q: %v", exec.Output, err)
	}
	return res, exec
}

func TestCodeStudioWorkspaceToolReadsWritesAndPatches(t *testing.T) {
	p := useFakeCodeStudioProvider(t)
	cfg := codeStudioWorkspaceTestConfig()

	res, _ := runCodeStudioWorkspaceTool(t, cfg, map[string]interface{}{"operation": "read_file", "workspace_id": "ws_test", "path": "/workspace/src/main.go"})
	if res.Status != "ok" || res.Data["content"] != "package main\n\nfunc main() {}\n" || res.Data["path"] != "src/main.go" {
		t.Fatalf("read_file = %+v", res)
	}

	res, exec := runCodeStudioWorkspaceTool(t, cfg, map[string]interface{}{"operation": "write_file", "workspace_id": "ws_test", "path": "src/util.go", "content": "package main\n"})
	if res.Status != "ok" || p.ws.files["/workspace/src/util.go"] != "package main\n" {
		t.Fatalf("write_file = %+v, files %v", res, p.ws.files)
	}
	if exec.Event == nil || exec.Event.Type != "code_studio_file_changed" {
		t.Fatalf("event = %+v", exec.Event)
	}
	if payload, _ := exec.Event.Payload.(map[string]interface{}); payload["workspace_id"] != "ws_test" || payload["path"] != "/workspace/src/util.go" {
		t.Fatalf("event payload = %#v", exec.Event.Payload)
	}
	if len(p.changed) != 1 || !slices.Equal(p.changed[0], []string{"ws_test", "/workspace/src/util.go"}) {
		t.Fatalf("changed = %v", p.changed)
	}

	res, _ = runCodeStudioWorkspaceTool(t, cfg, map[string]interface{}{"operation": "patch_file", "workspace_id": "ws_test", "path": "src/main.go", "find": "func main() {}", "replace": "func main() { run() }"})
	if res.Status != "ok" || p.ws.files["/workspace/src/main.go"] != "package main\n\nfunc main() { run() }\n" {
		t.Fatalf("patch_file = %+v", res)
	}

	p.ws.files["/workspace/twice.txt"] = "a\na\n"
	res, _ = runCodeStudioWorkspaceTool(t, cfg, map[string]interface{}{"operation": "patch_file", "workspace_id": "ws_test", "path": "twice.txt", "replacements": []interface{}{map[string]interface{}{"find": "a", "replace": "b"}}})
	if res.Status != "error" || !strings.Contains(res.Message, "occurs 2 times") || p.ws.files["/workspace/twice.txt"] != "a\na\n" {
		t.Fatalf("ambiguous patch = %+v, file %q", res, p.ws.files["/workspace/twice.txt"])
	}
}

func TestCodeStudioWorkspaceToolRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(cfg *config.Config, p *fakeCodeStudioProvider)
		args   map[string]interface{}
		want   string
	}{
		{"agent control off", func(cfg *config.Config, _ *fakeCodeStudioProvider) { cfg.VirtualDesktop.AllowAgentControl = false },
			map[string]interface{}{"operation": "read_file", "workspace_id": "ws_test", "path": "src/main.go"}, "agent control"},
		{"read-only config blocks writes", func(cfg *config.Config, _ *fakeCodeStudioProvider) { cfg.VirtualDesktop.ReadOnly = true },
			map[string]interface{}{"operation": "write_file", "workspace_id": "ws_test", "path": "x.go", "content": "x"}, "read-only"},
		{"live read-only blocks commands", func(_ *config.Config, p *fakeCodeStudioProvider) { p.readOnly = true },
			map[string]interface{}{"operation": "run_command", "workspace_id": "ws_test", "command": "rm -rf ."}, "read-only"},
		{"local workspace", nil, map[string]interface{}{"operation": "read_file", "workspace_id": "local", "path": "a"}, "virtual_desktop_files"},
		{"unknown workspace", nil, map[string]interface{}{"operation": "read_file", "workspace_id": "ws_nope", "path": "a"}, "does not exist"},
		{"missing workspace id", nil, map[string]interface{}{"operation": "read_file", "path": "a"}, "workspace_id is required"},
		{"parent path", nil, map[string]interface{}{"operation": "write_file", "workspace_id": "ws_test", "path": "../x", "content": "x"}, "leaves the workspace"},
		{"absolute path", nil, map[string]interface{}{"operation": "read_file", "workspace_id": "ws_test", "path": "/etc/passwd"}, "relative to the workspace root"},
		{"unknown operation", nil, map[string]interface{}{"operation": "delete_file", "workspace_id": "ws_test"}, "unsupported operation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := useFakeCodeStudioProvider(t)
			cfg := codeStudioWorkspaceTestConfig()
			if tc.mutate != nil {
				tc.mutate(cfg, p)
			}
			res, exec := runCodeStudioWorkspaceTool(t, cfg, tc.args)
			if res.Status != "error" || !strings.Contains(res.Message, tc.want) {
				t.Fatalf("result = %+v; want an error containing %q", res, tc.want)
			}
			if exec.Event != nil || len(p.changed) != 0 || p.ws.execCmd != "" || len(p.ws.files) != 1 {
				t.Fatalf("a refused call had effects: event=%v changed=%v exec=%q files=%v", exec.Event, p.changed, p.ws.execCmd, p.ws.files)
			}
		})
	}
	// Read-only still lets the agent read.
	useFakeCodeStudioProvider(t)
	cfg := codeStudioWorkspaceTestConfig()
	cfg.VirtualDesktop.ReadOnly = true
	if res, _ := runCodeStudioWorkspaceTool(t, cfg, map[string]interface{}{"operation": "read_file", "workspace_id": "ws_test", "path": "src/main.go"}); res.Status != "ok" {
		t.Fatalf("read-only read = %+v", res)
	}
}

func TestCodeStudioWorkspaceToolRunCommandClampsAndCaps(t *testing.T) {
	p := useFakeCodeStudioProvider(t)
	p.ws.execOut = CodeStudioWorkspaceExecResult{ExitCode: 2, Output: strings.Repeat("x", 100<<10)}
	res, exec := runCodeStudioWorkspaceTool(t, codeStudioWorkspaceTestConfig(), map[string]interface{}{
		"operation": "run_command", "workspace_id": "ws_test", "command": "go test ./...", "cwd": "cmd/api", "timeout_seconds": 999,
	})
	if res.Status != "ok" || res.Message != "command exited with code 2" {
		t.Fatalf("run_command = %+v", res)
	}
	if p.ws.execCmd != "go test ./..." || p.ws.execCwd != "/workspace/cmd/api" || p.ws.execTimeout != 120*time.Second {
		t.Fatalf("exec = %q in %q for %v", p.ws.execCmd, p.ws.execCwd, p.ws.execTimeout)
	}
	if out, _ := res.Data["output"].(string); res.Data["output_truncated"] != true || len(out) > 49<<10 {
		t.Fatalf("output %d bytes, truncated %v", len(out), res.Data["output_truncated"])
	}
	if len(p.changed) != 1 || !slices.Equal(p.changed[0], []string{"ws_test"}) {
		t.Fatalf("changed = %v; want the workspace without paths", p.changed)
	}
	if payload, _ := exec.Event.Payload.(map[string]interface{}); payload["path"] != "" {
		t.Fatalf("event payload = %#v", exec.Event.Payload)
	}
}
