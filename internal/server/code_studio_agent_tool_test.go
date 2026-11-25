package server

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"aurago/internal/tools"
)

type agentToolTestBackend struct {
	codeStudioBackend // nil: only Write and Exec are used
	writes            []string
	argv              []string
	cwd               string
}

func (b *agentToolTestBackend) Write(_ context.Context, vpath string, _ []byte, createOnly bool) error {
	if createOnly {
		return errors.New("agent writes must not be create-only")
	}
	b.writes = append(b.writes, vpath)
	return nil
}

func (b *agentToolTestBackend) Exec(_ context.Context, argv []string, vcwd string, _ []byte, _ time.Duration) (codeStudioExecResult, error) {
	b.argv, b.cwd = argv, vcwd
	return codeStudioExecResult{ExitCode: 0, Output: "ok\n"}, nil
}

func TestCodeStudioAgentWorkspacesRefuseTheLocalWorkspace(t *testing.T) {
	p := codeStudioAgentWorkspaces{s: testCodeStudioServer(t)}
	for _, id := range []string{"", "local", "  local "} {
		if _, err := p.Workspace(context.Background(), id); !errors.Is(err, tools.ErrCodeStudioWorkspaceLocal) {
			t.Fatalf("Workspace(%q) err = %v; want ErrCodeStudioWorkspaceLocal", id, err)
		}
	}
}

func TestCodeStudioAgentWorkspaceMapsToTheBackend(t *testing.T) {
	backend := &agentToolTestBackend{}
	ws := codeStudioAgentWorkspace{backend: backend, info: codeStudioWorkspaceInfo{ID: "ws_7f3k", Kind: codeStudioWorkspaceKindSSH, Host: "build-box", User: "dev", Root: "/home/dev/api"}}
	if info := ws.Info(); info.Host != "dev@build-box" || info.Root != "/home/dev/api" {
		t.Fatalf("info = %+v", info)
	}
	if err := ws.Write(context.Background(), "/workspace/src/a.go", []byte("x")); err != nil || !slices.Equal(backend.writes, []string{"/workspace/src/a.go"}) {
		t.Fatalf("write err = %v, writes %v", err, backend.writes)
	}
	if err := ws.Write(context.Background(), "/etc/passwd", []byte("x")); err == nil {
		t.Fatal("a path outside /workspace reached the backend")
	}
	if _, err := ws.Exec(context.Background(), "go test ./...", "/workspace/cmd", time.Minute); err != nil ||
		!slices.Equal(backend.argv, []string{"/bin/sh", "-c", "go test ./..."}) || backend.cwd != "/workspace/cmd" {
		t.Fatalf("exec err = %v argv %q cwd %q", err, backend.argv, backend.cwd)
	}
}
