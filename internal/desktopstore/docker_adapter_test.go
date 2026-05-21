package desktopstore

import (
	"archive/tar"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/tools"
)

func TestToolsDockerAdapterCopyToContainerUsesArchiveEndpoint(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	var sawArchive bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.45/containers/aurago-store-olivetin/archive" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if got := r.URL.Query().Get("path"); got != "/config" {
			t.Fatalf("path query = %q, want /config", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-tar" {
			t.Fatalf("content type = %q, want application/x-tar", got)
		}
		tr := tar.NewReader(r.Body)
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("read tar header: %v", err)
		}
		if header.Name != "config.yaml" {
			t.Fatalf("tar entry = %q, want config.yaml", header.Name)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar body: %v", err)
		}
		if !strings.Contains(string(body), `title: "Hello world!"`) {
			t.Fatalf("config body = %q", string(body))
		}
		sawArchive = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), "", nil)
	if err := adapter.CopyToContainer(context.Background(), "aurago-store-olivetin", "/config", map[string]string{
		"config.yaml": oliveTinDefaultConfig,
	}); err != nil {
		t.Fatalf("copy to container: %v", err)
	}
	if !sawArchive {
		t.Fatal("archive endpoint was not called")
	}
}
