package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

func TestOwnOverlayUpperDirReadsTheRootOverlayMount(t *testing.T) {
	for name, tc := range map[string]struct{ mountinfo, want string }{
		"overlay2":               {selfMountinfoFixture, "/var/lib/docker/overlay2/9f1c2e/diff"},
		"containerd snapshotter": {"2517 2097 0:69 / / ro,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16354/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/work,nouserxattr\n", "/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/fs"},
		"escaped data root":      {"1520 1351 0:132 / / rw,relatime - overlay overlay rw,lowerdir=/srv/docker\\040data/overlay2/l/A,upperdir=/srv/docker\\040data/overlay2/ab/diff,workdir=/srv/docker\\040data/overlay2/ab/work\n", "/srv/docker data/overlay2/ab/diff"},
		"optional fields":        {"1520 1351 0:132 / / rw,relatime master:612 shared:7 - overlay overlay rw,upperdir=/var/lib/docker/overlay2/cc/diff\n", "/var/lib/docker/overlay2/cc/diff"},
		"fuse-overlayfs":         {"1520 1351 0:132 / / rw,relatime - fuse.fuse-overlayfs fuse-overlayfs rw,user_id=0,group_id=0\n", ""},
		"btrfs root":             {"1520 1351 0:44 /@/var/lib/docker/btrfs/subvolumes/ab / rw,relatime - btrfs /dev/sda2 rw\n", ""},
		"overlay elsewhere":      {"1600 1520 0:140 / /mnt/data rw - overlay overlay rw,upperdir=/x/diff\n", ""},
		"conflicting roots":      {"1 0 0:1 / / rw - overlay overlay rw,upperdir=/a/diff\n2 1 0:2 / / rw - overlay overlay rw,upperdir=/b/diff\n", ""},
		"no upperdir":            {"1520 1351 0:132 / / rw - overlay overlay rw,lowerdir=/a:/b\n", ""},
		"empty":                  {"", ""},
	} {
		if got := ownOverlayUpperDir(tc.mountinfo); got != tc.want {
			t.Fatalf("%s: ownOverlayUpperDir = %q, want %q", name, got, tc.want)
		}
	}
}

func upperDirSidecarAPI(t *testing.T, providerID, appID, providerDriver, appDriver string) string {
	t.Helper()
	resetContainerSelfProofCache()
	t.Cleanup(resetContainerSelfProofCache)
	provider := `{"Id":"` + providerID + `","Name":"/tailscale","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"bridge"},` + providerDriver + `}`
	app := `{"Id":"` + appID + `","Name":"/aurago","Config":{"Labels":{}},"HostConfig":{"NetworkMode":"container:` + providerID + `"},` + appDriver + `}`
	return newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
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

func listFlags(t *testing.T, s *Server) map[string]map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	var list struct {
		Containers []map[string]interface{} `json:"containers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list %s: %v", rec.Body.String(), err)
	}
	out := map[string]map[string]interface{}{}
	for _, c := range list.Containers {
		out[c["id"].(string)] = c
	}
	return out
}

// TestContainerUpperDirProvesSelfBehindANetworkSidecar: with the overlay2
// driver, AuraGo's root upper directory names its own container even when it
// joins a sidecar's network namespace, so its update is refused again.
func TestContainerUpperDirProvesSelfBehindANetworkSidecar(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	host := upperDirSidecarAPI(t, providerID, appID,
		`"GraphDriver":{"Name":"overlay2","Data":{"UpperDir":"/var/lib/docker/overlay2/77aa11/diff"}}`,
		`"GraphDriver":{"Name":"overlay2","Data":{"UpperDir":"/var/lib/docker/overlay2/9f1c2e/diff"}}`)
	t.Cleanup(replaceContainerSelfHostname("aurago-host"))
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}))
	t.Cleanup(replaceContainerEndpointAddresses())
	s := testContainerServer(true, false)
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Docker.Host = host
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
	rec = httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/tailscale/update", nil))
	if body := decodeContainerResponse(t, rec); rec.Code != http.StatusConflict || body["code"] != containerCodeConfirmationRequired || body["owner"] != "shared-network" {
		t.Fatalf("provider update = %d %v, want 409 confirmation", rec.Code, body)
	}
	flags := listFlags(t, s)
	if c := flags[appID[:12]]; c["self"] != true || c["shared_network"] != nil {
		t.Fatalf("list app entry = %v, want self only", c)
	}
	if c := flags[providerID[:12]]; c["self"] != nil || c["shared_network"] != true {
		t.Fatalf("list provider entry = %v, want shared_network", c)
	}
}

// TestContainerUpperDirUnavailableKeepsTheSharedNetworkConfirmation: the
// containerd image store reports no UpperDir; today's confirmation stays.
func TestContainerUpperDirUnavailableKeepsTheSharedNetworkConfirmation(t *testing.T) {
	const providerID, appID = selfContainerID, otherContainerID
	containerdMountinfo := strings.Replace(selfMountinfoFixture,
		"upperdir=/var/lib/docker/overlay2/9f1c2e/diff,workdir=/var/lib/docker/overlay2/9f1c2e/work",
		"upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16355/work", 1)
	for name, driver := range map[string]string{
		"API 1.45": `"GraphDriver":{"Name":"overlayfs","Data":null}`,
		"API 1.56": `"GraphDriver":null,"Storage":{"RootFS":{"Snapshot":{"Name":"overlayfs"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			host := upperDirSidecarAPI(t, providerID, appID, driver, driver)
			t.Cleanup(replaceContainerSelfHostname("aurago-host"))
			t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": containerdMountinfo}))
			t.Cleanup(replaceContainerEndpointAddresses())
			s := testContainerServer(true, false)
			s.Cfg.Runtime.IsDocker = true
			s.Cfg.Docker.Host = host
			if got, want := classifyContainerForAction(context.Background(), s, tools.DockerConfig{Host: host}, "aurago"), (containerProtection{Owner: dockerutil.AppOwner, SharedNetwork: true}); got != want {
				t.Fatalf("app without an UpperDir = %+v, want %+v", got, want)
			}
			for id, c := range listFlags(t, s) {
				if c["self"] != nil || c["shared_network"] != true {
					t.Fatalf("list entry %s = %v, want shared_network and no self", id, c)
				}
			}
		})
	}
}

// TestContainerUpperDirNeedsTheDockerRuntime: a native AuraGo whose host root
// happens to be an overlay mount never proves self.
func TestContainerUpperDirNeedsTheDockerRuntime(t *testing.T) {
	t.Cleanup(replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture}))
	if got := containerOwnUpperDir(false); got != "" {
		t.Fatalf("native runtime upper dir = %q, want none", got)
	}
	if got := containerOwnUpperDir(true); got != "/var/lib/docker/overlay2/9f1c2e/diff" {
		t.Fatalf("Docker runtime upper dir = %q", got)
	}
}
