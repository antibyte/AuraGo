package tools

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageQuickSnapshotSizeAndLedger(t *testing.T) {
	cfg := fixtureHomepageQuickConfig(t)
	large := filepath.Join(cfg.HomepageWorkspace, "site", "dist", "large.bin")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(65 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := newHomepageQuickOrigin(cfg, "native"); err == nil {
		t.Fatal("oversized publication accepted")
	}
	if err := os.Remove(large); err != nil {
		t.Fatal(err)
	}
	origin, err := newHomepageQuickOrigin(cfg, "native")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	if err := origin.recordPublication("https://fixture.trycloudflare.com"); err != nil {
		t.Fatal(err)
	}
	var artifact, status string
	if err := origin.db.QueryRow("SELECT artifact_hash, status FROM homepage_deployments WHERE project_id = ?", origin.projectID).Scan(&artifact, &status); err != nil {
		t.Fatal(err)
	}
	if artifact != origin.artifactHash || status != "published_unverified" {
		t.Fatalf("invalid publication record: %s %s", artifact, status)
	}
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		cfg.quickOrigin, cfg.DockerHost = origin, "tcp://remote.invalid:2375"
		if result := startDockerQuickTunnel(cfg, 0, slog.Default()); !strings.Contains(result, "local Docker socket") {
			t.Fatalf("remote daemon accepted: %s", result)
		}
	}
}

func TestCloudflareQuickUncertainDockerStartStopsOriginalDaemon(t *testing.T) {
	resetCloudflareTunnelRuntimeForTest()
	defer resetCloudflareTunnelRuntimeForTest()
	configureDockerSecurityTestPermissions(t, false)
	stops := 0
	host := fakeDockerHost(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/create"):
			w.WriteHeader(201)
		case strings.HasSuffix(r.URL.Path, "/start"):
			w.WriteHeader(504) // the daemon may already have accepted the start
		case strings.HasSuffix(r.URL.Path, "/stop"):
			stops++
			w.WriteHeader(204)
		case r.Method == http.MethodDelete:
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	result := createAndStartContainer(DockerConfig{Host: host}, cfdContainerName, map[string]interface{}{}, slog.Default(), "quick")
	if !strings.Contains(result, "unconfirmed") || tunnelMode != "docker" || tunnelDockerHost != host {
		t.Fatalf("start state lost: %s", result)
	}
	result = CloudflareTunnelStop(CloudflareTunnelConfig{DockerHost: "tcp://changed.invalid:2375", DataDir: t.TempDir()}, nil, slog.Default())
	if !cloudflareTunnelToolResultOK(result) || stops < 1 {
		t.Fatalf("wrong daemon or failed stop: %s stops=%d", result, stops)
	}
}

func fixtureHomepageQuickConfig(t *testing.T) CloudflareTunnelConfig {
	t.Helper()
	root := t.TempDir()
	cfg := CloudflareTunnelConfig{Enabled: true, HomepageEnabled: true, Mode: "native", AuthMethod: "quick", QuickProjectDir: "site", HomepageWorkspace: filepath.Join(root, "workspace"), HomepageRegistryPath: filepath.Join(root, "registry.db"), DataDir: filepath.Join(root, "data")}
	if err := os.MkdirAll(filepath.Join(cfg.HomepageWorkspace, "site", "dist"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.HomepageWorkspace, "site", "dist", "index.html"), []byte("immutable-site"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := InitHomepageRegistryDB(cfg.HomepageRegistryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, _, err := RegisterProject(db, HomepageProject{Name: "Site", ProjectDir: "site", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestHomepageQuickOriginBindsRegisteredSnapshotAndRevocation(t *testing.T) {
	cfg := fixtureHomepageQuickConfig(t)
	origin, err := newHomepageQuickOrigin(cfg, "native")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	get := func(host, path string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, origin.URL("127.0.0.1")+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	if code, _ := get("untrusted.example", "/"); code != 404 {
		t.Fatalf("untrusted origin host status=%d", code)
	}
	if err := os.WriteFile(filepath.Join(cfg.HomepageWorkspace, "site", "dist", "index.html"), []byte("changed-after-publication"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, body := get(origin.hostHeader, "/"); code != 200 || body != "immutable-site" {
		t.Fatalf("snapshot replaced: %d %q", code, body)
	}
	if code, _ := get(origin.hostHeader, "/../registry.db"); code != 404 {
		t.Fatalf("outside file status=%d", code)
	}
	db, err := InitHomepageRegistryDB(cfg.HomepageRegistryPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("UPDATE homepage_projects SET status='archived'"); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(origin.hostHeader, "/"); code != 404 {
		t.Fatalf("archived project still published: %d", code)
	}
	snapshotRoot := origin.snapshot.root
	origin.Close()
	if _, err := os.Stat(snapshotRoot); !os.IsNotExist(err) {
		t.Fatalf("snapshot cleanup: %v", err)
	}
}

func TestHomepageQuickPublicationRejectsPortsUnregisteredAndSensitiveFiles(t *testing.T) {
	cfg := fixtureHomepageQuickConfig(t)
	for _, port := range []int{22, 80, 2375, 8080, 11434, 65536, -1} {
		got := CloudflareTunnelQuickTunnel(cfg, nil, slogDiscard(), port)
		if !strings.Contains(got, "no longer accepts a port") {
			t.Fatalf("arbitrary port %d accepted: %s", port, got)
		}
	}
	for _, dir := range []string{"", ".", "../site", "unregistered"} {
		cfg.QuickProjectDir = dir
		if origin, err := newHomepageQuickOrigin(cfg, "native"); err == nil {
			origin.Close()
			t.Fatalf("accepted project %q", dir)
		}
	}
	cfg.QuickProjectDir = "site"
	if err := os.WriteFile(filepath.Join(cfg.HomepageWorkspace, "site", "dist", ".env"), []byte("fixture-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if origin, err := newHomepageQuickOrigin(cfg, "native"); err == nil {
		origin.Close()
		t.Fatal("published sensitive file")
	}
	if got := HomepageTunnel(HomepageConfig{}, 2375, slogDiscard()); !strings.Contains(got, "arbitrary ports are disabled") {
		t.Fatal("legacy entry can expose arbitrary ports")
	}
}

func TestHomepageQuickUncertainTerminationRetainsDisabledPort(t *testing.T) {
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	cfg := fixtureHomepageQuickConfig(t)
	origin, err := newHomepageQuickOrigin(cfg, "native")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	registry := NewProcessRegistry(slogDiscard())
	tunnelMu.Lock()
	tunnelQuickOrigin = origin
	tunnelMode = "native"
	tunnelPID = 987654321 // No process handle: termination must remain unconfirmed.
	tunnelMu.Unlock()
	cfg.ReadOnly = true
	got := CloudflareTunnelShutdown(cfg, registry, slogDiscard(), true)
	if !strings.Contains(got, "unconfirmed") || !origin.disabled.Load() {
		t.Fatalf("uncertain stop did not revoke serving: %s", got)
	}
	address := origin.listener.Addr().String()
	if listener, err := net.Listen("tcp4", address); err == nil {
		listener.Close()
		t.Fatal("unconfirmed termination released the origin port")
	}
	req, _ := http.NewRequest(http.MethodGet, origin.URL("127.0.0.1")+"/", nil)
	req.Host = origin.hostHeader
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("revoked listener served files: %d", resp.StatusCode)
	}
}

func TestHomepageQuickInvalidRestartPreservesExistingTunnel(t *testing.T) {
	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)
	cfg := fixtureHomepageQuickConfig(t)
	origin, err := newHomepageQuickOrigin(cfg, "native")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	tunnelMu.Lock()
	tunnelQuickOrigin = origin
	tunnelMode = "native"
	tunnelMu.Unlock()
	cfg.QuickProjectDir = "unknown"
	got := CloudflareTunnelRestart(cfg, nil, nil, slogDiscard())
	if !strings.Contains(got, `"status":"error"`) || origin.disabled.Load() {
		t.Fatalf("invalid restart disrupted existing publication: %s", got)
	}
}
