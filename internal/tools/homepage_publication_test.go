package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

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
	rel, err := filepath.Rel(workspace, snapshot.root)
	if err != nil {
		t.Fatal(err)
	}
	state := webserverState{ProjectDir: "site", BuildDir: ".", SnapshotDir: filepath.ToSlash(rel), IndexSHA256: hash}
	if _, err := homepageSavedSnapshot(workspace, state); err != nil {
		t.Fatal(err)
	}
	if err := validateHomepageSourceEditPath(filepath.ToSlash(filepath.Join(rel, "index.html"))); err == nil {
		t.Fatal("native editor must refuse publication snapshots")
	}
	if err := os.WriteFile(filepath.Join(snapshot.root, "index.html"), []byte("<html>tampered</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := homepageSavedSnapshot(workspace, state); err == nil {
		t.Fatal("changed saved snapshot must not restart")
	}
}

func TestHomepageWebServerFailedStartRestoresPreviousPublication(t *testing.T) {
	workspace := t.TempDir()
	writeNetlifyDeployFiles(t, filepath.Join(workspace, "site"), map[string]string{"index.html": "<html>new publication</html>"})
	const previousConfig = ":8080 { root * /srv; file_server }\n"
	if err := os.WriteFile(filepath.Join(workspace, ".aurago-Caddyfile"), []byte(previousConfig), 0600); err != nil {
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
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"Id": "previous-id", "Config": map[string]interface{}{"Labels": dockerutil.ManagedLabels(dockerutil.HomepageOwner, "homepage", "web", "")}})
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
				if strings.HasSuffix(bind, ":/srv:ro") && strings.Contains(filepath.ToSlash(bind), "/.aurago/published/") {
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
	config, err := os.ReadFile(filepath.Join(workspace, ".aurago-Caddyfile"))
	if err != nil || string(config) != previousConfig {
		t.Fatalf("previous config lost: %s %v", config, err)
	}
	if project, build := loadWebserverState(workspace); project != "previous-site" || build != "out" {
		t.Fatalf("failed deployment replaced state: %s/%s", project, build)
	}
	entries, err := os.ReadDir(filepath.Join(workspace, ".aurago", "published"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed candidate retained: %v %v", entries, err)
	}
}
