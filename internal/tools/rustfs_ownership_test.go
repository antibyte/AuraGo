package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerutil"
)

func TestRustFSResourcesRemainPrivateThroughDockerAliases(t *testing.T) {
	for _, bind := range []string{"aurago_rustfs_data:/data", "project_aurago_rustfs_credentials:/keys:ro", "/var/lib/docker/volumes/aurago_rustfs_credentials/_data:/keys:ro"} {
		if !DockerBindUsesRustFSVolume(bind) || validateDockerBindMount(DockerConfig{}, bind) == nil {
			t.Fatalf("protected mount admitted: %s", bind)
		}
	}
	if DockerBindUsesRustFSVolume("ordinary:/data") {
		t.Fatal("ordinary volume blocked")
	}
	for _, body := range []string{
		`{"services":{"x":{"container_name":"aurago-rustfs"}}}`,
		`{"services":{"x":{"labels":{"aurago.managed":"rustfs"}}}}`,
		`{"services":{"x":{"volumes":[{"type":"volume","source":"alias","target":"/data"}]}},"volumes":{"alias":{"name":"aurago_rustfs_data"}}}`,
		`{"services":{"x":{"volumes":[{"type":"bind","source":"/var/lib/docker/volumes/aurago_rustfs_credentials/_data","target":"/data"}]}}}`,
		`{"volumes":{"alias":{"driver_opts":{"device":"/var/lib/docker/volumes/aurago_rustfs_data/_data"}}}}`,
		`{"services":{"x":{"volumes_from":["container:aurago-rustfs:ro"]}}}`,
		`{"services":{"x":{"pid":"container:aurago-rustfs-key-seed"}}}`,
		`{"volumes":{"alias":{"labels":{"com.aurago.managed":"true","com.aurago.owner":"rustfs"}}}}`,
	} {
		model, err := ParseDockerComposeModel(body)
		if err != nil {
			t.Fatal(err)
		}
		if owner := DockerComposeModelOwner(model); owner != dockerutil.RustFSOwner {
			t.Fatalf("owner=%q for %s", owner, body)
		}
	}
	for _, inspect := range []string{`{"Name":"/aurago-rustfs","Config":{"Labels":{}}}`, `{"Name":"/renamed","Config":{"Labels":{"aurago.managed":"rustfs"}}}`} {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/containers/abcd/json") {
				t.Errorf("unexpected request: %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(inspect))
		})
		if !DockerContainerManagedBy(DockerConfig{Host: host}, "abcd", dockerutil.RustFSOwner) {
			t.Fatal("container ID bypassed ownership")
		}
	}
}

func TestRustFSSecretsAreNotPythonAccessible(t *testing.T) {
	for _, key := range []string{"rustfs_state", "rustfs_root_secret", "rustfs_child_secret"} {
		if IsPythonAccessibleSecret(key) {
			t.Fatalf("private key admitted: %s", key)
		}
	}
}

func TestRustFSDockerAndComposeBlockBackingParentsAndNativeAliases(t *testing.T) {
	for _, source := range []string{"/var/lib/docker/volumes", "/var/lib/docker", "/host_mnt/var/lib/docker/volumes", "/host_mnt/opt/../var/lib/docker/volumes", "/var/lib/docker/volumes/public/../aurago_rustfs_credentials/_data"} {
		if !DockerBindUsesRustFSVolume(source + ":/keys:ro") {
			t.Fatalf("ancestor/alias bind admitted: %s", source)
		}
		for _, model := range []DockerComposeModel{
			{Services: map[string]DockerComposeService{"reader": {Volumes: []DockerComposeMount{{Type: "bind", Source: source, Target: "/keys"}}}}},
			{Volumes: map[string]DockerComposeNamedVolume{"alias": {Driver: "local", DriverOpts: map[string]string{"type": "none", "o": "bind", "device": source}}}},
		} {
			if owner := DockerComposeModelOwner(model); owner != dockerutil.RustFSOwner {
				t.Fatalf("Compose parent admitted: %s", source)
			}
		}
	}
	root := t.TempDir()
	private := filepath.Join(root, "private")
	alias := filepath.Join(root, "link")
	if err := os.MkdirAll(private, 0700); err != nil {
		t.Fatal(err)
	}
	dockerutil.RegisterRustFSVolumePath(private)
	if err := os.Symlink(private, alias); err != nil {
		t.Skipf("native symlink unavailable: %v", err)
	}
	if !DockerBindUsesRustFSVolume(alias + ":/keys") {
		t.Fatal("resolved native alias bypassed ownership")
	}
	if validateDockerBindMount(DockerConfig{WorkspaceDir: root}, alias+":/keys") == nil {
		t.Fatal("workspace alias bypassed bind guard")
	}
}

func TestRustFSReservedRenameNeverReachesDocker(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var requests atomic.Int32
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNoContent) })
	for _, name := range []string{dockerutil.RustFSContainerName, dockerutil.RustFSSeedContainerName, "project-aurago-rustfs"} {
		if got := DockerRenameContainer(DockerConfig{Host: host}, "ordinary", name); !strings.Contains(got, "reserved AuraGo") {
			t.Fatalf("reserved rename admitted: %s", got)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("reserved rename contacted Engine")
	}
}

func TestRustFSExistingNamedVolumeAliasesAreInspectedAndFailClosed(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, tc := range []struct {
		name, body    string
		status        int
		owned, failed bool
	}{
		{"device", `{"Name":"alias","Mountpoint":"/var/lib/docker/volumes/alias/_data","Options":{"device":"/var/lib/docker/volumes/aurago_rustfs_credentials/_data"}}`, 200, true, false},
		{"label", `{"Name":"alias","Mountpoint":"/var/lib/docker/volumes/alias/_data","Labels":{"aurago.managed":"rustfs"}}`, 200, true, false},
		{"backing", `{"Name":"alias","Mountpoint":"/var/lib/docker/volumes/aurago_rustfs_data/_data"}`, 200, true, false},
		{"ordinary", `{"Name":"alias","Mountpoint":"/var/lib/docker/volumes/alias/_data","Options":{}}`, 200, false, false},
		{"absent", `{"message":"absent"}`, 404, false, false},
		{"denied", `{"message":"private diagnostic must not escape"}`, 403, false, true},
		{"malformed", `{`, 200, false, true},
		{"incomplete", `{"Name":"alias"}`, 200, false, true},
		{"relative mountpoint", `{"Name":"alias","Mountpoint":"relative"}`, 200, false, true},
		{"relative backing path", `{"Name":"alias","Driver":"local","Mountpoint":"/var/lib/docker/volumes/alias/_data","Options":{"type":"none","device":"relative/private"}}`, 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mutations atomic.Int32
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					mutations.Add(1)
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			cfg := DockerConfig{Host: host, Context: context.Background()}
			owned, err := DockerVolumeUsesRustFS(cfg, "alias")
			if owned != tc.owned || (err != nil) != tc.failed {
				t.Fatalf("ownership=%v error=%v", owned, err)
			}
			if err != nil && strings.Contains(err.Error(), "private diagnostic") {
				t.Fatal("Engine body escaped")
			}
			model := DockerComposeModel{Services: map[string]DockerComposeService{"reader": {Volumes: []DockerComposeMount{{Type: "volume", Source: "x", Target: "/keys"}}}}, Volumes: map[string]DockerComposeNamedVolume{"x": {Name: "alias"}}}
			owned, err = DockerComposeUsesRustFSVolumeAliases(cfg, model)
			if owned != tc.owned || (err != nil) != tc.failed {
				t.Fatalf("Compose ownership=%v error=%v", owned, err)
			}
			if tc.owned || tc.failed {
				got := DockerCreateContainerWithOptions(cfg, "reader", "alpine", nil, nil, []string{"alias:/keys"}, nil, "no", nil, ContainerCreateOptions{})
				var result map[string]any
				if json.Unmarshal([]byte(got), &result) != nil || result["status"] != "error" {
					t.Fatalf("private/unverified mount admitted: %s", got)
				}
				if mutations.Load() != 0 {
					t.Fatal("alias mount reached a mutation")
				}
			} else if err := validateDockerBindMount(cfg, "alias:/keys"); err != nil {
				t.Fatalf("ordinary/absent named volume refused: %v", err)
			}
		})
	}
}
