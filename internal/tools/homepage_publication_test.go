package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerutil"
)

func testHomepagePublicationDir(t *testing.T, workspace string) string {
	t.Helper()
	dir, err := ensureHomepagePublicationDir(workspace)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHomepageLocalSnapshotKeepsPublishedBytesPrivate(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "site")
	writeNetlifyDeployFiles(t, project, map[string]string{
		"index.html":            "<html><article>original</article></html>",
		"index.html.bak-pre325": "private backup", ".git/HEAD": "private ref",
		".scripts/repair.py": "private script", ".aurago-site-manifest.json": "private manifest",
		"assets/app.js": "console.log('ok')", ".well-known/security.txt": "public contact",
	})
	cfg := HomepageConfig{WorkspacePath: workspace}
	snapshot, hash, err := homepageLocalSnapshot(cfg, "site", ".")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	for _, name := range []string{"index.html.bak-pre325", ".git/HEAD", ".scripts/repair.py", ".aurago-site-manifest.json"} {
		if _, err := os.Stat(filepath.Join(snapshot.root, name)); !os.IsNotExist(err) {
			t.Fatalf("published private path %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("<html>edited</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := homepageStaticIndexHash(snapshot.root); err != nil || got != hash {
		t.Fatalf("source edit changed snapshot: %s %v", got, err)
	}
	workspaceRel, err := filepath.Rel(workspace, snapshot.root)
	if err != nil || filepath.IsLocal(workspaceRel) {
		t.Fatalf("publication must be outside the dev workspace: %s, %v", workspaceRel, err)
	}
	dir := testHomepagePublicationDir(t, workspace)
	rel, err := filepath.Rel(dir, snapshot.root)
	if err != nil {
		t.Fatal(err)
	}
	state := webserverState{ProjectDir: "site", BuildDir: ".", SnapshotDir: filepath.ToSlash(rel), IndexSHA256: hash}
	if _, err := homepageSavedSnapshot(dir, state); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshot.root, "index.html"), []byte("<html>tampered</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := homepageSavedSnapshot(dir, state); err == nil {
		t.Fatal("changed saved snapshot must not restart")
	}
}

func TestHomepageLegacyStateRequiresExplicitRepublish(t *testing.T) {
	workspace := t.TempDir()
	writeNetlifyDeployFiles(t, workspace, map[string]string{
		webserverStateFile:          `{"project_dir":"legacy","build_dir":"out"}`,
		".aurago-proxy-routes.json": `[{"path":"/legacy","port":3000}]`,
		"legacy/out/index.html":     "<html>Legacy</html>",
	})
	if routes, err := loadProxyRoutes(workspace); err != nil || len(routes) != 0 {
		t.Fatalf("routes were loaded from the dev workspace: %v %v", routes, err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: unavailableHomepageDockerHost(t)}
	if got := HomepageWebServerStart(cfg, "", "", slogDiscard()); !strings.Contains(got, "explicit publish") {
		t.Fatalf("legacy state was eligible for automatic restore: %s", got)
	}
}

func TestHomepageRetireLegacyWebServerStopsOnlyInspectedLegacyContainer(t *testing.T) {
	for _, protected := range []bool{false, true} {
		var stops atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/version":
				_, _ = w.Write([]byte(`{"ApiVersion":"1.44","Version":"fixture"}`))
			case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/"+homepageWebContainer+"/json"):
				var info homepageWebContainerInfo
				info.ID = "fixture-id"
				if protected {
					info.Config.Labels = map[string]string{homepagePublicationLabel: "protected"}
				}
				info.Mounts = append(info.Mounts, struct{ Source, Destination string }{
					Source: "/fixture/" + homepageCaddyfileName, Destination: "/etc/caddy/Caddyfile",
				})
				_ = json.NewEncoder(w).Encode(info)
			case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/fixture-id/stop"):
				stops.Add(1)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
			}
		}))
		host := strings.Replace(server.URL, "http://", "tcp://", 1)
		err := HomepageRetireLegacyWebServer(context.Background(), host)
		server.Close()
		if got := stops.Load(); err != nil || (protected && got != 0) || (!protected && got != 1) {
			t.Fatalf("protected=%v, stop requests=%d, error=%v", protected, got, err)
		}
	}
}

func TestHomepageWebServerLocalPublishBundlesAssetsOnlyInSnapshot(t *testing.T) {
	workspace, data := t.TempDir(), t.TempDir()
	writeNetlifyDeployFiles(t, filepath.Join(workspace, "site"), map[string]string{
		"index.html": `<html><img src="/files/generated_images/sample.png"></html>`,
	})
	writeNetlifyDeployFiles(t, data, map[string]string{"generated_images/sample.png": "asset"})
	cfg := HomepageConfig{WorkspacePath: workspace, DataDir: data, DockerHost: unavailableHomepageDockerHost(t)}
	if got := HomepageWebServerStart(cfg, "site", ".", slogDiscard()); !strings.Contains(got, "Docker not available") {
		t.Fatalf("expected private snapshot to reach runtime selection: %s", got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "site", "files")); !os.IsNotExist(err) {
		t.Fatalf("source was mutated by asset bundling: %v", err)
	}
}

func TestHomepageWebServerFailedStartRestoresPreviousPublication(t *testing.T) {
	workspace := t.TempDir()
	dir := testHomepagePublicationDir(t, workspace)
	writeNetlifyDeployFiles(t, filepath.Join(workspace, "site"), map[string]string{"index.html": "<html>new publication</html>"})
	const previousConfig = ":8080 { root * /srv; file_server }\n"
	if err := os.WriteFile(filepath.Join(dir, homepageCaddyfileName), []byte(previousConfig), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveWebserverState(workspace, "previous-site", "out"); err != nil {
		t.Fatal(err)
	}
	var renamed, restored, startedPrevious, mountedReadOnly bool
	starts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		if parts := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2); len(parts) == 2 && strings.HasPrefix(parts[0], "v1.") {
			p = "/" + parts[1]
		}
		switch {
		case r.Method == http.MethodGet && p == "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.44","Version":"fixture"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/_ping"):
			w.WriteHeader(200)
		case strings.HasPrefix(p, "/images/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && strings.HasSuffix(p, "/json"):
			labels := dockerutil.ManagedLabels(dockerutil.HomepageOwner, "homepage", "web", "")
			labels[homepagePublicationLabel] = homepageDocumentHash([]byte(dir))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"Id": "previous-id", "Config": map[string]interface{}{"Labels": labels}})
		case strings.HasSuffix(p, "/rename"):
			if r.URL.Query().Get("name") == homepageWebContainer {
				restored = true
			} else {
				renamed = true
			}
			w.WriteHeader(204)
		case strings.HasSuffix(p, "/stop") || r.Method == http.MethodDelete:
			w.WriteHeader(204)
		case p == "/containers/create":
			var payload struct{ HostConfig struct{ Binds []string } }
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			for _, bind := range payload.HostConfig.Binds {
				if strings.HasSuffix(bind, ":/srv:ro") && strings.HasPrefix(filepath.ToSlash(bind), filepath.ToSlash(dir)+"/published/") {
					mountedReadOnly = true
				}
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"Id":"new-id"}`))
		case strings.HasSuffix(p, "/start"):
			starts++
			if starts == 1 {
				http.Error(w, "fixture start failure", 500)
			} else {
				startedPrevious = true
				w.WriteHeader(204)
			}
		default:
			t.Errorf("unexpected Docker request %s %s", r.Method, p)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	result := HomepageWebServerStart(HomepageConfig{WorkspacePath: workspace, DockerHost: strings.Replace(server.URL, "http://", "tcp://", 1)}, "site", ".", slogDiscard())
	if !strings.Contains(result, "Failed to start") || !renamed || !restored || !startedPrevious || !mountedReadOnly {
		t.Fatalf("rollback incomplete: result=%s rename=%v restore=%v start=%v readOnly=%v", result, renamed, restored, startedPrevious, mountedReadOnly)
	}
	config, err := os.ReadFile(filepath.Join(dir, homepageCaddyfileName))
	if err != nil || string(config) != previousConfig {
		t.Fatalf("previous config lost: %s %v", config, err)
	}
	if project, build := loadWebserverState(workspace); project != "previous-site" || build != "out" {
		t.Fatalf("failed deployment replaced state: %s/%s", project, build)
	}
	entries, err := os.ReadDir(filepath.Join(dir, homepagePublicationsFolder))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed candidate retained: %v %v", entries, err)
	}
}
