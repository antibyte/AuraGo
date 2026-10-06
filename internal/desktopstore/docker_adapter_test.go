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
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
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

func TestToolsDockerAdapterTrustsOnlyCatalogHostBinds(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	var created []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/v1.45/containers/create" {
			created = append(created, r.URL.Query().Get("name"))
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Id":"created-id"}`)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), t.TempDir(), nil)
	socket := HostBinding{HostPath: "/var/run/docker.sock", ContainerPath: "/var/run/docker.sock", ReadOnly: true}
	labels := func(appID, companion string) map[string]string {
		out := map[string]string{"aurago.desktop_store": "true", "aurago.desktop_store.app_id": appID}
		if companion != "" {
			out["aurago.desktop_store.companion"] = companion
		}
		return out
	}
	ctx := context.Background()

	// Every host bind the code catalog declares, resolved the way install and
	// update resolve them, must create (Dozzle, the Beszel agent and the Arcane
	// socket proxy mount the read-only Docker socket).
	var catalogSpecs []ContainerSpec
	for _, entry := range DefaultCatalog() {
		if len(entry.HostBinds) > 0 {
			catalogSpecs = append(catalogSpecs, ContainerSpec{Name: "aurago-store-" + entry.ID, Image: entry.Image, HostBinds: resolveHostBinds(entry.HostBinds), Labels: labels(entry.ID, "")})
		}
		for _, companion := range entry.Companions {
			if len(companion.HostBinds) > 0 {
				catalogSpecs = append(catalogSpecs, ContainerSpec{Name: "aurago-store-" + entry.ID + "-" + companion.ID, Image: companion.Image, HostBinds: resolveHostBinds(companion.HostBinds), Labels: labels(entry.ID, companion.ID)})
			}
		}
	}
	sawArcaneProxy := false
	for _, spec := range catalogSpecs {
		if spec.Name == "aurago-store-arcane-socket-proxy" {
			sawArcaneProxy = true
		}
		if _, err := adapter.CreateContainer(ctx, spec); err != nil {
			t.Fatalf("%s: catalog host bind rejected: %v", spec.Name, err)
		}
	}
	if !sawArcaneProxy {
		t.Fatalf("catalog specs %v do not include the Arcane socket proxy that failed on aurago-test", catalogSpecs)
	}

	writable := socket
	writable.ReadOnly = false
	for name, spec := range map[string]ContainerSpec{
		"app without catalog bind":    {Name: "aurago-store-excalidraw", Image: "excalidraw/excalidraw:latest", HostBinds: []HostBinding{socket}, Labels: labels("excalidraw", "")},
		"writable socket":             {Name: "aurago-store-dozzle", Image: "ghcr.io/amir20/dozzle:latest", HostBinds: []HostBinding{writable}, Labels: labels("dozzle", "")},
		"extra host root":             {Name: "aurago-store-dozzle", Image: "ghcr.io/amir20/dozzle:latest", HostBinds: []HostBinding{socket, {HostPath: "/", ContainerPath: "/host"}}, Labels: labels("dozzle", "")},
		"arcane main app has none":    {Name: "aurago-store-arcane", Image: "ghcr.io/getarcaneapp/manager:latest", HostBinds: []HostBinding{socket}, Labels: labels("arcane", "")},
		"managed flag is not trusted": {Name: "aurago-store-dozzle", Image: "ghcr.io/amir20/dozzle:latest", HostBinds: []HostBinding{{HostPath: socket.HostPath, ContainerPath: socket.ContainerPath, ReadOnly: true, Managed: true}}, Labels: labels("dozzle", "")},
		"unknown companion":           {Name: "aurago-store-arcane-other", Image: "tecnativa/docker-socket-proxy:latest", HostBinds: []HostBinding{socket}, Labels: labels("arcane", "other")},
		"no app label":                {Name: "aurago-store-dozzle", Image: "ghcr.io/amir20/dozzle:latest", HostBinds: []HostBinding{socket}, Labels: map[string]string{"aurago.desktop_store": "true"}},
	} {
		if _, err := adapter.CreateContainer(ctx, spec); err == nil || !strings.Contains(err.Error(), "mounting sensitive host path") {
			t.Fatalf("%s: error = %v, want sensitive-path denial", name, err)
		}
	}
	if len(created) != len(catalogSpecs) {
		t.Fatalf("created = %v, want only the %d catalog containers", created, len(catalogSpecs))
	}
}
