package desktopstore

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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
	ctx := context.Background()

	// Specs come from the production record builders, so the labels the trust
	// check reads are the labels install, update and rollback write.
	appSpec := func(appID, image string, binds []HostBinding) ContainerSpec {
		return containerSpecFromRecord(InstalledApp{AppID: appID, ContainerName: "aurago-store-" + appID, Image: image, HostBinds: append([]HostBinding(nil), binds...)})
	}
	companionSpec := func(appID string, companion CompanionTemplate, binds []HostBinding) ContainerSpec {
		return companionContainerSpec(InstalledApp{AppID: appID, ContainerName: "aurago-store-" + appID}, CompanionApp{
			ID:            companion.ID,
			Name:          companion.Name,
			ContainerName: "aurago-store-" + appID + "-" + companion.ID,
			Image:         companion.Image,
			NetworkMode:   companion.NetworkMode,
			HostBinds:     append([]HostBinding(nil), binds...),
		})
	}

	// Every host bind the code catalog declares, resolved the way install and
	// update resolve them, must create. The Arcane socket proxy (read-only
	// Docker socket) is the companion that failed on aurago-test and keeps its
	// catalog socket bind on main, so the negative cases below start from it.
	var catalogSpecs []ContainerSpec
	var arcaneProxy, beszelAgent CompanionTemplate
	var dozzleImage string
	for _, entry := range DefaultCatalog() {
		if entry.ID == "dozzle" {
			dozzleImage = entry.Image
		}
		if len(entry.HostBinds) > 0 {
			catalogSpecs = append(catalogSpecs, appSpec(entry.ID, entry.Image, resolveHostBinds(entry.HostBinds)))
		}
		for _, companion := range entry.Companions {
			if entry.ID == "beszel" && companion.ID == "agent" {
				beszelAgent = companion
			}
			if len(companion.HostBinds) == 0 {
				continue
			}
			catalogSpecs = append(catalogSpecs, companionSpec(entry.ID, companion, resolveHostBinds(companion.HostBinds)))
			if entry.ID == "arcane" && companion.ID == "socket-proxy" {
				arcaneProxy = companion
			}
		}
	}
	if len(arcaneProxy.HostBinds) == 0 {
		t.Fatal("catalog has no Arcane socket-proxy host bind")
	}
	if dozzleImage == "" || beszelAgent.ID == "" {
		t.Fatal("catalog has no Dozzle app or Beszel agent companion")
	}
	// The read-only monitoring proxies (main, baa3b047e) moved the Docker socket
	// of Dozzle and the Beszel agent into socket-proxy companions, so the only
	// trusted catalog binds are the sockets of the three socket proxies. A new
	// catalog host bind is a policy exemption: add it here only after review.
	var trustedNames []string
	for _, spec := range catalogSpecs {
		trustedNames = append(trustedNames, spec.Name)
	}
	sort.Strings(trustedNames)
	if want := []string{"aurago-store-arcane-socket-proxy", "aurago-store-beszel-socket-proxy", "aurago-store-dozzle-socket-proxy"}; !reflect.DeepEqual(trustedNames, want) {
		t.Fatalf("catalog containers with trusted host binds = %v, want %v", trustedNames, want)
	}
	for _, spec := range catalogSpecs {
		if _, err := adapter.CreateContainer(ctx, spec); err != nil {
			t.Fatalf("%s: catalog host bind rejected: %v", spec.Name, err)
		}
	}

	proxyBinds := resolveHostBinds(arcaneProxy.HostBinds)
	socketDenial := fmt.Sprintf("mounting sensitive host path %q", proxyBinds[0].HostPath)
	variant := func(change func([]HostBinding) []HostBinding) []HostBinding {
		return change(append([]HostBinding(nil), proxyBinds...))
	}
	for name, tc := range map[string]struct {
		spec ContainerSpec
		want string
	}{
		"writable socket": {companionSpec("arcane", arcaneProxy, variant(func(b []HostBinding) []HostBinding {
			b[0].ReadOnly = false
			return b
		})), socketDenial},
		"other container path": {companionSpec("arcane", arcaneProxy, variant(func(b []HostBinding) []HostBinding {
			b[0].ContainerPath = "/sock"
			return b
		})), socketDenial},
		"managed flag is not trusted": {companionSpec("arcane", arcaneProxy, variant(func(b []HostBinding) []HostBinding {
			b[0].Managed = true
			return b
		})), socketDenial},
		"extra host root": {companionSpec("arcane", arcaneProxy, variant(func(b []HostBinding) []HostBinding {
			return append(b, HostBinding{HostPath: "/", ContainerPath: "/host"})
		})), `mounting sensitive host path "/"`},
		"arcane main app has none":    {appSpec("arcane", "ghcr.io/getarcaneapp/manager:latest", proxyBinds), socketDenial},
		"app without catalog bind":    {appSpec("excalidraw", "excalidraw/excalidraw:latest", proxyBinds), socketDenial},
		"unknown companion":           {companionSpec("arcane", CompanionTemplate{ID: "other", Image: arcaneProxy.Image}, proxyBinds), socketDenial},
		"companion of another app":    {companionSpec("excalidraw", arcaneProxy, proxyBinds), socketDenial},
		"record without an app id":    {appSpec("", arcaneProxy.Image, proxyBinds), socketDenial},
		"companion without an app id": {companionSpec("", arcaneProxy, proxyBinds), socketDenial},
		// Records installed before the monitoring proxies keep a direct socket
		// bind the catalog no longer declares; only the Store Update migrates them.
		"legacy dozzle direct socket":       {appSpec("dozzle", dozzleImage, proxyBinds), socketDenial},
		"legacy beszel agent direct socket": {companionSpec("beszel", beszelAgent, proxyBinds), socketDenial},
	} {
		if _, err := adapter.CreateContainer(ctx, tc.spec); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error = %v, want %s", name, err, tc.want)
		}
	}
	if len(created) != len(catalogSpecs) {
		t.Fatalf("created = %v, want only the %d catalog containers", created, len(catalogSpecs))
	}
}

func TestToolsDockerAdapterRenameContainerMapsEngineAnswers(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	status := map[string]int{
		"aurago-store-ok":       http.StatusNoContent,
		"aurago-store-missing":  http.StatusNotFound,
		"aurago-store-conflict": http.StatusConflict,
		"aurago-store-broken":   http.StatusInternalServerError,
	}
	var renames []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1.45/containers/"), "/rename")
		code, ok := status[name]
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/rename") || !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		renames = append(renames, name+"->"+r.URL.Query().Get("name"))
		w.WriteHeader(code)
		if code >= 400 {
			_, _ = io.WriteString(w, `{"message":"engine says no"}`)
		}
	}))
	defer server.Close()
	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), "", nil)
	ctx := context.Background()

	if err := adapter.RenameContainer(ctx, "aurago-store-ok", parkedContainerName("aurago-store-ok")); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := adapter.RenameContainer(ctx, "aurago-store-missing", "x"); !errors.Is(err, errContainerNotFound) {
		t.Fatalf("404 rename error = %v, want errContainerNotFound", err)
	}
	if err := adapter.RenameContainer(ctx, "aurago-store-conflict", "x"); !errors.Is(err, errContainerNameConflict) {
		t.Fatalf("409 rename error = %v, want errContainerNameConflict", err)
	}
	err := adapter.RenameContainer(ctx, "aurago-store-broken", "x")
	if err == nil || errors.Is(err, errContainerNotFound) || errors.Is(err, errContainerNameConflict) || !strings.Contains(err.Error(), "engine says no") {
		t.Fatalf("500 rename error = %v, want a plain engine error", err)
	}
	if len(renames) == 0 || renames[0] != "aurago-store-ok->aurago-store-ok.prev" {
		t.Fatalf("renames = %#v", renames)
	}
}

func TestToolsDockerAdapterMarksMissingContainers(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"No such container: aurago-store-gone"}`)
	}))
	defer server.Close()
	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), "", nil)
	ctx := context.Background()

	if _, err := adapter.InspectContainer(ctx, "aurago-store-gone"); !errors.Is(err, errContainerNotFound) || err.Error() != "container aurago-store-gone not found" {
		t.Fatalf("inspect error = %v, want errContainerNotFound with the historical message", err)
	}
	if err := adapter.StartContainer(ctx, "aurago-store-gone"); !errors.Is(err, errContainerNotFound) || err.Error() != "container aurago-store-gone not found" {
		t.Fatalf("start error = %v, want errContainerNotFound with the historical message", err)
	}
}

func TestToolsDockerAdapterInspectReportsExitAndRestartState(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/version" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1.45/containers/aurago-store-demo-db/json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"Name":"/aurago-store-demo-db","RestartCount":4,"State":{"Running":true,"Restarting":true,"Status":"restarting","ExitCode":1}}`)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	adapter := NewToolsDockerAdapter("tcp://"+strings.TrimPrefix(server.URL, "http://"), "", nil)
	state, err := adapter.InspectContainer(context.Background(), "aurago-store-demo-db")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	want := ContainerState{Name: "aurago-store-demo-db", Running: true, Restarting: true, Status: "restarting", ExitCode: 1, RestartCount: 4}
	if !reflect.DeepEqual(state, want) {
		t.Fatalf("state = %#v, want %#v", state, want)
	}
}
