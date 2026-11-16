package desktopstore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestStoreIntegrationOperationExcludesLifecycle(t *testing.T) {
	s := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18000))
	op, err := s.StartInstall(context.Background(), InstallRequest{AppID: "uptime-kuma", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	finish, err := s.IntegrationOperation(context.Background(), "uptime-kuma")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartAppOperation(context.Background(), "uptime-kuma", OperationStop, OperationRequest{}); !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("concurrent stop: %v", err)
	}
	if err = s.IntegrationIdle(context.Background(), "uptime-kuma"); !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("dispatch during test: %v", err)
	}
	finish(nil)
	if err = s.IntegrationIdle(context.Background(), "uptime-kuma"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreIntegrationDNSAndNetworkAreExplicit(t *testing.T) {
	secrets := &fakeSecretStore{data: map[string]string{}}
	s := newTestServiceWithSecrets(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18000), secrets)
	app := InstalledApp{AppID: "adguard-home", Env: []string{"KEEP=yes"}}
	if err := s.applyIntegrationRuntime(&app, ""); err != nil {
		t.Fatal(err)
	}
	if len(app.Ports) != 0 {
		t.Fatal("DNS published without consent")
	}
	raw, _ := json.Marshal(IntegrationSettings{DNSAddress: "192.168.2.5", DockerNetwork: "shared"})
	secrets.data[IntegrationKey(app.AppID)] = string(raw)
	if err := s.applyIntegrationRuntime(&app, ""); err != nil {
		t.Fatal(err)
	}
	if len(app.Ports) != 2 || app.Ports[0].Protocol != "tcp" || app.Ports[1].Protocol != "udp" {
		t.Fatalf("ports: %+v", app.Ports)
	}
	for _, p := range app.Ports {
		if p.HostPort != 53 || p.HostIP != "192.168.2.5" {
			t.Fatalf("DNS fallback: %+v", p)
		}
	}
	spec := containerSpecFromRecord(app)
	if spec.NetworkMode != "shared" || len(spec.Env) != 1 || spec.Env[0] != "KEEP=yes" {
		t.Fatalf("runtime: %+v", spec)
	}
}

func TestStoreIntegrationEndpointUsesPublishedAdminPort(t *testing.T) {
	app := InstalledApp{ContainerName: "store-adguard", Ports: []PortBinding{{ID: "main", HostIP: "127.0.0.1", HostPort: 13000, ContainerPort: 3000, Protocol: "tcp"}, {ID: "admin", HostPort: 13080, ContainerPort: 80, Protocol: "tcp"}}}
	for _, tc := range []struct {
		docker bool
		want   string
	}{{false, "http://127.0.0.1:13080"}, {true, "http://store-adguard:80"}} {
		got, err := IntegrationURL(app, "admin", tc.docker)
		if err != nil || got != tc.want {
			t.Fatalf("got %q %v", got, err)
		}
	}
	if _, err := IntegrationURL(app, "dns-tcp", false); err == nil {
		t.Fatal("accepted missing web port")
	}
}

func TestStoreIntegrationLegacyAdminReadinessAndDNSPreview(t *testing.T) {
	s := newTestService(t, &fakeDockerAdapter{}, &fakeDesktopAdapter{}, &fakeLaunchpadAdapter{}, fixedPorts(18000, 18080))
	s.portProbe = func(_ context.Context, _ string, port int) bool { return port == 18080 }
	op, err := s.StartInstall(t.Context(), InstallRequest{AppID: "adguard-home", BindMode: BindModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RunOperation(t.Context(), op.ID); err != nil {
		t.Fatal(err)
	}
	app, _, _ := s.GetInstalled(t.Context(), "adguard-home")
	app.Ports = append(app.Ports, PortBinding{ID: "dns-tcp", HostPort: 53, Protocol: "tcp"})
	if _, err = selectAppPort(app, "dns-tcp"); err == nil {
		t.Fatal("DNS admitted to HTTP preview")
	}
}
