package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

func TestOpenSCADDockerAdapterUsesJobRootForBindValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := desktop.Config{
		WorkspaceDir: filepath.Join(root, "desktop"),
		DataDir:      filepath.Join(root, "data"),
		DBPath:       filepath.Join(root, "virtual_desktop.db"),
	}

	codeAdapter := newCodeStudioDockerAdapter(cfg, nil)
	if codeAdapter.cfg.WorkspaceDir != cfg.WorkspaceDir {
		t.Fatalf("code studio WorkspaceDir = %q, want %q", codeAdapter.cfg.WorkspaceDir, cfg.WorkspaceDir)
	}

	openSCADAdapter := newOpenSCADDockerAdapter(cfg, nil)
	want := filepath.Join(cfg.DataDir, "openscad", "jobs")
	if openSCADAdapter.cfg.WorkspaceDir != want {
		t.Fatalf("openscad WorkspaceDir = %q, want %q", openSCADAdapter.cfg.WorkspaceDir, want)
	}
}

// The Code Studio workspace check compares the bind source with the host
// workspace path, so the adapter must see the full source even though the
// agent- and UI-facing docker inspect shortens bind sources.
func TestCodeStudioDockerAdapterInspectKeepsBindSource(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			io.WriteString(w, `{"ApiVersion":"1.45"}`)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/containers/code-studio/json") {
			t.Errorf("unexpected Docker request %s", r.URL.Path)
		}
		io.WriteString(w, `{"Id":"abc","Name":"/code-studio","State":{"Running":true,"Status":"running"},`+
			`"Mounts":[{"Type":"bind","Source":"/home/aurago/workspace/code","Destination":"/workspace","RW":true}]}`)
	}))
	defer srv.Close()

	adapter := codeStudioDockerAdapter{cfg: tools.DockerConfig{Host: "tcp://" + strings.TrimPrefix(srv.URL, "http://")}}
	inspect, err := adapter.InspectContainer(context.Background(), "code-studio")
	if err != nil {
		t.Fatalf("InspectContainer: %v", err)
	}
	if len(inspect.Mounts) != 1 || inspect.Mounts[0].Source != "/home/aurago/workspace/code" || inspect.Mounts[0].Destination != "/workspace" {
		t.Fatalf("mounts = %+v, want the full workspace bind", inspect.Mounts)
	}
	if !inspect.State.Running {
		t.Fatalf("state = %+v, want running", inspect.State)
	}
}
