package rustfs

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestObservedEndpointUsesOwnedContainerAddress(t *testing.T) {
	cfg := testConfig()
	cfg.Runtime.IsDocker = true
	s := newState()
	s.ContainerID = "runtime-container-id"
	s.Endpoint = "http://aurago-rustfs:9000" // saved alias from older state
	m := &Manager{names: resourceNames{container: "aurago-rustfs"}}
	placement := placement{network: "app_default", networkID: "network-id"}
	info := containerInfo{ID: s.ContainerID}
	info.Config.Labels = m.labels("runtime", s)
	info.HostConfig.NetworkMode = placement.network
	info.NetworkSettings.Networks = map[string]containerNetwork{
		placement.network: {NetworkID: placement.networkID, IPAddress: "172.23.0.8"},
	}

	endpoint, err := m.observedEndpoint(cfg, s, info, placement)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "http://172.23.0.8:9000" || strings.Contains(endpoint, m.names.container) {
		t.Fatalf("endpoint %q is not the inspected container address", endpoint)
	}
	network := info.NetworkSettings.Networks[placement.network]
	network.IPAddress = "172.23.0.9"
	info.NetworkSettings.Networks[placement.network] = network
	endpoint, err = m.observedEndpoint(cfg, s, info, placement)
	if err != nil || endpoint != "http://172.23.0.9:9000" {
		t.Fatalf("changed container address %q: %v", endpoint, err)
	}
	network.IPAddress = ""
	network.GlobalIPv6Address = "2001:db8::8"
	info.NetworkSettings.Networks[placement.network] = network
	endpoint, err = m.observedEndpoint(cfg, s, info, placement)
	if err != nil || endpoint != "http://[2001:db8::8]:9000" {
		t.Fatalf("IPv6 container address %q: %v", endpoint, err)
	}
}

func TestObservedEndpointRejectsUnprovenContainerAddresses(t *testing.T) {
	cfg := testConfig()
	cfg.Runtime.IsDocker = true
	s := newState()
	s.ContainerID = "runtime-container-id"
	m := &Manager{names: resourceNames{container: "aurago-rustfs"}}
	p := placement{network: "app_default", networkID: "network-id"}
	base := func() containerInfo {
		info := containerInfo{ID: s.ContainerID}
		info.Config.Labels = m.labels("runtime", s)
		info.HostConfig.NetworkMode = p.network
		info.NetworkSettings.Networks = map[string]containerNetwork{
			p.network: {NetworkID: p.networkID, IPAddress: "172.23.0.8"},
		}
		return info
	}
	tests := []struct {
		name   string
		mutate func(*containerInfo)
	}{
		{name: "foreign container", mutate: func(info *containerInfo) { info.ID = "foreign-container-id" }},
		{name: "foreign labels", mutate: func(info *containerInfo) { info.Config.Labels = nil }},
		{name: "different network", mutate: func(info *containerInfo) {
			network := info.NetworkSettings.Networks[p.network]
			network.NetworkID = "other-network"
			info.NetworkSettings.Networks[p.network] = network
		}},
		{name: "missing address", mutate: func(info *containerInfo) {
			network := info.NetworkSettings.Networks[p.network]
			network.IPAddress = "aurago-rustfs"
			info.NetworkSettings.Networks[p.network] = network
		}},
		{name: "published port", mutate: func(info *containerInfo) {
			info.NetworkSettings.Ports = map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string
			}{"9000/tcp": {{HostIP: "0.0.0.0", HostPort: "9000"}}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := base()
			test.mutate(&info)
			if _, err := m.observedEndpoint(cfg, s, info, p); ErrorCode(err) != "unsupported_placement" {
				t.Fatalf("unproven address accepted: %v", err)
			}
		})
	}
}

func TestObservedEndpointRejectsNonUnicastAddresses(t *testing.T) {
	cfg := testConfig()
	cfg.Runtime.IsDocker = true
	s := newState()
	s.ContainerID = "runtime-container-id"
	m := &Manager{names: resourceNames{container: "aurago-rustfs"}}
	p := placement{network: "app_default", networkID: "network-id"}
	addresses := []string{"0.0.0.0", "127.0.0.1", "169.254.1.2", "224.0.0.1", "255.255.255.255", "::", "::1", "fe80::1", "ff02::1", "not-an-ip"}
	for _, address := range addresses {
		t.Run(address, func(t *testing.T) {
			info := containerInfo{ID: s.ContainerID}
			info.Config.Labels = m.labels("runtime", s)
			info.HostConfig.NetworkMode = p.network
			info.NetworkSettings.Networks = map[string]containerNetwork{
				p.network: {NetworkID: p.networkID, IPAddress: address},
			}
			if _, err := m.observedEndpoint(cfg, s, info, p); ErrorCode(err) != "unsupported_placement" {
				t.Fatalf("non-unicast endpoint %q accepted: %v", address, err)
			}
		})
	}
}

func TestObservedEndpointKeepsNativeLoopbackPublication(t *testing.T) {
	cfg := testConfig()
	s := newState()
	s.ContainerID = "runtime-container-id"
	m := &Manager{names: resourceNames{container: "aurago-rustfs"}}
	info := containerInfo{ID: s.ContainerID}
	info.Config.Labels = m.labels("runtime", s)
	info.HostConfig.NetworkMode = "bridge"
	info.NetworkSettings.Ports = map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string
	}{"9000/tcp": {{HostIP: "127.0.0.1", HostPort: "43210"}}}

	endpoint, err := m.observedEndpoint(cfg, s, info, placement{network: "bridge"})
	if err != nil || endpoint != "http://127.0.0.1:43210" {
		t.Fatalf("native endpoint %q: %v", endpoint, err)
	}
}

func TestEndpointRefreshCancelsOnlyManagedLeases(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey, cfg.S3.SecretKey = "unit-access", "unit-secret"
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	lease, err := m.AcquireS3(context.Background(), cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	m.mu.Lock()
	managedLeases := m.desired
	m.updateManagedEndpointLocked("http://172.23.0.8:9000")
	m.mu.Unlock()
	if managedLeases.Err() != context.Canceled {
		t.Fatal("managed leases survived endpoint replacement")
	}
	if err = lease.Check(); err != nil {
		t.Fatalf("external lease was cancelled by a RustFS endpoint change: %v", err)
	}
}

func TestAcquireS3RefreshesOwnedEndpointAndRevokesOldLease(t *testing.T) {
	cfg := testConfig()
	names := resourceNames{container: "rustfs-endpoint-test"}
	s := newState()
	s.EngineID = "fixture-engine"
	s.ContainerID = "runtime-container-id"
	s.Provisioned = true
	s.Endpoint = "http://127.0.0.1:43210"
	port := "43210"
	engine := &fixtureEngine{}
	owner, cancel := context.WithCancel(context.Background())
	defer cancel()
	desired, desiredCancel := context.WithCancel(owner)
	m := &Manager{
		managerState: managerState{cfg: cfg, desired: desired, desiredCancel: desiredCancel, generation: 1, fingerprint: configFingerprint(cfg), state: s, jobs: map[string]*job{}},
		vault:        &memoryVault{}, logger: quietLogger(), docker: engine, http: &http.Client{}, names: names, owner: owner,
	}
	m.status.Ready = true
	m.status.AccessState = "ready"
	engine.request = func(ctx context.Context, method, path string, body, result any) (int, error) {
		if method != "GET" {
			t.Fatalf("endpoint refresh performed Docker mutation %s %s", method, path)
		}
		switch path {
		case "info":
			fixtureResult(result, map[string]string{"ID": s.EngineID, "OSType": "linux"})
		case "containers/" + names.container + "/json":
			fixtureResult(result, map[string]any{
				"Id":         s.ContainerID,
				"Config":     map[string]any{"Image": Image, "User": "10001:10001", "Labels": m.labels("runtime", s)},
				"State":      map[string]any{"Running": true},
				"HostConfig": map[string]any{"NetworkMode": "bridge"},
				"Mounts": []map[string]any{
					{"Type": "volume", "Name": names.data, "Destination": "/data", "RW": true},
					{"Type": "volume", "Name": names.credentials, "Destination": "/run/aurago-rustfs", "RW": false},
				},
				"NetworkSettings": map[string]any{"Ports": map[string]any{"9000/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": port}}}},
			})
		default:
			t.Fatalf("unexpected Docker read %s", path)
		}
		return 200, nil
	}
	old, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Release()
	port = "43211"
	current, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Release()
	if current.Endpoint != "http://127.0.0.1:43211" {
		t.Fatalf("lease kept stale endpoint %q", current.Endpoint)
	}
	select {
	case <-old.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("endpoint change did not cancel the earlier managed lease context")
	}
	if ErrorCode(old.Check()) != "stale_target" {
		t.Fatal("endpoint change did not revoke the earlier managed lease")
	}
}
