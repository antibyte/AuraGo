package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// replaceContainerSelfMarker installs marker as this process's marker path and
// forgets cached proofs; the restore does both again.
func replaceContainerSelfMarker(marker string) func() {
	old := currentContainerSelfMarker()
	setContainerSelfMarker(marker)
	resetContainerSelfProofCache()
	return func() {
		setContainerSelfMarker(old)
		resetContainerSelfProofCache()
	}
}

// writtenSelfMarker creates a marker file the way initContainerSelfMarker
// does and returns its slash path: a marker proves something only while it
// exists in AuraGo's own layer.
func writtenSelfMarker(t *testing.T) string {
	t.Helper()
	marker, err := writeContainerSelfMarker(t.TempDir())
	if err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return filepath.ToSlash(marker)
}

// containerdSidecarMountinfo is selfMountinfoFixture on the containerd image
// store: the root overlay's upper directory is a snapshot, which inspect does
// not report, and the /etc files still name the network provider.
var containerdSidecarMountinfo = strings.Replace(selfMountinfoFixture,
	"upperdir=/var/lib/docker/overlay2/9f1c2e/diff,workdir=/var/lib/docker/overlay2/9f1c2e/work",
	"upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/work", 1)

// markerSidecarAPI is a containerd engine (no GraphDriver data) where the app
// container joins the provider's network namespace. archive answers HEAD
// /containers/{id}/archive for a full ID with a status code.
func markerSidecarAPI(t *testing.T, providerID, appID, marker string, archive map[string]int, heads *[]string) string {
	t.Helper()
	const driver = `"GraphDriver":{"Name":"overlayfs","Data":null}`
	provider := `{"Id":"` + providerID + `","Name":"/tailscale","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"bridge"},` + driver + `}`
	app := `{"Id":"` + appID + `","Name":"/aurago","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"container:` + providerID + `"},` + driver + `}`
	var mu sync.Mutex
	return newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && strings.HasSuffix(r.URL.Path, "/archive") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path[strings.Index(r.URL.Path, "/containers/"):], "/containers/"), "/archive")
			mu.Lock()
			*heads = append(*heads, id)
			mu.Unlock()
			if r.URL.Query().Get("path") != marker {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if code, ok := archive[id]; ok {
				if code == http.StatusOK {
					w.Header().Set("X-Docker-Container-Path-Stat", "eyJuYW1lIjoibWFya2VyIn0=")
				}
				w.WriteHeader(code)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/tailscale/json"), strings.HasSuffix(r.URL.Path, "/containers/"+providerID+"/json"):
			_, _ = w.Write([]byte(provider))
		case strings.HasSuffix(r.URL.Path, "/containers/aurago/json"), strings.HasSuffix(r.URL.Path, "/containers/"+appID+"/json"):
			_, _ = w.Write([]byte(app))
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[
				{"Id":"` + providerID + `","Names":["/tailscale"],"Image":"tailscale/tailscale","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"bridge"}},
				{"Id":"` + appID + `","Names":["/aurago"],"Image":"aurago","State":"running","Status":"Up","Labels":{},"HostConfig":{"NetworkMode":"container:` + providerID + `"}}
			]`))
		default:
			http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
		}
	})
}

func markerSidecarServer(t *testing.T, host string) *Server {
	t.Helper()
	t.Cleanup(replaceContainerSelfHostname("aurago-host"))
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": containerdSidecarMountinfo}))
	t.Cleanup(replaceContainerEndpointAddresses())
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
	return s
}

// TestContainerSelfMarkerProvesSelfBehindASidecar: on the containerd image
// store the upper directory is not reported, but the marker file AuraGo wrote
// into its own writable layer is found in exactly one container of the
// shared-network group, which is then self: its update is refused with the
// compose hint, and the provider keeps the confirmation (F-A15).
func TestContainerSelfMarkerProvesSelfBehindASidecar(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	marker := writtenSelfMarker(t)
	var heads []string
	host := markerSidecarAPI(t, providerID, appID, marker, map[string]int{appID: http.StatusOK, providerID: http.StatusNotFound}, &heads)
	s := markerSidecarServer(t, host)
	t.Cleanup(replaceContainerSelfMarker(marker))
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	if got, want := classifyContainerForAction(ctx, s, cfg, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, Self: true}); got != want {
		t.Fatalf("app behind the sidecar = %+v, want %+v", got, want)
	}
	if got, want := classifyContainerForAction(ctx, s, cfg, "tailscale"), (containerProtection{SharedNetwork: true}); got != want {
		t.Fatalf("network provider = %+v, want %+v", got, want)
	}
	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/aurago/update?confirm=protected", nil))
	if body := decodeContainerResponse(t, rec); rec.Code != http.StatusConflict || body["code"] != containerCodeSelfUpdateUnsupported || body["owner"] != "self" {
		t.Fatalf("self update = %d %v, want 409 %s", rec.Code, body, containerCodeSelfUpdateUnsupported)
	}
	flags := listFlags(t, s)
	if c := flags[appID[:12]]; c["self"] != true || c["shared_network"] != nil {
		t.Fatalf("list app entry = %v, want self only", c)
	}
	if c := flags[providerID[:12]]; c["self"] != nil || c["shared_network"] != true {
		t.Fatalf("list provider entry = %v, want shared_network", c)
	}
	// Each member was asked once; later classifications use the cached proof.
	if len(heads) != 2 {
		t.Fatalf("archive HEAD requests = %v, want one per group member", heads)
	}
}

// TestContainerSelfMarkerWithoutProofKeepsTheConfirmation: no marker, a Docker
// answer that is neither 200 nor 404, or the marker in more than one member
// (for example a committed copy of AuraGo's container) proves nothing.
func TestContainerSelfMarkerWithoutProofKeepsTheConfirmation(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	marker := writtenSelfMarker(t)
	for name, tc := range map[string]struct {
		marker  string
		archive map[string]int
	}{
		"no marker":          {"", map[string]int{appID: http.StatusOK}},
		"refused by a proxy": {marker, map[string]int{appID: http.StatusForbidden, providerID: http.StatusNotFound}},
		"engine error":       {marker, map[string]int{appID: http.StatusOK, providerID: http.StatusInternalServerError}},
		"marker in two":      {marker, map[string]int{appID: http.StatusOK, providerID: http.StatusOK}},
		"marker in none":     {marker, map[string]int{appID: http.StatusNotFound, providerID: http.StatusNotFound}},
	} {
		t.Run(name, func(t *testing.T) {
			var heads []string
			host := markerSidecarAPI(t, providerID, appID, marker, tc.archive, &heads)
			s := markerSidecarServer(t, host)
			t.Cleanup(replaceContainerSelfMarker(tc.marker))
			if got, want := classifyContainerForAction(context.Background(), s, tools.DockerConfig{Host: host}, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, SharedNetwork: true}); got != want {
				t.Fatalf("app = %+v, want %+v (today's confirmation)", got, want)
			}
			for id, c := range listFlags(t, s) {
				if c["self"] != nil || c["shared_network"] != true {
					t.Fatalf("list entry %s = %v, want shared_network and no self", id, c)
				}
			}
		})
	}
}

// TestContainerSelfMarkerDirMustBeOnTheRootLayer: the marker goes only where
// the covering mount is "/", the container's own writable layer. A tmpfs, a
// volume or a bind could be invisible to the archive API or shared with
// another container.
func TestContainerSelfMarkerDirMustBeOnTheRootLayer(t *testing.T) {
	const mountinfo = `1520 1351 0:132 / / rw,relatime - overlay overlay rw,upperdir=/var/lib/docker/overlay2/9f1c2e/diff
1521 1520 0:135 / /proc rw,nosuid - proc proc rw
1530 1520 0:150 / /tmp rw,nosuid,nodev - tmpfs tmpfs rw
1534 1520 8:1 /var/lib/docker/volumes/aurago_data/_data /app/data rw,relatime - ext4 /dev/sda1 rw
1540 1520 8:1 /srv/share /srv/my\040dir rw,relatime - ext4 /dev/sda1 rw
`
	for dir, want := range map[string]bool{
		"/var/tmp":           true,
		"/home/aurago":       true,
		"/app":               true,
		"/tmpx":              true,
		"/tmp":               false,
		"/tmp/sub":           false,
		"/app/data":          false,
		"/app/data/x":        false,
		"/srv/my dir/x":      false,
		"/proc":              false,
		"relative/dir":       false,
		"":                   false,
		"/var/tmp/../../tmp": false,
	} {
		if got := containerSelfMarkerOnRootLayer(mountinfo, dir); got != want {
			t.Fatalf("containerSelfMarkerOnRootLayer(%q) = %v, want %v", dir, got, want)
		}
	}
	if containerSelfMarkerOnRootLayer("1531 1520 8:1 /x /etc/hostname rw - ext4 /dev/sda1 rw\n", "/var/tmp") {
		t.Fatal("without a \"/\" mount nothing is known about the writable layer")
	}
}

func TestWriteContainerSelfMarkerCreatesAFreshFile(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, containerSelfMarkerPrefix+"00000000000000000000000000000000")
	if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "unrelated")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := writeContainerSelfMarker(dir)
	if err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if filepath.Dir(a) != dir || !regexp.MustCompile(`^\.aurago-self-[0-9a-f]{32}$`).MatchString(filepath.Base(a)) {
		t.Fatalf("marker path %q", a)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale marker of an earlier run still exists: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("an unrelated file was removed: %v", err)
	}
	b, err := writeContainerSelfMarker(dir)
	if err != nil || b == a {
		t.Fatalf("second marker = %q, %v; want a new random name", b, err)
	}
}

func TestInitContainerSelfMarkerOnlyInTheDockerRuntime(t *testing.T) {
	dir := t.TempDir()
	oldDirs, oldCheck := containerSelfMarkerDirs, containerSelfMarkerDirCheck
	containerSelfMarkerDirs = func() []string { return []string{dir} }
	onRoot := true
	containerSelfMarkerDirCheck = func(string, string) bool { return onRoot }
	t.Cleanup(func() { containerSelfMarkerDirs, containerSelfMarkerDirCheck = oldDirs, oldCheck })
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}))
	t.Cleanup(replaceContainerSelfMarker(""))
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))

	initContainerSelfMarker(false, logger)
	if got := currentContainerSelfMarker(); got != "" {
		t.Fatalf("native runtime wrote marker %q", got)
	}
	onRoot = false
	initContainerSelfMarker(true, logger)
	if got := currentContainerSelfMarker(); got != "" {
		t.Fatalf("a directory off the writable layer got marker %q", got)
	}
	onRoot = true
	initContainerSelfMarker(true, logger)
	marker := currentContainerSelfMarker()
	if marker == "" {
		t.Fatal("no marker in the Docker runtime")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker file: %v", err)
	}
}

// TestContainerSelfMarkerProvesNothingOnceItIsGone: a marker that vanished
// from AuraGo's own layer (removed by hand, or a container whose layer was
// replaced) proves nothing, even when an earlier proof is cached; the
// confirmation comes back.
func TestContainerSelfMarkerProvesNothingOnceItIsGone(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	marker := writtenSelfMarker(t)
	var heads []string
	host := markerSidecarAPI(t, providerID, appID, marker, map[string]int{appID: http.StatusOK, providerID: http.StatusNotFound}, &heads)
	s := markerSidecarServer(t, host)
	t.Cleanup(replaceContainerSelfMarker(marker))
	cfg := tools.DockerConfig{Host: host}
	ctx := context.Background()

	if got := classifyContainerForAction(ctx, s, cfg, "aurago"); !got.Self {
		t.Fatalf("app with its marker = %+v, want self", got)
	}
	if err := os.Remove(filepath.FromSlash(marker)); err != nil {
		t.Fatal(err)
	}
	if got, want := classifyContainerForAction(ctx, s, cfg, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, SharedNetwork: true}); got != want {
		t.Fatalf("app after its marker vanished = %+v, want %+v", got, want)
	}
	if c := listFlags(t, s)[appID[:12]]; c["self"] != nil || c["shared_network"] != true {
		t.Fatalf("list app entry after the marker vanished = %v, want shared_network", c)
	}
}

// TestInitContainerSelfMarkerLogsOnlyTheDirectory: the random name is the
// secret part of the proof, so the debug log names only its directory.
func TestInitContainerSelfMarkerLogsOnlyTheDirectory(t *testing.T) {
	dir := t.TempDir()
	oldDirs, oldCheck := containerSelfMarkerDirs, containerSelfMarkerDirCheck
	containerSelfMarkerDirs = func() []string { return []string{dir} }
	containerSelfMarkerDirCheck = func(string, string) bool { return true }
	t.Cleanup(func() { containerSelfMarkerDirs, containerSelfMarkerDirCheck = oldDirs, oldCheck })
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}))
	t.Cleanup(replaceContainerSelfMarker(""))
	var logs strings.Builder
	initContainerSelfMarker(true, slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	marker := currentContainerSelfMarker()
	if marker == "" {
		t.Fatal("no marker written")
	}
	out := logs.String()
	if !strings.Contains(out, "Self marker written") || !strings.Contains(out, filepath.ToSlash(dir)) {
		t.Fatalf("log = %q, want the marker's directory", out)
	}
	if strings.Contains(out, path.Base(marker)) {
		t.Fatalf("log = %q names the marker file", out)
	}
}
