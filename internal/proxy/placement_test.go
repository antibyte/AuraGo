package proxy

import (
	"errors"
	"reflect"
	"testing"
)

// composeSelfInspection mirrors `docker inspect aurago` for the default
// docker-compose.yml deployment.
const composeSelfInspection = `{
	"Id": "4f1c0ffee",
	"Name": "/aurago",
	"HostConfig": {
		"NetworkMode": "aurago_default",
		"PortBindings": {"8088/tcp": [{"HostIp": "", "HostPort": "8088"}]}
	},
	"Mounts": [
		{"Type": "bind", "Source": "/srv/aurago/config", "Destination": "/run/optional-config"},
		{"Type": "volume", "Name": "aurago_aurago_data", "Source": "/var/lib/docker/volumes/aurago_aurago_data/_data", "Destination": "/app/data"},
		{"Type": "volume", "Name": "aurago_models", "Source": "/var/lib/docker/volumes/aurago_models/_data", "Destination": "/app/data/models/aurago-qwen35"},
		{"Type": "volume", "Name": "aurago_aurago_workdir", "Destination": "/app/agent_workspace/workdir"}
	],
	"NetworkSettings": {"Networks": {
		"aurago_default": {"NetworkID": "net-default"},
		"aurago-app": {"NetworkID": "net-app"},
		"aurago_docker-control": {"NetworkID": "net-control"}
	}}
}`

var composeInternalNetworks = map[string]bool{
	"aurago_default":        false,
	"aurago-app":            true,
	"aurago_docker-control": true,
}

func mustParseSelf(t *testing.T, raw string) selfContainer {
	t.Helper()
	self, err := parseSelfContainer([]byte(raw))
	if err != nil {
		t.Fatalf("parseSelfContainer() error = %v", err)
	}
	return self
}

func TestDockerPlacementSharesComposeVolumeAndNetwork(t *testing.T) {
	self := mustParseSelf(t, composeSelfInspection)

	got, err := dockerPlacement(self, "/app/data/proxy", "1.45", composeInternalNetworks, 8088)
	if err != nil {
		t.Fatalf("dockerPlacement() error = %v", err)
	}
	if got.network != "aurago_default" {
		t.Fatalf("network = %q, want the non-internal compose network", got.network)
	}
	if got.upstream != "aurago:8088" {
		t.Fatalf("upstream = %q, want the AuraGo container name on the shared network", got.upstream)
	}
	if got.binds != nil {
		t.Fatalf("binds = %#v, want none: container paths are not host paths", got.binds)
	}
	want := []map[string]interface{}{
		{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/etc/caddy", "ReadOnly": true, "VolumeOptions": map[string]interface{}{"Subpath": "proxy"}},
		{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/data", "VolumeOptions": map[string]interface{}{"Subpath": "proxy/caddy_data"}},
		{"Type": "volume", "Source": "aurago_aurago_data", "Target": "/config", "VolumeOptions": map[string]interface{}{"Subpath": "proxy/caddy_config"}},
	}
	if !reflect.DeepEqual(got.mounts, want) {
		t.Fatalf("mounts = %#v\nwant %#v", got.mounts, want)
	}
}

func TestDockerPlacementTranslatesBindMountedDataDirectory(t *testing.T) {
	self := mustParseSelf(t, `{
		"Name": "/aurago",
		"HostConfig": {"NetworkMode": "aurago_default"},
		"Mounts": [{"Type": "bind", "Source": "/srv/aurago/data", "Destination": "/app/data"}],
		"NetworkSettings": {"Networks": {"aurago_default": {"NetworkID": "net-default"}}}
	}`)

	// Bind mounts need no subpath support, so an old engine still works.
	got, err := dockerPlacement(self, "/app/data/proxy", "1.41", map[string]bool{"aurago_default": false}, 8088)
	if err != nil {
		t.Fatalf("dockerPlacement() error = %v", err)
	}
	want := []map[string]interface{}{
		{"Type": "bind", "Source": "/srv/aurago/data/proxy", "Target": "/etc/caddy", "ReadOnly": true},
		{"Type": "bind", "Source": "/srv/aurago/data/proxy/caddy_data", "Target": "/data"},
		{"Type": "bind", "Source": "/srv/aurago/data/proxy/caddy_config", "Target": "/config"},
	}
	if !reflect.DeepEqual(got.mounts, want) {
		t.Fatalf("mounts = %#v\nwant %#v", got.mounts, want)
	}
}

func TestDockerPlacementNeedsVolumeSubpathSupport(t *testing.T) {
	self := mustParseSelf(t, composeSelfInspection)

	_, err := dockerPlacement(self, "/app/data/proxy", "1.44", composeInternalNetworks, 8088)
	if !errors.Is(err, ErrDockerPlacement) {
		t.Fatalf("dockerPlacement() error = %v, want ErrDockerPlacement", err)
	}
}

func TestDockerPlacementRejectsDataOutsideMounts(t *testing.T) {
	self := mustParseSelf(t, `{
		"Name": "/aurago",
		"HostConfig": {"NetworkMode": "aurago_default"},
		"Mounts": [{"Type": "volume", "Name": "aurago_aurago_workdir", "Destination": "/app/agent_workspace/workdir"}],
		"NetworkSettings": {"Networks": {"aurago_default": {"NetworkID": "net-default"}}}
	}`)

	_, err := dockerPlacement(self, "/app/data/proxy", "1.45", map[string]bool{"aurago_default": false}, 8088)
	if !errors.Is(err, ErrDockerPlacement) {
		t.Fatalf("dockerPlacement() error = %v, want ErrDockerPlacement", err)
	}
}

func TestDockerPlacementWithoutSharedNetworkUsesPublishedPort(t *testing.T) {
	for _, tc := range []struct {
		name       string
		inspection string
		want       string
	}{
		{
			name: "default bridge with published port",
			inspection: `{
				"Name": "/aurago",
				"HostConfig": {"NetworkMode": "default", "PortBindings": {"8088/tcp": [{"HostIp": "0.0.0.0", "HostPort": "9000"}]}},
				"Mounts": [{"Type": "volume", "Name": "aurago_data", "Destination": "/app/data"}],
				"NetworkSettings": {"Networks": {"bridge": {"NetworkID": "net-bridge"}}}
			}`,
			want: "host.docker.internal:9000",
		},
		{
			name: "host network",
			inspection: `{
				"Name": "/aurago",
				"HostConfig": {"NetworkMode": "host"},
				"Mounts": [{"Type": "volume", "Name": "aurago_data", "Destination": "/app/data"}],
				"NetworkSettings": {"Networks": {"host": {"NetworkID": "net-host"}}}
			}`,
			want: "host.docker.internal:8088",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			self := mustParseSelf(t, tc.inspection)
			got, err := dockerPlacement(self, "/app/data/proxy", "1.45", map[string]bool{"bridge": false, "host": false}, 8088)
			if err != nil {
				t.Fatalf("dockerPlacement() error = %v", err)
			}
			if got.network != "" {
				t.Fatalf("network = %q, want the engine default", got.network)
			}
			if got.upstream != tc.want {
				t.Fatalf("upstream = %q, want %q", got.upstream, tc.want)
			}
		})
	}
}

func TestDockerPlacementPrefersComposeDefaultNetwork(t *testing.T) {
	self := mustParseSelf(t, `{
		"Name": "/aurago",
		"HostConfig": {"NetworkMode": "aurago_default"},
		"Mounts": [{"Type": "volume", "Name": "aurago_data", "Destination": "/app/data"}],
		"NetworkSettings": {"Networks": {
			"aurago_default": {"NetworkID": "a"},
			"aaa-lan": {"NetworkID": "b"},
			"docker-control": {"NetworkID": "c"}
		}}
	}`)

	got, err := dockerPlacement(self, "/app/data/proxy", "1.45", map[string]bool{"aurago_default": false, "aaa-lan": false, "docker-control": false}, 8088)
	if err != nil {
		t.Fatalf("dockerPlacement() error = %v", err)
	}
	if got.network != "aurago_default" {
		t.Fatalf("network = %q, want aurago_default", got.network)
	}
}

func TestCompareAPIVersions(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        int
	}{
		{"1.45", "1.45", 0},
		{"1.55", "1.45", 1},
		{"1.9", "1.45", -1},
		{"v1.46", "1.45", 1},
		{"2.0", "1.45", 1},
		{"", "1.45", -1},
	} {
		if got := compareAPIVersions(tc.left, tc.right); got != tc.want {
			t.Errorf("compareAPIVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

const (
	proxySelfContainerID  = "4f6c1d0b9a2e8c7f53e1a0b2c4d6e8f0a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1"
	proxyOtherContainerID = "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d"
)

// etcMountinfo is the part of /proc/self/mountinfo with the three /etc files
// Docker bind-mounts into a container from root.
func etcMountinfo(root string) string {
	return "1531 1520 8:1 " + root + "/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/sda1 rw\n" +
		"1532 1520 8:1 " + root + "/hostname /etc/hostname rw,relatime - ext4 /dev/sda1 rw\n" +
		"1533 1520 8:1 " + root + "/hosts /etc/hosts rw,relatime - ext4 /dev/sda1 rw\n"
}

// TestSelfContainerIDsUseProcAndTheDefaultHostnameOnly pins the shared self
// detection (dockerutil, M1) on the proxy side: the container ID from /proc
// first, then Docker's default hostname. A custom hostname is skipped because
// inspectSelf would look it up as a container name.
func TestSelfContainerIDsUseProcAndTheDefaultHostnameOnly(t *testing.T) {
	standard := etcMountinfo("/var/lib/docker/containers/" + proxySelfContainerID)
	for _, tc := range []struct {
		name     string
		files    map[string]string
		hostname string
		want     []string
	}{
		// Unchanged.
		{"default hostname", map[string]string{"/proc/self/mountinfo": standard}, proxySelfContainerID[:12], []string{proxySelfContainerID, proxySelfContainerID[:12]}},
		{"default hostname without /proc", nil, proxySelfContainerID[:12], []string{proxySelfContainerID[:12]}},
		// Changed by the shared helper.
		{"custom hostname", map[string]string{"/proc/self/mountinfo": standard}, "aurago", []string{proxySelfContainerID}},
		{"native or LXC guest", nil, "lxc-host", nil},
		{"another container's file mounted first", map[string]string{"/proc/self/mountinfo": "1530 1520 8:1 /var/lib/docker/containers/" + proxyOtherContainerID + "/hostname /mnt/other-hostname ro - ext4 /dev/sda1 rw\n" + standard}, "aurago", []string{proxySelfContainerID}},
		{"dedicated containers filesystem", map[string]string{"/proc/self/mountinfo": etcMountinfo("/" + proxySelfContainerID)}, "aurago", []string{proxySelfContainerID}},
		{"cgroup v1 only", map[string]string{"/proc/self/cgroup": "12:memory:/docker/" + proxySelfContainerID + "\n"}, "aurago", []string{proxySelfContainerID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readFile := func(path string) ([]byte, error) {
				if text, ok := tc.files[path]; ok {
					return []byte(text), nil
				}
				return nil, errors.New("no such file")
			}
			got := selfContainerIDsFrom(readFile, func() (string, error) { return tc.hostname, nil })
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("selfContainerIDs = %#v, want %#v", got, tc.want)
			}
		})
	}
}
